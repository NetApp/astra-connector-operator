/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package register

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/NetApp/astraagent-operator/common"

	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/NetApp/astraagent-operator/api/v1"
)

const (
	errorRetrySleep time.Duration = time.Second * 3
)

func RegisterClient(m *v1.AstraConnector) (string, error) {
	natsSyncClientRegisterURL := GetNatssyncClientRegistrationURL(m)
	reqBodyBytes, err := generateAuthPayload(m)
	if err != nil {
		return "", err
	}

	response, err := http.Post(natsSyncClientRegisterURL, "application/json", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return "", err
	}

	if response.StatusCode != 201 {
		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			return "", err
		}
		errMsg := fmt.Sprintf("Unexpected registration status code: %d; %s", response.StatusCode, string(bodyBytes))
		return "", errors.New(errMsg)
	}
	type AstraConnectorID struct {
		AstraConnectorID string `json:"astraConnectorID"`
	}
	astraConnectorID := &AstraConnectorID{}
	err = json.NewDecoder(response.Body).Decode(astraConnectorID)
	if err != nil {
		return "", err
	}
	return astraConnectorID.AstraConnectorID, nil
}

func UnregisterClient(m *v1.AstraConnector) error {
	natsSyncClientUnregisterURL := getNatssyncClientUnregisterURL(m)
	reqBodyBytes, err := generateAuthPayload(m)
	if err != nil {
		return err
	}

	response, err := http.Post(natsSyncClientUnregisterURL, "application/json", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusNoContent {
		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			return err
		}
		errMsg := fmt.Sprintf("Unexpected unregistration status code: %d; %s", response.StatusCode, string(bodyBytes))
		return errors.New(errMsg)
	}
	return nil
}

func generateAuthPayload(m *v1.AstraConnector) ([]byte, error) {
	if m.Spec.Astra.OldAuth {
		reqBodyBytes, err := json.Marshal(map[string]string{"authToken": m.Spec.Astra.Token})
		if err != nil {
			return nil, err
		}
		return reqBodyBytes, nil
	}

	authPayload, err := json.Marshal(map[string]string{
		"userToken": m.Spec.Astra.Token,
		"accountId": m.Spec.Astra.AccountID,
	})
	if err != nil {
		return nil, err
	}
	reqBodyBytes, err := json.Marshal(map[string]string{"authToken": base64.StdEncoding.EncodeToString(authPayload)})
	if err != nil {
		return nil, err
	}
	return reqBodyBytes, nil
}

func GetAstraHostURL(m *v1.AstraConnector, ctx context.Context) string {
	log := ctrllog.FromContext(ctx)
	var astraHost string
	if m.Spec.NatssyncClient.CloudBridgeURL != "" {
		astraHost = m.Spec.NatssyncClient.CloudBridgeURL
	} else {
		astraHost = common.NatssyncClientDefaultCloudBridgeURL
	}
	log.Info("CloudBridgeURL for Astra Host", "CloudBridgeURL", astraHost)
	return astraHost
}

func getAstraHostFromURL(astraHostURL string) (string, error) {
	cloudBridgeURLSplit := strings.Split(astraHostURL, "://")
	if len(cloudBridgeURLSplit) != 2 {
		errStr := fmt.Sprintf("invalid cloudBridgeURL provided: %s, format - https://hostname", astraHostURL)
		return "", errors.New(errStr)
	}
	return cloudBridgeURLSplit[1], nil
}

func AddConnectorIDtoAstra(m *v1.AstraConnector, astraConnectorID string, ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	setTlsValidation(m.Spec.NatssyncClient.SkipTLSValidation, ctx)

	astraHost := GetAstraHostURL(m, ctx)
	if m.Spec.NatssyncClient.HostAlias {
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}

		cloudBridgeHost, err := getAstraHostFromURL(astraHost)
		if err != nil {
			return err
		}

		http.DefaultTransport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if addr == cloudBridgeHost+":443" {
				addr = m.Spec.NatssyncClient.HostAliasIP + ":443"
			}
			if addr == cloudBridgeHost+":80" {
				addr = m.Spec.NatssyncClient.HostAliasIP + ":80"
			}
			return dialer.DialContext(ctx, network, addr)
		}
	}
	log.Info("Checking for a valid SA credential for cloud", "cloudType", common.AstraDefaultCloudType)
	credentialName, err := checkCloudCreds(astraHost, m, ctx)
	if err != nil {
		log.Error(err, "Error finding a valid SA cred for cloud", "cloudType", common.AstraDefaultCloudType)
		return err
	}
	if credentialName == "" {
		return fmt.Errorf("could not find a valid SA cred for cloud %s", common.AstraDefaultCloudType)
	}
	log.Info("Found a valid SA credential for cloud", "cloudType", common.AstraDefaultCloudType, "credName", credentialName)

	log.Info("Fetching cloud ID")
	cloudID, err := GetCloudId(astraHost, m.Spec.Astra.AccountID, m.Spec.Astra.Token, common.AstraDefaultCloudType, ctx)
	if err != nil {
		log.Error(err, "Error fetching cloud ID")
		return err
	}
	if cloudID == "" && credentialName != "" {
		log.Info("Cloud doesn't seem to exist, creating the cloud", "cloudType", common.AstraDefaultCloudType)
		cloudID, err = createCloud(astraHost, m, ctx)
		if err != nil {
			log.Error(err, "Failed to create cloud", "cloudType", common.AstraDefaultCloudType)
			return err
		}
		if cloudID == "" {
			return fmt.Errorf("could not create cloud %s", common.AstraDefaultCloudType)
		}
	}
	log.Info("Found cloud ID", "cloudID", cloudID)

	log.Info("Finding cluster ID")
	clusterId, managedState, err := GetClusterId(astraHost, m.Spec.Astra.ClusterName, m.Spec.Astra.AccountID, cloudID, m.Spec.Astra.Token, ctx)
	if err != nil {
		log.Error(err, "Error fetching cluster ID")
		return err
	}
	log.Info("Found cluster ID", "clusterId", clusterId)

	// Register the astraConnectorID with Astra
	astraConnectorID = fmt.Sprintf("v1:%s", astraConnectorID)
	err = RegisterConnectorID(astraHost, m.Spec.Astra.AccountID, m.Spec.Astra.Token, astraConnectorID, cloudID, clusterId, managedState, ctx)
	if err != nil {
		log.Error(err, "Error registering location ID with Astra")
		return err
	}
	return nil
}

func RemoveConnectorIDFromAstra(m *v1.AstraConnector, ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	setTlsValidation(m.Spec.NatssyncClient.SkipTLSValidation, ctx)

	astraHost := GetAstraHostURL(m, ctx)
	if m.Spec.NatssyncClient.HostAlias {
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}

		cloudBridgeHost, err := getAstraHostFromURL(astraHost)
		if err != nil {
			return err
		}

		http.DefaultTransport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if addr == cloudBridgeHost+":443" {
				addr = m.Spec.NatssyncClient.HostAliasIP + ":443"
			}
			if addr == cloudBridgeHost+":80" {
				addr = m.Spec.NatssyncClient.HostAliasIP + ":80"
			}
			return dialer.DialContext(ctx, network, addr)
		}
	}
	log.Info("Fetching cloud ID")
	cloudID, err := GetCloudId(astraHost, m.Spec.Astra.AccountID, m.Spec.Astra.Token, common.AstraDefaultCloudType, ctx)
	if err != nil {
		log.Error(err, "Error fetching cloud ID")
		return err
	}
	log.Info("Found cloud ID", "cloudID", cloudID)

	log.Info("Finding cluster ID")
	clusterId, managedState, err := GetClusterId(astraHost, m.Spec.Astra.ClusterName, m.Spec.Astra.AccountID, cloudID, m.Spec.Astra.Token, ctx)
	if err != nil {
		log.Error(err, "Error fetching cluster ID")
		return err
	}
	log.Info("Found cluster ID", "clusterId", clusterId)

	// Unregister the astraConnectorID
	err = RegisterConnectorID(astraHost, m.Spec.Astra.AccountID, m.Spec.Astra.Token, "", cloudID, clusterId, managedState, ctx)
	if err != nil {
		log.Error(err, "Error unregistering location ID with Astra")
		return err
	}
	return nil
}

func LogHttpError(response *http.Response, ctx context.Context) {
	log := ctrllog.FromContext(ctx)
	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		log.Error(err, "Error reading response body")
	} else {
		log.Info("Received unexpected status code", "responseBody", string(bodyBytes), "statusCode", response.StatusCode)
		err = response.Body.Close()
		if err != nil {
			log.Error(err, "Error closing the response body")
		}
	}
}

func ListClusters(astraHost string, accountId string, cloudId string, token string) (*http.Response, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s/clusters", astraHost, accountId, cloudId)
	httpClient := http.Client{}
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	request.Header = http.Header{
		"authorization": []string{fmt.Sprintf("Bearer %s", token)},
		"content-type":  []string{"application/json"},
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func GetClusterId(astraHost string, clusterName string, accountId string, cloudId string, token string, ctx context.Context) (string, string, error) {
	log := ctrllog.FromContext(ctx)
	var response *http.Response
	success := false
	timeout := time.Second * 30
	timeExpire := time.Now().Add(timeout)
	for time.Now().Before(timeExpire) {
		var err error
		response, err = ListClusters(astraHost, accountId, cloudId, token)
		if err != nil {
			log.Error(err, "Error listing clusters")
			time.Sleep(errorRetrySleep)
			continue
		}
		if response.StatusCode == 200 {
			success = true
			break
		}
		LogHttpError(response, ctx)
		response.Body.Close()
		time.Sleep(errorRetrySleep)
	}

	if !success {
		return "", "", fmt.Errorf("timed out querying Astra API")
	}
	defer response.Body.Close()

	type respData struct {
		Items []struct {
			Name         string `json:"name"`
			Id           string `json:"id"`
			ManagedState string `json:"managedState"`
		} `json:"items"`
	}
	bodyBytes, err := ReadResponseBody(response)
	if err != nil {
		return "", "", err
	}
	resp := respData{}
	err = json.Unmarshal(bodyBytes, &resp)
	if err != nil {
		return "", "", err
	}
	// Find ID for given name
	var clusterId string
	var managedState string
	for _, clusterData := range resp.Items {
		if clusterData.Name == clusterName {
			clusterId = clusterData.Id
			managedState = clusterData.ManagedState
			break
		}
	}
	if clusterId == "" {
		return "", "", fmt.Errorf("could not find cluster ID for cluster %s", clusterName)
	}

	return clusterId, managedState, nil
}

func ReadResponseBody(response *http.Response) ([]byte, error) {
	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return bodyBytes, nil
}

func ListClouds(astraHost string, accountId string, token string) (*http.Response, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds", astraHost, accountId)

	httpClient := http.Client{}
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	request.Header = http.Header{
		"authorization": []string{fmt.Sprintf("Bearer %s", token)},
		"content-type":  []string{"application/json"},
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func GetCloudId(astraHost string, accountId string, token string, cloudType string, ctx context.Context) (string, error) {
	log := ctrllog.FromContext(ctx)
	success := false
	var response *http.Response
	timeout := time.Second * 30
	timeExpire := time.Now().Add(timeout)
	for time.Now().Before(timeExpire) {
		var err error
		response, err = ListClouds(astraHost, accountId, token)
		if err != nil {
			log.Error(err, "Error listing clouds")
			time.Sleep(errorRetrySleep)
			continue
		}
		if response.StatusCode == 200 {
			success = true
			break
		}
		LogHttpError(response, ctx)
		response.Body.Close()
		time.Sleep(errorRetrySleep)
	}

	if !success {
		return "", fmt.Errorf("timed out querying Astra API")
	}
	defer response.Body.Close()

	type respData struct {
		Items []struct {
			CloudType string `json:"cloudType"`
			Id        string `json:"id"`
		} `json:"items"`
	}
	bodyBytes, err := ReadResponseBody(response)
	if err != nil {
		return "", err
	}
	resp := respData{}
	err = json.Unmarshal(bodyBytes, &resp)
	if err != nil {
		return "", err
	}

	var cloudId string
	for _, cloudInfo := range resp.Items {
		if cloudInfo.CloudType == cloudType {
			cloudId = cloudInfo.Id
			break
		}
	}

	return cloudId, nil
}

func RegisterConnectorID(astraCloudHost string, pcloudAccountId string, token string, astraConnectorID string, cloudId string, clusterId string, managedState string, ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	timeout := time.Second * 30
	success := false
	timeExpire := time.Now().Add(timeout)
	for time.Now().Before(timeExpire) {
		response, err := PostConnectorID(astraCloudHost, pcloudAccountId, token, astraConnectorID, cloudId, clusterId, managedState, ctx)
		if err != nil {
			log.Error(err, "Error posting location ID")
			time.Sleep(errorRetrySleep)
			continue
		}
		if response.StatusCode == 200 { // Assuming the response code at this point
			log.Info("successfully registered astraConnectorID", "astraConnectorID", astraConnectorID)
			success = true
			break
		}
		LogHttpError(response, ctx)
		response.Body.Close()
		time.Sleep(errorRetrySleep)
	}
	if !success {
		return fmt.Errorf("timed out registering with Astra")
	}
	return nil
}

func PostConnectorID(astraCloudHost string, pcloudAccountId string, token string, astraConnectorID string, cloudId string, clusterId string, managedState string, ctx context.Context) (*http.Response, error) {
	log := ctrllog.FromContext(ctx)
	clusterUrl := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s/clusters/%s", astraCloudHost, pcloudAccountId, cloudId, clusterId)
	managedClusterUrl := fmt.Sprintf("%s/accounts/%s/topology/v1/managedClusters/%s", astraCloudHost, pcloudAccountId, clusterId)
	registerUrl := clusterUrl
	payLoad := map[string]string{
		"privateRouteID": astraConnectorID,
		"version":        "1.1",
		"type":           "application/astra-cluster",
	}

	if managedState == "managed" || managedState == "managing" {
		log.Info("Using managedClusters URL", "managedState", managedState)
		registerUrl = managedClusterUrl
		payLoad["type"] = "application/astra-managedCluster"
	}
	reqBodyBytes, err := json.Marshal(payLoad)
	httpClient := http.Client{}
	request, err := http.NewRequest("PUT", registerUrl, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return nil, err
	}

	request.Header = http.Header{
		"Content-Type":  []string{"application/json"},
		"Authorization": []string{fmt.Sprintf("Bearer %s", token)},
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func setTlsValidation(disableTls bool, ctx context.Context) {
	log := ctrllog.FromContext(ctx)
	if disableTls {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		log.Info("TLS Validation Disabled! Not for use in production!")
	}
}

func checkCloudCreds(astraHost string, m *v1.AstraConnector, ctx context.Context) (string, error) {
	log := ctrllog.FromContext(ctx)
	setTlsValidation(m.Spec.NatssyncClient.SkipTLSValidation, ctx)

	if m.Spec.NatssyncClient.HostAlias {
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}

		cloudBridgeHost, err := getAstraHostFromURL(astraHost)
		if err != nil {
			return "", err
		}

		http.DefaultTransport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if addr == cloudBridgeHost+":443" {
				addr = m.Spec.NatssyncClient.HostAliasIP + ":443"
			}
			if addr == cloudBridgeHost+":80" {
				addr = m.Spec.NatssyncClient.HostAliasIP + ":80"
			}
			return dialer.DialContext(ctx, network, addr)
		}
	}

	credentialURL := fmt.Sprintf("%s/accounts/%s/core/v1/credentials", astraHost, m.Spec.Astra.AccountID)
	httpClient := http.Client{}
	request, err := http.NewRequest("GET", credentialURL, nil)
	if err != nil {
		log.Error(err, "Error forming http GET request")
		return "", err
	}
	request.Header = http.Header{
		"authorization": []string{fmt.Sprintf("Bearer %s", m.Spec.Astra.Token)},
		"content-type":  []string{"application/json"},
	}
	response, err := httpClient.Do(request)
	if err != nil {
		log.Error(err, "http GET error")
		return "", err
	}

	type Labels struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}

	type Metadata struct {
		Labels []Labels `json:"labels"`
	}

	type CredData struct {
		Name     string   `json:"name"`
		Valid    string   `json:"valid"`
		Metadata Metadata `json:"metadata"`
	}

	type CredItems struct {
		Items []CredData `json:"items"`
	}

	credItems := &CredItems{}
	err = json.NewDecoder(response.Body).Decode(credItems)
	if err != nil {
		log.Error(err, "error decoding response for credentials GET", "response", response.Body)
		return "", err
	}

	credTypeKey := "astra.netapp.io/labels/read-only/credType"
	cloudNameKey := "astra.netapp.io/labels/read-only/cloudName"
	validatedKey := "astra.netapp.io/labels/read-only/validated"

	for _, creds := range credItems.Items {
		var credTypeSAPresent bool
		var cloudNamePresent bool
		var validated bool

		for _, lables := range creds.Metadata.Labels {
			name := lables.Name
			value := lables.Value

			if name == credTypeKey && value == "service-account" {
				credTypeSAPresent = true
			}
			if name == cloudNameKey && value == common.AstraDefaultCloudType {
				cloudNamePresent = true
			}
			if name == validatedKey && value == "true" {
				validated = true
			}
		}

		if credTypeSAPresent && cloudNamePresent && validated {
			return creds.Name, nil
		}
	}
	return "", nil
}

func createCloud(astraHost string, m *v1.AstraConnector, ctx context.Context) (string, error) {
	log := ctrllog.FromContext(ctx)
	setTlsValidation(m.Spec.NatssyncClient.SkipTLSValidation, ctx)

	if m.Spec.NatssyncClient.HostAlias {
		cloudBridgeHost, err := getAstraHostFromURL(astraHost)
		if err != nil {
			return "", err
		}

		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}

		http.DefaultTransport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if addr == cloudBridgeHost+":443" {
				addr = m.Spec.NatssyncClient.HostAliasIP + ":443"
			}
			if addr == cloudBridgeHost+":80" {
				addr = m.Spec.NatssyncClient.HostAliasIP + ":80"
			}
			return dialer.DialContext(ctx, network, addr)
		}
	}

	cloudsURL := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds", astraHost, m.Spec.Astra.AccountID)
	httpClient := http.Client{}
	payLoad := map[string]string{
		"type":      "application/astra-cloud",
		"version":   "1.0",
		"name":      "Azure",
		"cloudType": common.AstraDefaultCloudType,
	}

	reqBodyBytes, err := json.Marshal(payLoad)
	request, err := http.NewRequest("POST", cloudsURL, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		log.Error(err, "Error forming http GET request")
		return "", err
	}
	request.Header = http.Header{
		"authorization": []string{fmt.Sprintf("Bearer %s", m.Spec.Astra.Token)},
		"content-type":  []string{"application/json"},
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return "", err
	}

	type CloudResp struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	cloudResp := &CloudResp{}
	err = json.NewDecoder(response.Body).Decode(cloudResp)
	if err != nil {
		log.Error(err, "error decoding response for cloud creation", "response", response.Body)
		return "", err
	}

	return cloudResp.ID, nil
}

func GetConnectorIDFromConfigMap(cmData map[string]string) (string, error) {
	var serviceKeyDataString string
	var serviceKeyData map[string]interface{}
	for key := range cmData {
		if key == "cloud-master_locationData.json" {
			continue
		}
		serviceKeyDataString = cmData[key]
		if err := json.Unmarshal([]byte(serviceKeyDataString), &serviceKeyData); err != nil {
			return "", err
		}
	}
	return serviceKeyData["locationID"].(string), nil
}

// GetNatssyncClientRegistrationURL returns NatssyncClient Registration URL
func GetNatssyncClientRegistrationURL(m *v1.AstraConnector) string {
	natsSyncClientURL := fmt.Sprintf("http://%s.%s:%d/bridge-client/1", common.NatssyncClientName, m.Namespace, common.NatssyncClientPort)
	natsSyncClientRegisterURL := fmt.Sprintf("%s/register", natsSyncClientURL)
	return natsSyncClientRegisterURL
}

// getNatssyncClientUnregisterURL returns NatssyncClient Unregister URL
func getNatssyncClientUnregisterURL(m *v1.AstraConnector) string {
	natsSyncClientURL := fmt.Sprintf("http://%s.%s:%d/bridge-client/1", common.NatssyncClientName, m.Namespace, common.NatssyncClientPort)
	natsSyncClientRegisterURL := fmt.Sprintf("%s/unregister", natsSyncClientURL)
	return natsSyncClientRegisterURL
}
