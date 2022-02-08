package controllers

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

	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	cachev1 "github.com/NetApp/astraagent-operator/api/v1"
)

const (
	errorRetrySleep time.Duration = time.Second * 3
)

func (r *AstraAgentReconciler) RegisterClient(m *cachev1.AstraAgent) (string, error) {
	natsSyncClientRegisterURL := r.getNatssyncClientRegistrationURL(m)
	reqBodyBytes, err := r.generateAuthPayload(m)
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
	type LocationId struct {
		LocationId string `json:"locationID"`
	}
	locationId := &LocationId{}
	err = json.NewDecoder(response.Body).Decode(locationId)
	if err != nil {
		return "", err
	}
	return locationId.LocationId, nil
}

func (r *AstraAgentReconciler) UnregisterClient(m *cachev1.AstraAgent) error {
	natsSyncClientUnregisterURL := r.getNatssyncClientUnregisterURL(m)
	reqBodyBytes, err := r.generateAuthPayload(m)
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

func (r *AstraAgentReconciler) generateAuthPayload(m *cachev1.AstraAgent) ([]byte, error) {
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

func (r *AstraAgentReconciler) AddLocationIDtoCloudExtension(m *cachev1.AstraAgent, locationID string, ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	setTlsValidation(m.Spec.NatssyncClient.SkipTLSValidation, ctx)

	astraHost := m.Spec.NatssyncClient.CloudBridgeURL
	if m.Spec.NatssyncClient.HostAlias {
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}

		http.DefaultTransport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if addr == strings.Split(astraHost, "://")[1]+":443" {
				addr = m.Spec.NatssyncClient.HostAliasIP + ":443"
			}
			if addr == strings.Split(astraHost, "://")[1]+":80" {
				addr = m.Spec.NatssyncClient.HostAliasIP + ":80"
			}
			return dialer.DialContext(ctx, network, addr)
		}
	}
	log.Info("Fetching cloud ID")
	cloudID, err := GetCloudId(astraHost, m.Spec.Astra.AccountID, m.Spec.Astra.Token, m.Spec.Astra.CloudType, ctx)
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

	// Register the locationId with Astra
	locationID = fmt.Sprintf("v1:%s", locationID)
	err = RegisterLocationId(astraHost, m.Spec.Astra.AccountID, m.Spec.Astra.Token, locationID, cloudID, clusterId, managedState, ctx)
	if err != nil {
		log.Error(err, "Error registering location ID with Astra")
		return err
	}
	return nil
}

func (r *AstraAgentReconciler) RemoveLocationIDFromCloudExtension(m *cachev1.AstraAgent, ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	setTlsValidation(m.Spec.NatssyncClient.SkipTLSValidation, ctx)

	astraHost := m.Spec.NatssyncClient.CloudBridgeURL
	if m.Spec.NatssyncClient.HostAlias {
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}

		http.DefaultTransport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if addr == strings.Split(astraHost, "://")[1]+":443" {
				addr = m.Spec.NatssyncClient.HostAliasIP + ":443"
			}
			if addr == strings.Split(astraHost, "://")[1]+":80" {
				addr = m.Spec.NatssyncClient.HostAliasIP + ":80"
			}
			return dialer.DialContext(ctx, network, addr)
		}
	}
	log.Info("Fetching cloud ID")
	cloudID, err := GetCloudId(astraHost, m.Spec.Astra.AccountID, m.Spec.Astra.Token, m.Spec.Astra.CloudType, ctx)
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

	// Unregister the locationId with Astra
	err = RegisterLocationId(astraHost, m.Spec.Astra.AccountID, m.Spec.Astra.Token, "", cloudID, clusterId, managedState, ctx)
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

func GetCluster(astraHost string, clusterID string, accountId string, cloudId string, token string, ctx context.Context) (*http.Response, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s/clusters/%s", astraHost, accountId, cloudId, clusterID)
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
	if cloudId == "" {
		return "", fmt.Errorf("Could not find cloud ID")
	}

	return cloudId, nil
}

func RegisterLocationId(astraCloudHost string, pcloudAccountId string, token string, locationId string, cloudId string, clusterId string, managedState string, ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	timeout := time.Second * 30
	success := false
	timeExpire := time.Now().Add(timeout)
	for time.Now().Before(timeExpire) {
		response, err := PostLocationId(astraCloudHost, pcloudAccountId, token, locationId, cloudId, clusterId, managedState, ctx)
		if err != nil {
			log.Error(err, "Error posting location ID")
			time.Sleep(errorRetrySleep)
			continue
		}
		if response.StatusCode == 200 { // Assuming the response code at this point
			log.Info("successfully registered locationId with Astra", "locationId", locationId)
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

func PostLocationId(astraCloudHost string, pcloudAccountId string, token string, locationId string, cloudId string, clusterId string, managedState string, ctx context.Context) (*http.Response, error) {
	log := ctrllog.FromContext(ctx)
	clusterUrl := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s/clusters/%s", astraCloudHost, pcloudAccountId, cloudId, clusterId)
	managedClusterUrl := fmt.Sprintf("%s/accounts/%s/topology/v1/managedClusters/%s", astraCloudHost, pcloudAccountId, clusterId)
	registerUrl := clusterUrl
	payLoad := map[string]string{
		"privateRouteID": locationId,
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

func setTlsValidation(disableTls string, ctx context.Context) {
	log := ctrllog.FromContext(ctx)
	if strings.ToLower(disableTls) == "true" {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		log.Info("TLS Validation Disabled! Not for use in production!")
	}
}

func (r *AstraAgentReconciler) checkCloudCreds(m *cachev1.AstraAgent, ctx context.Context) (string, error) {
	log := ctrllog.FromContext(ctx)
	setTlsValidation(m.Spec.NatssyncClient.SkipTLSValidation, ctx)

	if m.Spec.NatssyncClient.HostAlias {
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}

		http.DefaultTransport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if addr == strings.Split(m.Spec.NatssyncClient.CloudBridgeURL, "://")[1]+":443" {
				addr = m.Spec.NatssyncClient.HostAliasIP + ":443"
			}
			if addr == strings.Split(m.Spec.NatssyncClient.CloudBridgeURL, "://")[1]+":80" {
				addr = m.Spec.NatssyncClient.HostAliasIP + ":80"
			}
			return dialer.DialContext(ctx, network, addr)
		}
	}

	credentialURL := fmt.Sprintf("%s/accounts/%s/core/v1/credentials", m.Spec.NatssyncClient.CloudBridgeURL, m.Spec.Astra.AccountID)
	httpClient := http.Client{}
	request, err := http.NewRequest("GET", credentialURL, nil)
	if err != nil {
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

	//bodyBytes, err := ReadResponseBody(response)
	//if err != nil {
	//	log.Error(err, "Error reading credItems response body")
	//	return "", err
	//}
	credItems := &CredItems{}
	//err = json.Unmarshal(bodyBytes, &credItems)
	err = json.NewDecoder(response.Body).Decode(credItems)
	if err != nil {
		log.Error(err, "error unmarshaling credItems", "response", response.Body)
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
			if name == cloudNameKey && value == m.Spec.Astra.CloudType {
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
