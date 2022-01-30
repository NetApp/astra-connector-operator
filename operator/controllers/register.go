package controllers

import (
	"bytes"
	"context"
	"crypto/tls"
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
	reqBodyBytes, err := json.Marshal(map[string]string{"authToken": m.Spec.Astra.Token})
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
	reqBodyBytes, err := json.Marshal(map[string]string{"authToken": m.Spec.Astra.Token})
	response, err := http.Post(natsSyncClientUnregisterURL, "application/json", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return err
	}

	if response.StatusCode != 201 {
		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			return err
		}
		errMsg := fmt.Sprintf("Unexpected unregistration status code: %d; %s", response.StatusCode, string(bodyBytes))
		return errors.New(errMsg)
	}
	return nil
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
	clusterId, err := GetClusterId(astraHost, m.Spec.Astra.ClusterName, m.Spec.Astra.AccountID, cloudID, m.Spec.Astra.Token, ctx)
	if err != nil {
		log.Error(err, "Error fetching cluster ID")
		return err
	}
	log.Info("Found cluster ID", "clusterId", clusterId)

	// Register the locationId with Astra
	locationID = fmt.Sprintf("v1:%s", locationID)
	err = RegisterLocationId(astraHost, m.Spec.Astra.AccountID, m.Spec.Astra.Token, locationID, cloudID, clusterId, ctx)
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
	clusterId, err := GetClusterId(astraHost, m.Spec.Astra.ClusterName, m.Spec.Astra.AccountID, cloudID, m.Spec.Astra.Token, ctx)
	if err != nil {
		log.Error(err, "Error fetching cluster ID")
		return err
	}
	log.Info("Found cluster ID", "clusterId", clusterId)

	log.Info("Getting the cluster object")
	_, err = GetCluster(astraHost, clusterId, m.Spec.Astra.AccountID, cloudID, m.Spec.Astra.Token, ctx)
	if err != nil {
		log.Error(err, "Error fetching cluster object")
		return err
	}
	log.Info("Fetched the cluster object", "clusterId", clusterId)
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
	client := http.Client{}
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	request.Header = http.Header{
		"authorization": []string{fmt.Sprintf("Bearer %s", token)},
		"content-type":  []string{"application/json"},
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func GetClusterId(astraHost string, clusterName string, accountId string, cloudId string, token string, ctx context.Context) (string, error) {
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
		return "", fmt.Errorf("timed out querying Astra API")
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
		return "", err
	}
	resp := respData{}
	err = json.Unmarshal(bodyBytes, &resp)
	if err != nil {
		return "", err
	}
	// Find ID for given name
	var clusterId string
	for _, clusterData := range resp.Items {
		if clusterData.Name == clusterName {
			clusterId = clusterData.Id
			break
		}
	}
	if clusterId == "" {
		return "", fmt.Errorf("could not find cluster ID for cluster %s", clusterName)
	}

	return clusterId, nil
}

func GetCluster(astraHost string, clusterID string, accountId string, cloudId string, token string, ctx context.Context) (*http.Response, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s/clusters/%s", astraHost, accountId, cloudId, clusterID)
	client := http.Client{}
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	request.Header = http.Header{
		"authorization": []string{fmt.Sprintf("Bearer %s", token)},
		"content-type":  []string{"application/json"},
	}

	response, err := client.Do(request)
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

	client := http.Client{}
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	request.Header = http.Header{
		"authorization": []string{fmt.Sprintf("Bearer %s", token)},
		"content-type":  []string{"application/json"},
	}

	response, err := client.Do(request)
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

func RegisterLocationId(astraCloudHost string, pcloudAccountId string, token string, locationId string, cloudId string, clusterId string, ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	timeout := time.Second * 30
	success := false
	timeExpire := time.Now().Add(timeout)
	for time.Now().Before(timeExpire) {
		response, err := PostLocationId(astraCloudHost, pcloudAccountId, token, locationId, cloudId, clusterId)
		if err != nil {
			log.Error(err, "Error posting location ID")
			time.Sleep(errorRetrySleep)
			continue
		}
		if response.StatusCode == 200 { // Assuming the response code at this point
			log.Info("successfully registered locationId with Astra.")
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

func PostLocationId(astraCloudHost string, pcloudAccountId string, token string, locationId string, cloudId string, clusterId string) (*http.Response, error) {
	registerUrl := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s/clusters/%s", astraCloudHost, pcloudAccountId, cloudId, clusterId)
	reqBodyBytes, err := json.Marshal(map[string]string{"privateRouteID": locationId})

	client := http.Client{}
	request, err := http.NewRequest("PUT", registerUrl, bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return nil, err
	}

	request.Header = http.Header{
		"x-pcloud-accountid": []string{pcloudAccountId},
		"Content-Type":       []string{"application/json"},
		"x-pcloud-userid":    []string{"system"},
		"x-pcloud-role":      []string{"system"},
		"Authorization":      []string{fmt.Sprintf("Bearer %s", token)},
	}

	response, err := client.Do(request)
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
