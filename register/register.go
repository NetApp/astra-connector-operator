/*
 * Copyright (c) 2023. NetApp, Inc. All Rights Reserved.
 */

package register

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NetApp-Polaris/astra-connector-operator/common"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

const (
	errorRetrySleep       = time.Second * 3
	clusterUnManagedState = "unmanaged"
	clusterManagedState   = "managed"
	getClusterPollCount   = 5
)

// HTTPClient interface used for request and to facilitate testing
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// HeaderMap User specific details required for the http header
type HeaderMap struct {
	AccountID     string
	Authorization string
}

// DoRequest Makes http request with the given parameters
func DoRequest(ctx context.Context, client HTTPClient, method, url string, body io.Reader, headerMap HeaderMap) (*http.Response, error, context.CancelFunc) {
	// Child context that can't exceed a deadline specified
	childCtx, cancel := context.WithTimeout(ctx, 3*time.Minute) // TODO : Update timeout here

	req, _ := http.NewRequestWithContext(childCtx, method, url, body)

	req.Header.Add("Content-Type", "application/json")

	if headerMap.Authorization != "" {
		req.Header.Add("authorization", headerMap.Authorization)
	}

	httpResponse, err := client.Do(req)
	return httpResponse, err, cancel
}

type ClusterRegisterUtil interface {
	GetConnectorIDFromConfigMap(cmData map[string]string) (string, error)
	RegisterNatsSyncClient() (string, error)
	UnRegisterNatsSyncClient() error
	RegisterClusterWithAstra(astraConnectorId string) error
	DeleteClusterFromAstra() error
}

type clusterRegisterUtil struct {
	AstraConnector *v1.AstraConnector
	Client         HTTPClient
	K8sClient      client.Client
	Ctx            context.Context
	Log            logr.Logger
}

func NewClusterRegisterUtil(astraConnector *v1.AstraConnector, client HTTPClient, k8sClient client.Client, log logr.Logger, ctx context.Context) ClusterRegisterUtil {
	return &clusterRegisterUtil{
		AstraConnector: astraConnector,
		Client:         client,
		K8sClient:      k8sClient,
		Log:            log,
		Ctx:            ctx,
	}
}

// ******************************
//  FUNCTIONS TO REGISTER NATS
// ******************************

type AstraConnector struct {
	Id string `json:"locationID"`
}

// GetConnectorIDFromConfigMap Returns already registered ConnectorId
func (c clusterRegisterUtil) GetConnectorIDFromConfigMap(cmData map[string]string) (string, error) {
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

// getNatsSyncClientRegistrationURL Returns NatsSyncClient Registration URL
func (c clusterRegisterUtil) getNatsSyncClientRegistrationURL() string {
	natsSyncClientURL := fmt.Sprintf("http://%s.%s:%d/bridge-client/1", common.NatssyncClientName, c.AstraConnector.Namespace, common.NatssyncClientPort)
	natsSyncClientRegisterURL := fmt.Sprintf("%s/register", natsSyncClientURL)
	return natsSyncClientRegisterURL
}

// getNatsSyncClientUnregisterURL returns NatsSyncClient Unregister URL
func (c clusterRegisterUtil) getNatsSyncClientUnregisterURL() string {
	natsSyncClientURL := fmt.Sprintf("http://%s.%s:%d/bridge-client/1", common.NatssyncClientName, c.AstraConnector.Namespace, common.NatssyncClientPort)
	natsSyncClientRegisterURL := fmt.Sprintf("%s/unregister", natsSyncClientURL)
	return natsSyncClientRegisterURL
}

// generateAuthPayload Returns the payload for authentication
func (c clusterRegisterUtil) generateAuthPayload() ([]byte, error) {
	authPayload, err := json.Marshal(map[string]string{
		"userToken": c.AstraConnector.Spec.ConnectorSpec.Astra.Token,
		"accountId": c.AstraConnector.Spec.ConnectorSpec.Astra.AccountID,
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

// UnRegisterNatsSyncClient Unregisters NatsSyncClient
func (c clusterRegisterUtil) UnRegisterNatsSyncClient() error {
	natsSyncClientUnregisterURL := c.getNatsSyncClientUnregisterURL()
	reqBodyBytes, err := c.generateAuthPayload()
	if err != nil {
		return err
	}

	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodPost, natsSyncClientUnregisterURL, bytes.NewBuffer(reqBodyBytes), HeaderMap{})
	defer cancel()

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

// RegisterNatsSyncClient Registers NatsSyncClient with NatsSyncServer
func (c clusterRegisterUtil) RegisterNatsSyncClient() (string, error) {
	natsSyncClientRegisterURL := c.getNatsSyncClientRegistrationURL()
	reqBodyBytes, err := c.generateAuthPayload()
	if err != nil {
		return "", err
	}

	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodPost, natsSyncClientRegisterURL, bytes.NewBuffer(reqBodyBytes), HeaderMap{})
	defer cancel()
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

	astraConnector := &AstraConnector{}
	err = json.NewDecoder(response.Body).Decode(astraConnector)
	if err != nil {
		return "", err
	}

	return astraConnector.Id, nil
}

// ************************************************
//  FUNCTIONS TO REGISTER CLUSTER WITH ASTRA
// ************************************************

func GetAstraHostURL(astraConnector *v1.AstraConnector) string {
	var astraHost string
	if astraConnector.Spec.ConnectorSpec.NatssyncClient.CloudBridgeURL != "" {
		astraHost = astraConnector.Spec.ConnectorSpec.NatssyncClient.CloudBridgeURL
	} else {
		astraHost = common.NatssyncClientDefaultCloudBridgeURL
	}

	return astraHost
}

func (c clusterRegisterUtil) getAstraHostFromURL(astraHostURL string) (string, error) {
	cloudBridgeURLSplit := strings.Split(astraHostURL, "://")
	if len(cloudBridgeURLSplit) != 2 {
		errStr := fmt.Sprintf("invalid cloudBridgeURL provided: %s, format - https://hostname", astraHostURL)
		return "", errors.New(errStr)
	}
	return cloudBridgeURLSplit[1], nil
}

func (c clusterRegisterUtil) logHttpError(response *http.Response) {
	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		c.Log.Error(err, "Error reading response body")
	} else {
		c.Log.Info("Received unexpected status code", "responseBody", string(bodyBytes), "statusCode", response.StatusCode)
		err = response.Body.Close()
		if err != nil {
			c.Log.Error(err, "Error closing the response body")
		}
	}
}

func (c clusterRegisterUtil) readResponseBody(response *http.Response) ([]byte, error) {
	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return bodyBytes, nil
}

func (c clusterRegisterUtil) setHttpClient(disableTls bool, astraHost string) error {
	if disableTls {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		c.Log.WithValues("disableTls", disableTls).Info("TLS Validation Disabled! Not for use in production!")
	}

	if c.AstraConnector.Spec.ConnectorSpec.NatssyncClient.HostAlias {
		c.Log.WithValues("HostAliasIP", c.AstraConnector.Spec.ConnectorSpec.NatssyncClient.HostAlias).Info("Using the HostAlias IP")
		cloudBridgeHost, err := c.getAstraHostFromURL(astraHost)
		if err != nil {
			return err
		}

		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}

		http.DefaultTransport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if addr == cloudBridgeHost+":443" {
				addr = c.AstraConnector.Spec.ConnectorSpec.NatssyncClient.HostAliasIP + ":443"
			}
			if addr == cloudBridgeHost+":80" {
				addr = c.AstraConnector.Spec.ConnectorSpec.NatssyncClient.HostAliasIP + ":80"
			}
			return dialer.DialContext(ctx, network, addr)
		}
	}

	c.Client = &http.Client{}
	return nil
}

// ************************************************
//  FUNCTIONS RELATED TO CLOUD ENDPOINTS
// ************************************************

func (c clusterRegisterUtil) cloudExists(astraHost, cloudID string) bool {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s", astraHost, c.AstraConnector.Spec.ConnectorSpec.Astra.AccountID, cloudID)

	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", c.AstraConnector.Spec.ConnectorSpec.Astra.Token)}
	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodGet, url, nil, headerMap)
	defer cancel()

	if err != nil {
		c.Log.Error(err, "Error getting Cloud: "+cloudID)
		return false
	}

	if response.StatusCode == http.StatusNotFound {
		c.Log.Info("Cloud Not Found: " + cloudID)
		return false
	}

	if response.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("Get Clouds call returned with status code: %v", response.StatusCode)
		c.Log.Error(errors.New("Invalid Status Code"), msg)
		return false
	}

	c.Log.Info("Cloud Found: " + cloudID)
	return true
}

func (c clusterRegisterUtil) listClouds(astraHost string) (*http.Response, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds", astraHost, c.AstraConnector.Spec.ConnectorSpec.Astra.AccountID)

	c.Log.Info("Getting clouds")
	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", c.AstraConnector.Spec.ConnectorSpec.Astra.Token)}
	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodGet, url, nil, headerMap)
	defer cancel()

	if err != nil {
		return nil, err
	}

	return response, nil
}

func (c clusterRegisterUtil) getCloudId(astraHost, cloudType string) (string, error) {
	success := false
	var response *http.Response
	timeout := time.Second * 30
	timeExpire := time.Now().Add(timeout)

	for time.Now().Before(timeExpire) {
		var err error
		response, err = c.listClouds(astraHost)
		if err != nil {
			c.Log.Error(err, "Error listing clouds")
			time.Sleep(errorRetrySleep)
			continue
		}

		if response.StatusCode == 200 {
			success = true
			break
		}

		c.logHttpError(response)
		_ = response.Body.Close()
		time.Sleep(errorRetrySleep)
	}

	if !success {
		return "", fmt.Errorf("timed out querying Astra API")
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(response.Body)

	type respData struct {
		Items []struct {
			CloudType string `json:"cloudType"`
			Id        string `json:"id"`
		} `json:"items"`
	}

	bodyBytes, err := c.readResponseBody(response)
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

func (c clusterRegisterUtil) createCloud(astraHost, cloudType string) (string, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds", astraHost, c.AstraConnector.Spec.ConnectorSpec.Astra.AccountID)
	payLoad := map[string]string{
		"type":      "application/astra-cloud",
		"version":   "1.0",
		"name":      common.AstraPrivateCloudName,
		"cloudType": cloudType,
	}

	reqBodyBytes, err := json.Marshal(payLoad)

	c.Log.WithValues("cloudType", cloudType).Info("Creating cloud")
	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", c.AstraConnector.Spec.ConnectorSpec.Astra.Token)}
	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodPost, url, bytes.NewBuffer(reqBodyBytes), headerMap)
	defer cancel()

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
		c.Log.Error(err, "error decoding response for cloud creation", "response", response.Body)
		return "", err
	}

	return cloudResp.ID, nil
}

func (c clusterRegisterUtil) getOrCreateCloud(astraHost string) (string, error) {
	// If a cloudId is specified in the CR Spec, validate its existence.
	// If the provided cloudId is valid, return the same.
	// If it is not a valid cloudId i.e., provided cloudId doesn't exist in the DB, return an error
	cloudId := c.AstraConnector.Spec.ConnectorSpec.Astra.CloudID
	if cloudId != "" {
		c.Log.WithValues("cloudID", cloudId).Info("Validating the provided CloudId")
		if !c.cloudExists(astraHost, cloudId) {
			return "", errors.New("Invalid CloudId provided in the Spec : " + cloudId)
		}

		c.Log.WithValues("cloudID", cloudId).Info("CloudId exists in the system")
		return cloudId, nil
	}

	// When a cloudId is not specified in the CR Spec, check if a cloud of type "private"
	// exists in the system. If it exists, return the CloudId of the "private" cloud.
	// Otherwise, proceed to create a cloud of type "private" and the return the CloudId
	// of the newly created cloud.
	c.Log.Info("Fetching Cloud Id of type " + common.AstraPrivateCloudType)

	cloudId, err := c.getCloudId(astraHost, common.AstraPrivateCloudType)
	if err != nil {
		c.Log.Error(err, "Error fetching cloud ID")
		return "", err
	}

	if cloudId == "" {
		c.Log.Info("Cloud doesn't seem to exist, creating the cloud", "cloudType", common.AstraPrivateCloudType)
		cloudId, err = c.createCloud(astraHost, common.AstraPrivateCloudType)
		if err != nil {
			c.Log.Error(err, "Failed to create cloud", "cloudType", common.AstraPrivateCloudType)
			return "", err
		}
		if cloudId == "" {
			return "", fmt.Errorf("could not create cloud %s", common.AstraPrivateCloudType)
		}
	}

	c.Log.WithValues("cloudID", cloudId).Info("Found/Created Cloud")

	return cloudId, nil
}

// CLUSTER ENDPOINT RELATED FUNCTIONS

type Cluster struct {
	Type                       string   `json:"type,omitempty"`
	Version                    string   `json:"version,omitempty"`
	ID                         string   `json:"id,omitempty"`
	Name                       string   `json:"name,omitempty"`
	ManagedState               string   `json:"managedState,omitempty"`
	ClusterType                string   `json:"clusterType,omitempty"`
	CloudID                    string   `json:"cloudID,omitempty"`
	PrivateRouteID             string   `json:"privateRouteID,omitempty"`
	ConnectorCapabilities      []string `json:"connectorCapabilities,omitempty"`
	TridentManagedStateDesired string   `json:"tridentManagedStateDesired,omitempty"`
	ApiServiceID               string   `json:"apiServiceID,omitempty"`
	DefaultStorageClass        string   `json:"defaultStorageClass,omitempty"`
}

type GetClustersResponse struct {
	Items []Cluster `json:"items"`
}

type ClusterInfo struct {
	ID           string
	Name         string
	ManagedState string
}

// getClusters Returns a list of existing clusters
func (c clusterRegisterUtil) getClusters(astraHost, cloudId string) (GetClustersResponse, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s/clusters", astraHost, c.AstraConnector.Spec.ConnectorSpec.Astra.AccountID, cloudId)
	var clustersRespJson GetClustersResponse

	c.Log.Info("Getting Clusters")

	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", c.AstraConnector.Spec.ConnectorSpec.Astra.Token)}
	clustersResp, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodGet, url, nil, headerMap)
	defer cancel()

	if err != nil {
		return clustersRespJson, errors.Wrap(err, "error on request get clusters")
	}

	if clustersResp.StatusCode != http.StatusOK {
		return clustersRespJson, errors.New("get clusters failed " + strconv.Itoa(clustersResp.StatusCode))
	}

	respBody, err := io.ReadAll(clustersResp.Body)
	if err != nil {
		return clustersRespJson, errors.Wrap(err, "error reading response from get clusters")
	}

	err = json.Unmarshal(respBody, &clustersRespJson)
	if err != nil {
		return clustersRespJson, errors.Wrap(err, "unmarshall error when getting clusters")
	}

	return clustersRespJson, nil
}

// pollForClusterToBeInDesiredState Polls until a given cluster is in desired state (or until timeout)
func (c clusterRegisterUtil) pollForClusterToBeInDesiredState(astraHost, cloudId, clusterId, desiredState string) error {
	for i := 1; i <= getClusterPollCount; i++ {
		time.Sleep(15 * time.Second)
		getCluster, getClusterErr := c.getCluster(astraHost, cloudId, clusterId)

		if getClusterErr != nil {
			return errors.Wrap(getClusterErr, "error on get cluster")
		}

		if getCluster.ManagedState == desiredState {
			return nil
		}
	}
	return errors.New("cluster state not changed to desired state: " + clusterId)
}

// getCluster Returns the details of the given clusterID (if it exists)
func (c clusterRegisterUtil) getCluster(astraHost, cloudId, clusterId string) (Cluster, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s/clusters/%s", astraHost, c.AstraConnector.Spec.ConnectorSpec.Astra.AccountID, cloudId, clusterId)
	var clustersRespJson Cluster

	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", c.AstraConnector.Spec.ConnectorSpec.Astra.Token)}
	clustersResp, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodGet, url, nil, headerMap)
	defer cancel()

	if err != nil {
		return Cluster{}, errors.Wrap(err, "error on request get managed clusters")
	}

	if clustersResp.StatusCode != http.StatusOK {
		return Cluster{}, errors.New("get managed clusters failed with: " + strconv.Itoa(clustersResp.StatusCode))
	}

	respBody, err := io.ReadAll(clustersResp.Body)
	if err != nil {
		return Cluster{}, errors.Wrap(err, "error reading response from get managed clusters")
	}

	err = json.Unmarshal(respBody, &clustersRespJson)
	if err != nil {
		return Cluster{}, errors.Wrap(err, "unmarshall error when parsing get managed clusters response")
	}

	return clustersRespJson, nil
}

// createCluster Creates a cluster with the provided details
func (c clusterRegisterUtil) createCluster(astraHost, cloudId, astraConnectorId string) (ClusterInfo, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s/clusters", astraHost, c.AstraConnector.Spec.ConnectorSpec.Astra.AccountID, cloudId)
	var clustersRespJson Cluster

	clustersBody := Cluster{
		Type:                  "application/astra-cluster",
		Version:               common.AstraClustersAPIVersion,
		Name:                  c.AstraConnector.Spec.ConnectorSpec.Astra.ClusterName,
		ConnectorCapabilities: common.GetConnectorCapabilities(),
		PrivateRouteID:        astraConnectorId,
	}

	clustersBodyJson, _ := json.Marshal(clustersBody)
	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", c.AstraConnector.Spec.ConnectorSpec.Astra.Token)}
	clustersResp, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodPost, url, bytes.NewBuffer(clustersBodyJson), headerMap)
	defer cancel()

	if err != nil {
		return ClusterInfo{}, errors.Wrap(err, "error on request post clusters")
	}

	if clustersResp.StatusCode != http.StatusCreated {
		return ClusterInfo{}, errors.New("add cluster failed with: " + strconv.Itoa(clustersResp.StatusCode))
	}

	respBody, err := io.ReadAll(clustersResp.Body)
	if err != nil {
		return ClusterInfo{}, errors.Wrap(err, "error reading response from post clusters")
	}

	err = json.Unmarshal(respBody, &clustersRespJson)
	if err != nil {
		return ClusterInfo{}, errors.Wrap(err, "unmarshall error when parsing post clusters response")
	}

	if clustersRespJson.ID == "" {
		return ClusterInfo{}, errors.New("got empty id from post clusters response")
	}

	if clustersRespJson.ManagedState == clusterUnManagedState {
		c.Log.Info("Cluster added to Astra", "clusterId", clustersRespJson.ID)
		return ClusterInfo{ID: clustersRespJson.ID, ManagedState: clustersRespJson.ManagedState, Name: clustersRespJson.Name}, nil
	}

	err = c.pollForClusterToBeInDesiredState(astraHost, cloudId, clustersRespJson.ID, clusterUnManagedState)
	if err == nil {
		c.Log.Info("Cluster added to Astra", "clusterId", clustersRespJson.ID)
		return ClusterInfo{ID: clustersRespJson.ID, ManagedState: clustersRespJson.ManagedState, Name: clustersRespJson.Name}, nil
	}

	return ClusterInfo{}, errors.New("cluster state not changed to desired state")
}

// updateCluster Updates an existing cluster with the provided details
func (c clusterRegisterUtil) updateCluster(astraHost, cloudId, clusterId, astraConnectorId string) error {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s/clusters/%s", astraHost, c.AstraConnector.Spec.ConnectorSpec.Astra.AccountID, cloudId, clusterId)

	clustersBody := Cluster{
		Type:                  "application/astra-cluster",
		Version:               common.AstraClustersAPIVersion,
		Name:                  c.AstraConnector.Spec.ConnectorSpec.Astra.ClusterName,
		ConnectorCapabilities: common.GetConnectorCapabilities(),
		PrivateRouteID:        astraConnectorId,
	}

	clustersBodyJson, _ := json.Marshal(clustersBody)
	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", c.AstraConnector.Spec.ConnectorSpec.Astra.Token)}
	clustersResp, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodPut, url, bytes.NewBuffer(clustersBodyJson), headerMap)
	defer cancel()

	if err != nil {
		return errors.Wrap(err, "error on request put clusters")
	}

	if clustersResp.StatusCode > http.StatusNoContent {
		return errors.New("update cluster failed with: " + strconv.Itoa(clustersResp.StatusCode))
	}

	c.Log.WithValues("clusterId", clusterId).Info("Cluster updated")
	return nil
}

func (c clusterRegisterUtil) createOrUpdateCluster(astraHost, cloudId, clusterId, astraConnectorId, clustersMethod string) (ClusterInfo, error) {
	if clustersMethod == http.MethodPut {
		c.Log.WithValues("clusterId", clusterId).Info("Updating cluster")

		err := c.updateCluster(astraHost, cloudId, clusterId, astraConnectorId)
		if err != nil {
			return ClusterInfo{}, errors.Wrap(err, "error updating cluster")
		}

		return ClusterInfo{ID: clusterId}, nil
	}

	if clustersMethod == http.MethodPost {
		c.Log.Info("Creating Cluster")

		clusterInfo, err := c.createCluster(astraHost, cloudId, astraConnectorId)
		if err != nil {
			return ClusterInfo{}, errors.Wrap(err, "error creating cluster")
		}

		return clusterInfo, nil
	}

	c.Log.Info("Create/Update cluster not required!")
	return ClusterInfo{ID: clusterId}, nil
}

type StorageClass struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	IsDefault string `json:"isDefault"`
}

type GetStorageClassResponse struct {
	Items []StorageClass `json:"items"`
}

func (c clusterRegisterUtil) getStorageClass(astraHost, cloudId, clusterId string) (string, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s/clusters/%s/storageClasses", astraHost, c.AstraConnector.Spec.ConnectorSpec.Astra.AccountID, cloudId, clusterId)
	var storageClassesRespJson GetStorageClassResponse

	c.Log.Info("Getting Storage Classes")

	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", c.AstraConnector.Spec.ConnectorSpec.Astra.Token)}
	storageClassesResp, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodGet, url, nil, headerMap)
	defer cancel()

	if err != nil {
		return "", errors.Wrap(err, "error on request get storage classes")
	}

	if storageClassesResp.StatusCode != http.StatusOK {
		return "", errors.New("get storage classes failed " + strconv.Itoa(storageClassesResp.StatusCode))
	}

	respBody, err := io.ReadAll(storageClassesResp.Body)
	if err != nil {
		return "", errors.Wrap(err, "error reading response from get storage classes")
	}

	err = json.Unmarshal(respBody, &storageClassesRespJson)
	if err != nil {
		return "", errors.Wrap(err, "unmarshall error when getting storage classes")
	}

	var defaultStorageClassId string
	var defaultStorageClassName string
	for _, sc := range storageClassesRespJson.Items {
		if sc.Name == c.AstraConnector.Spec.ConnectorSpec.Astra.StorageClassName {
			c.Log.Info("Using the storage class specified in the CR Spec", "StorageClassName", sc.Name, "StorageClassID", sc.ID)
			return sc.ID, nil
		}

		if sc.IsDefault == "true" {
			defaultStorageClassId = sc.ID
			defaultStorageClassName = sc.Name
		}
	}

	if c.AstraConnector.Spec.ConnectorSpec.Astra.StorageClassName != "" {
		c.Log.Error(errors.New("invalid storage class specified"), "Storage Class Provided in the CR Spec is not valid : "+c.AstraConnector.Spec.ConnectorSpec.Astra.StorageClassName)
	}

	if defaultStorageClassId == "" {
		c.Log.Info("No Storage Class is set to default")
		return "", errors.New("no default storage class in the system")
	}

	c.Log.Info("Using the default storage class", "StorageClassName", defaultStorageClassName, "StorageClassID", defaultStorageClassId)
	return defaultStorageClassId, nil
}

// updateManagedCluster Updates the persisted record of the given managed cluster
func (c clusterRegisterUtil) updateManagedCluster(astraHost, clusterId, astraConnectorId, storageClass string) error {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/managedClusters/%s", astraHost, c.AstraConnector.Spec.ConnectorSpec.Astra.AccountID, clusterId)

	manageClustersBody := Cluster{
		Type:                  "application/astra-managedCluster",
		Version:               common.AstraManagedClustersAPIVersion,
		ConnectorCapabilities: common.GetConnectorCapabilities(),
		PrivateRouteID:        astraConnectorId,
		//DefaultStorageClass:   storageClass,
	}
	manageClustersBodyJson, _ := json.Marshal(manageClustersBody)

	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", c.AstraConnector.Spec.ConnectorSpec.Astra.Token)}
	manageClustersResp, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodPut, url, bytes.NewBuffer(manageClustersBodyJson), headerMap)
	defer cancel()

	if err != nil {
		return errors.Wrap(err, "error on request put manage clusters")
	}

	if manageClustersResp.StatusCode > http.StatusNoContent {
		return errors.New("manage cluster failed with: " + strconv.Itoa(manageClustersResp.StatusCode))
	}

	c.Log.WithValues("clusterId", clusterId).Info("Managed Cluster updated")
	return nil
}

// createManagedCluster Transitions a cluster from unmanaged state to managed state
func (c clusterRegisterUtil) createManagedCluster(astraHost, cloudId, clusterID, storageClass string) error {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/managedClusters", astraHost, c.AstraConnector.Spec.ConnectorSpec.Astra.AccountID)
	var manageClustersRespJson Cluster

	manageClustersBody := Cluster{
		Type:                       "application/astra-managedCluster",
		Version:                    common.AstraManagedClustersAPIVersion,
		ID:                         clusterID,
		TridentManagedStateDesired: clusterManagedState,
		//DefaultStorageClass:        storageClass,
	}
	manageClustersBodyJson, _ := json.Marshal(manageClustersBody)

	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", c.AstraConnector.Spec.ConnectorSpec.Astra.Token)}
	manageClustersResp, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodPost, url, bytes.NewBuffer(manageClustersBodyJson), headerMap)
	defer cancel()

	if err != nil {
		return errors.Wrap(err, "error on request post manage clusters")
	}

	if manageClustersResp.StatusCode != http.StatusCreated {
		return errors.New("manage cluster failed with: " + strconv.Itoa(manageClustersResp.StatusCode))
	}

	respBody, err := io.ReadAll(manageClustersResp.Body)
	if err != nil {
		return errors.Wrap(err, "error reading response from post manage clusters")
	}

	err = json.Unmarshal(respBody, &manageClustersRespJson)
	if err != nil {
		return errors.Wrap(err, "unmarshall error when parsing post manage clusters response")
	}

	if manageClustersRespJson.ManagedState == clusterManagedState {
		c.Log.WithValues("clusterId", manageClustersRespJson.ID).Info("Cluster Managed")
		return nil
	}

	err = c.pollForClusterToBeInDesiredState(astraHost, cloudId, clusterID, clusterManagedState)
	if err == nil {
		return nil
	}

	return errors.New("cluster state not changed to managed")
}

func (c clusterRegisterUtil) createOrUpdateManagedCluster(astraHost, cloudId, clusterId, astraConnectorId, managedClustersMethod string) (ClusterInfo, error) {
	// Get a Storage Class to be used for managing a cluster
	//time.Sleep(10 * time.Second)
	//storageClass, err := c.getStorageClass(astraHost, cloudId, clusterId)
	//if err != nil {
	//	return ClusterInfo{}, err
	//}

	storageClass := ""

	if managedClustersMethod == http.MethodPut {
		c.Log.Info("Updating Managed Cluster")

		err := c.updateManagedCluster(astraHost, clusterId, astraConnectorId, storageClass)
		if err != nil {
			return ClusterInfo{}, errors.Wrap(err, "error updating managed cluster")
		}

		return ClusterInfo{ID: clusterId, ManagedState: clusterManagedState}, nil
	}

	if managedClustersMethod == http.MethodPost {
		c.Log.Info("Creating Managed Cluster")

		err := c.createManagedCluster(astraHost, cloudId, clusterId, storageClass)
		if err != nil {
			return ClusterInfo{}, errors.Wrap(err, "error creating managed cluster")
		}

		return ClusterInfo{ID: clusterId, ManagedState: clusterManagedState}, nil
	}

	c.Log.Info("Create/Update managed cluster not required!")
	return ClusterInfo{ID: clusterId}, nil
}

func (c clusterRegisterUtil) validateAndGetCluster(astraHost, cloudId string) (ClusterInfo, error) {
	// If a clusterId is specified in the CR Spec, validate its existence.
	// If the provided clusterId exists in the DB, return the details of that cluster, otherwise return an error
	clusterId := c.AstraConnector.Spec.ConnectorSpec.Astra.ClusterID
	if clusterId != "" {
		c.Log.WithValues("cloudID", cloudId, "clusterID", clusterId).Info("Validating the provided ClusterId")
		getClusterResp, err := c.getCluster(astraHost, cloudId, clusterId)
		if err != nil {
			return ClusterInfo{}, errors.Wrap(err, "error on get cluster")
		}

		if getClusterResp.ID == "" {
			return ClusterInfo{}, errors.New("Invalid ClusterId provided in the Spec : " + clusterId)
		}

		c.Log.WithValues("cloudID", cloudId, "clusterID", clusterId).Info("ClusterId exists in the system")
		return ClusterInfo{ID: clusterId, Name: getClusterResp.Name, ManagedState: getClusterResp.ManagedState}, nil
	}

	// Check whether a cluster exists with a matching "apiServiceID"
	// Get all clusters and validate whether any of the response matches with the current cluster's "ServiceUUID"
	k8sService := &corev1.Service{}
	err := c.K8sClient.Get(c.Ctx, types.NamespacedName{Name: "kubernetes", Namespace: "default"}, k8sService)
	if err != nil {
		c.Log.Error(err, "Failed to get kubernetes service from default namespace")
		return ClusterInfo{}, err
	}
	k8sServiceUUID := string(k8sService.ObjectMeta.UID)
	c.Log.Info(fmt.Sprintf("Kubernetes service UUID is %s", k8sServiceUUID))

	// Check whether a cluster exists with the above "k8sServiceUUID" as "apiServiceID"
	getClustersResp, err := c.getClusters(astraHost, cloudId)
	if err != nil {
		return ClusterInfo{}, errors.Wrap(err, "error on get clusters")
	}

	c.Log.WithValues("cloudID", cloudId).Info("Checking existing records for current cluster's record")
	for _, value := range getClustersResp.Items {
		if value.ApiServiceID == k8sServiceUUID {
			c.Log.WithValues("ClusterId", value.ID, "Name", value.Name, "ManagedState", value.ManagedState).Info("Cluster Info found in the existing records")
			return ClusterInfo{ID: value.ID, Name: value.Name, ManagedState: value.ManagedState}, nil
		}
	}

	// This is the case for creation of cluster with POST calls to /clusters and /managedClusters
	c.Log.WithValues("cloudID", cloudId).Info("ClusterId not specified in CR Spec and an existing cluster doesn't exist in the system")
	return ClusterInfo{}, nil
}

// RegisterClusterWithAstra Adds the cluster to Astra
func (c clusterRegisterUtil) RegisterClusterWithAstra(astraConnectorId string) error {
	astraHost := GetAstraHostURL(c.AstraConnector)
	c.Log.WithValues("URL", astraHost).Info("Astra Host Info")

	err := c.setHttpClient(c.AstraConnector.Spec.ConnectorSpec.Astra.SkipTLSValidation, astraHost)
	if err != nil {
		return err
	}

	// 1. Checks the existence of cloud in the system with the cloudId (if it was specified in the CR Spec)
	//    If the CloudId was specified and the cloud exists in the system, the same cloudId is returned.
	//    If the CloudId was specified and the cloud doesn't exist in the system, an error is returned.
	// 2. If the CloudId was not specified in the CR Spec, checks whether a cloud of type "private"
	//    exists in the system, if so returns the cloudId of the "private" cloud. Otherwise, a new cloud of
	//    type "private" is created and the cloudId is returned.
	cloudId, err := c.getOrCreateCloud(astraHost)
	if err != nil {
		return err
	}

	// 1. Checks the existence of cluster in the system with the clusterId (if it was specified in the CR Spec)
	//    If the ClusterId was specified and the cluster exists in the system, details related to that cluster are returned.
	//    If the ClusterId was specified and the cluster doesn't exist in the system, an error is returned.
	// 2. If the ClusterId was not specified in the CR Spec, checks the existence of a cluster in the system
	//    with "K8s Service UUID" of the current cluster as "ApiServiceID" field value. If there exists such a record,
	//    details related to that cluster will be returned. Otherwise, empty cluster details will be returned
	clusterInfo, err := c.validateAndGetCluster(astraHost, cloudId)
	if err != nil {
		return err
	}

	var clustersMethod, managedClustersMethod string
	if clusterInfo.ID != "" {
		// clusterInfo.ID != "" ====>
		// 1. ClusterId specified in the CR Spec, and it is present in the system
		// 							OR
		// 2. A cluster record with matching "apiServiceID" is present in the system
		c.Log.WithValues("cloudID", cloudId, "clusterID", clusterInfo.ID, "clusterManagedState", clusterInfo.ManagedState).Info("Cluster exists in the system, updating the existing cluster")
		if clusterInfo.ManagedState == clusterUnManagedState {
			clustersMethod = http.MethodPut         // PUT /clusters to update the record
			managedClustersMethod = http.MethodPost // POST /managedClusters to create a new managed record
		} else {
			clustersMethod = ""                    // no call on /clusters
			managedClustersMethod = http.MethodPut // PUT /managedClusters to update the record
		}
	} else {
		// Case where clusterId was not specified in the CR Spec
		// and a cluster with matching "apiServiceID" was not found
		c.Log.Info("Cluster doesn't exist in the system, creating a new cluster and managing it")
		clustersMethod = http.MethodPost
		managedClustersMethod = http.MethodPost
	}

	// Adding or Updating a Cluster based on the status from above
	clusterInfo, err = c.createOrUpdateCluster(astraHost, cloudId, clusterInfo.ID, astraConnectorId, clustersMethod)
	if err != nil {
		return err
	}

	// Adding or Updating Managed Cluster based on the status from above
	clusterInfo, err = c.createOrUpdateManagedCluster(astraHost, cloudId, clusterInfo.ID, astraConnectorId, managedClustersMethod)
	if err != nil {
		return err
	}

	c.Log.WithValues("clusterId", clusterInfo.ID, "clusterName", clusterInfo.Name).Info("Cluster managed by Astra!!!!")
	return nil
}

// RemoveCluster Removes a cluster from persistence
func (c clusterRegisterUtil) RemoveCluster(astraHost, cloudId, clusterId string) error {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s/clusters/%s", astraHost, c.AstraConnector.Spec.ConnectorSpec.Astra.AccountID, cloudId, clusterId)

	c.Log.WithValues("clusterID", clusterId).Info("Removing Cluster")

	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", c.AstraConnector.Spec.ConnectorSpec.Astra.Token)}
	removeClusterResp, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodDelete, url, nil, headerMap)
	defer cancel()

	if err != nil {
		return errors.Wrap(err, "error on request delete cluster with id: "+clusterId)
	}

	if removeClusterResp.StatusCode != http.StatusNoContent {
		return errors.New("remove cluster failed with statusCode: " + strconv.Itoa(removeClusterResp.StatusCode))
	}

	return nil
}

// UnManageCluster UnManages a cluster with given clusterID
func (c clusterRegisterUtil) UnManageCluster(astraHost, clusterId string) error {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/managedClusters/%s", astraHost, c.AstraConnector.Spec.ConnectorSpec.Astra.AccountID, clusterId)

	c.Log.WithValues("clusterID", clusterId).Info("UnManaging Cluster")

	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", c.AstraConnector.Spec.ConnectorSpec.Astra.Token)}
	unManageClusterResp, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodDelete, url, nil, headerMap)
	defer cancel()

	if err != nil {
		return errors.Wrap(err, "error on request delete managedCluster with id: "+clusterId)
	}

	if unManageClusterResp.StatusCode != http.StatusNoContent {
		return errors.New("unManage cluster failed with statusCode: " + strconv.Itoa(unManageClusterResp.StatusCode))
	}

	return nil
}

// DeleteClusterFromAstra UnManages and Deletes the cluster from Astra
func (c clusterRegisterUtil) DeleteClusterFromAstra() error {
	astraHost := GetAstraHostURL(c.AstraConnector)
	c.Log.WithValues("URL", astraHost).Info("Astra Host Info")

	err := c.setHttpClient(c.AstraConnector.Spec.ConnectorSpec.Astra.SkipTLSValidation, astraHost)
	if err != nil {
		return err
	}

	// Get the cloudId
	cloudId, err := c.getCloudId(astraHost, common.AstraPrivateCloudType)
	if err != nil {
		return errors.Wrap(err, "error getting cloudId")
	}

	// Get clusters and check for a cluster with required cluster name and private route id
	// to be unmanaged and unregistered
	clustersResp, err := c.getClusters(astraHost, cloudId)
	if err != nil {
		return errors.Wrap(err, "error getting clusters")
	}

	var clusterInfo ClusterInfo
	for _, value := range clustersResp.Items {
		if value.Name == c.AstraConnector.Spec.ConnectorSpec.Astra.ClusterName {
			c.Log.WithValues("ClusterId", value.ID, "Name", value.Name, "ManagedState", value.ManagedState).Info("Found the required cluster info")
			clusterInfo = ClusterInfo{ID: value.ID, ManagedState: value.ManagedState, Name: value.Name}
		}
	}

	if clusterInfo.ID == "" {
		return errors.New("Required cluster not found")
	}

	if clusterInfo.ManagedState == clusterManagedState {
		if err = c.UnManageCluster(astraHost, clusterInfo.ID); err != nil {
			return errors.Wrap(err, "error unManaging cluster")
		}
	} else {
		c.Log.WithValues("clusterId", clusterInfo.ID, "clusterName", clusterInfo.Name).Info("Cluster already unManaged")
	}

	if err = c.RemoveCluster(astraHost, cloudId, clusterInfo.ID); err != nil {
		return errors.Wrap(err, "error removing cluster")
	}

	c.Log.WithValues("clusterId", clusterInfo.ID, "clusterName", clusterInfo.Name).Info("Cluster unregistered with Astra")
	return nil
}
