/*
 * Copyright (c) 2024. NetApp, Inc. All Rights Reserved.
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
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NetApp-Polaris/astra-connector-operator/common"
	"github.com/NetApp-Polaris/astra-connector-operator/details/k8s"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

const (
	errorRetrySleep         = time.Second * 3
	clusterUnManagedState   = "unmanaged"
	clusterManagedState     = "managed"
	getClusterPollCount     = 5
	connectorInstalled      = "installed"
	connectorInstallPending = "pending"
)

// HTTPClient interface used for request and to facilitate testing
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// HeaderMap User specific details required for the http header
type HeaderMap struct {
	AccountId     string
	Authorization string
}

// DoRequest Makes http request with the given parameters
func DoRequest(ctx context.Context, client HTTPClient, method, url string, bodyBytes []byte, headerMap HeaderMap, log logr.Logger, retryCount ...int) (*http.Response, error, context.CancelFunc) {
	// Default retry count
	retries := 1
	if len(retryCount) > 0 {
		retries = retryCount[0]
	}

	var httpResponse *http.Response
	var err error
	var cancel context.CancelFunc

	for i := 0; i < retries; i++ {

		sleepTimeout := time.Duration(math.Pow(2, float64(i))) * time.Second
		log.Info(fmt.Sprintf("Retry %d, waiting for %v before next retry\n", i, sleepTimeout))

		// Child context that can't exceed a deadline specified
		var childCtx context.Context
		childCtx, cancel = context.WithTimeout(ctx, 3*time.Minute)

		req, _ := http.NewRequestWithContext(childCtx, method, url, bytes.NewReader(bodyBytes))

		req.Header.Add("Content-Type", "application/json")

		if headerMap.Authorization != "" {
			req.Header.Add("authorization", headerMap.Authorization)
		}

		httpResponse, err = client.Do(req)
		if err == nil && httpResponse.StatusCode >= 200 && httpResponse.StatusCode < 300 {
			log.Info("Request successful")
			break
		}

		if err != nil {
			log.Info(fmt.Sprintf("Request failed with error: %v\n", err))
		} else {
			log.Info(fmt.Sprintf("Request failed with error: %v\n", httpResponse.Status))
		}

		// If the request failed or the server returned a non-2xx status code, wait before retrying
		time.Sleep(sleepTimeout)
	}

	return httpResponse, err, cancel
}

type ClusterRegisterUtil interface {
	GetConnectorIDFromConfigMap(cmData map[string]string) (string, error)
	GetNatsSyncClientRegistrationURL() string
	GetNatsSyncClientUnregisterURL() string
	RegisterNatsSyncClient() (string, string, error)
	UnRegisterNatsSyncClient() error
	GetAPITokenFromSecret(secretName string) (string, string, error)
	RegisterClusterWithAstra(astraConnectorId, clusterId string) (string, string, error)
	CloudExists(astraHost, cloudID, apiToken string) bool
	ListClouds(astraHost, apiToken string) (*http.Response, error)
	GetCloudId(astraHost, cloudType, apiToken string, retryTimeout ...time.Duration) (string, string, error)
	CreateCloud(astraHost, cloudType, apiToken string) (string, string, error)
	GetOrCreateCloud(astraHost, cloudType, apiToken string) (string, string, error)
	GetClusters(astraHost, cloudId, apiToken string) (GetClustersResponse, string, error)
	GetCluster(astraHost, cloudId, clusterId, apiToken string) (Cluster, string, error)
	CreateCluster(astraHost, cloudId, astraConnectorId, apiToken string) (ClusterInfo, string, error)
	UpdateCluster(astraHost, cloudId, clusterId, astraConnectorId, apiToken string) (string, error)
	CreateOrUpdateCluster(astraHost, cloudId, clusterId, astraConnectorId, connectorInstall, clustersMethod, apiToken string) (ClusterInfo, string, error)
	CreateManagedCluster(astraHost, cloudId, clusterID, connectorInstall, apiToken string) (string, error)
	UpdateManagedCluster(astraHost, clusterId, astraConnectorId, connectorInstall, apiToken string) (string, error)
	CreateOrUpdateManagedCluster(astraHost, cloudId, clusterId, astraConnectorId, managedClustersMethod, apiToken string) (ClusterInfo, string, error)
	ValidateAndGetCluster(astraHost, cloudId, apiToken, clusterId string) (ClusterInfo, string, error)
	UnmanageCluster(clusterID string) error
	IsClusterManaged() (bool, string, error)
	SetHttpClient(disableTls bool, astraHost string) error
}

type clusterRegisterUtil struct {
	AstraConnector *v1.AstraConnector
	Client         HTTPClient
	K8sClient      client.Client
	K8sUtil        k8s.K8sUtilInterface
	Ctx            context.Context
	Log            logr.Logger
}

func NewClusterRegisterUtil(astraConnector *v1.AstraConnector, client HTTPClient, k8sClient client.Client, k8sUtil k8s.K8sUtilInterface, log logr.Logger, ctx context.Context) ClusterRegisterUtil {
	return &clusterRegisterUtil{
		AstraConnector: astraConnector,
		Client:         client,
		K8sClient:      k8sClient,
		K8sUtil:        k8sUtil,
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

// GetNatsSyncClientRegistrationURL Returns NatsSyncClient Registration URL
func (c clusterRegisterUtil) GetNatsSyncClientRegistrationURL() string {
	natsSyncClientURL := fmt.Sprintf("http://%s.%s:%d/bridge-client/1", common.NatsSyncClientName, c.AstraConnector.Namespace, common.NatsSyncClientPort)
	natsSyncClientRegisterURL := fmt.Sprintf("%s/register", natsSyncClientURL)
	return natsSyncClientRegisterURL
}

// GetNatsSyncClientUnregisterURL returns NatsSyncClient Unregister URL
func (c clusterRegisterUtil) GetNatsSyncClientUnregisterURL() string {
	natsSyncClientURL := fmt.Sprintf("http://%s.%s:%d/bridge-client/1", common.NatsSyncClientName, c.AstraConnector.Namespace, common.NatsSyncClientPort)
	natsSyncClientRegisterURL := fmt.Sprintf("%s/unregister", natsSyncClientURL)
	return natsSyncClientRegisterURL
}

// generateAuthPayload Returns the payload for authentication
func (c clusterRegisterUtil) generateAuthPayload() ([]byte, string, error) {
	apiToken, errorReason, err := c.GetAPITokenFromSecret(c.AstraConnector.Spec.Astra.TokenRef)
	if err != nil {
		return nil, errorReason, err
	}

	authPayload, err := json.Marshal(map[string]string{
		"userToken": apiToken,
		"accountId": c.AstraConnector.Spec.Astra.AccountId,
	})

	if err != nil {
		return nil, "Failed to marshal auth payload", err
	}

	reqBodyBytes, err := json.Marshal(map[string]string{"authToken": base64.StdEncoding.EncodeToString(authPayload)})
	if err != nil {
		return nil, "Failed to marshal auth token", err
	}

	return reqBodyBytes, "", nil
}

// UnRegisterNatsSyncClient Unregisters NatsSyncClient
func (c clusterRegisterUtil) UnRegisterNatsSyncClient() error {
	natsSyncClientUnregisterURL := c.GetNatsSyncClientUnregisterURL()
	reqBodyBytes, _, err := c.generateAuthPayload()
	if err != nil {
		return err
	}

	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodPost, natsSyncClientUnregisterURL, reqBodyBytes, HeaderMap{}, c.Log)
	defer cancel()

	if err != nil {
		if response != nil {
			return errors.New(CreateErrorMsg("UnRegisterNatsSyncClient", "make POST call", natsSyncClientUnregisterURL, response.Status, "", err))
		}
		return errors.New(CreateErrorMsg("UnRegisterNatsSyncClient", "make POST call", natsSyncClientUnregisterURL, "", "", err))
	}

	if response.StatusCode != http.StatusNoContent {
		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			return errors.New(CreateErrorMsg("UnRegisterNatsSyncClient", "read response", natsSyncClientUnregisterURL, response.Status, "", err))
		}
		return errors.New(CreateErrorMsg("UnRegisterNatsSyncClient", "make POST call", natsSyncClientUnregisterURL, response.Status, string(bodyBytes), errors.New("Unexpected unregistration status")))
	}

	return nil
}

// RegisterNatsSyncClient Registers NatsSyncClient with NatsSyncServer
func (c clusterRegisterUtil) RegisterNatsSyncClient() (string, string, error) {
	natsSyncClientRegisterURL := c.GetNatsSyncClientRegistrationURL()
	reqBodyBytes, errorReason, err := c.generateAuthPayload()
	if err != nil {
		return "", errorReason, err
	}

	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodPost, natsSyncClientRegisterURL, reqBodyBytes, HeaderMap{}, c.Log, 3)
	defer cancel()
	if err != nil {
		if response != nil {
			return "", CreateErrorMsg("RegisterNatsSyncClient", "make POST call", natsSyncClientRegisterURL, response.Status, "", err), err
		}
		return "", CreateErrorMsg("RegisterNatsSyncClient", "make POST call", natsSyncClientRegisterURL, "", "", err), err
	}

	c.Log.Info(fmt.Sprintf("response %v, %v, %v", response.Body, response.Status, response.StatusCode))

	if response.StatusCode != http.StatusCreated {
		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			return "", CreateErrorMsg("RegisterNatsSyncClient", "read response from POST call", natsSyncClientRegisterURL, response.Status, "", err), err
		}
		errorMsg := CreateErrorMsg("RegisterNatsSyncClient", "make POST call", natsSyncClientRegisterURL, response.Status, string(bodyBytes), errors.New("Unexpected registration status"))
		return "", errorMsg, errors.New(errorMsg)
	}

	astraConnector := &AstraConnector{}
	err = json.NewDecoder(response.Body).Decode(astraConnector)
	if err != nil {
		return "", CreateErrorMsg("RegisterNatsSyncClient", "decode response", natsSyncClientRegisterURL, response.Status, "", err), err
	}

	return astraConnector.Id, "", nil
}

// ************************************************
//  FUNCTIONS TO REGISTER CLUSTER WITH ASTRA
// ************************************************

func GetAstraHostURL(astraConnector *v1.AstraConnector) string {
	var astraHost string
	if astraConnector.Spec.NatsSyncClient.CloudBridgeURL != "" {
		astraHost = astraConnector.Spec.NatsSyncClient.CloudBridgeURL
		astraHost = strings.TrimSuffix(astraHost, "/")
	} else {
		astraHost = common.NatsSyncClientDefaultCloudBridgeURL
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
		c.Log.Info("Received unexpected status", "responseBody", string(bodyBytes), "status", response.Status)
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

func (c clusterRegisterUtil) SetHttpClient(disableTls bool, astraHost string) error {
	if disableTls {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		c.Log.WithValues("disableTls", disableTls).Info("TLS Validation Disabled! Not for use in production!")
	}

	if c.AstraConnector.Spec.NatsSyncClient.HostAliasIP != "" {
		c.Log.WithValues("HostAliasIP", c.AstraConnector.Spec.NatsSyncClient.HostAliasIP).Info("Using the HostAlias IP")
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
				addr = c.AstraConnector.Spec.NatsSyncClient.HostAliasIP + ":443"
			}
			if addr == cloudBridgeHost+":80" {
				addr = c.AstraConnector.Spec.NatsSyncClient.HostAliasIP + ":80"
			}
			return dialer.DialContext(ctx, network, addr)
		}
	}

	c.Client = &http.Client{}
	return nil
}

func (c clusterRegisterUtil) CloudExists(astraHost, cloudID, apiToken string) bool {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s", astraHost, c.AstraConnector.Spec.Astra.AccountId, cloudID)

	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", apiToken)}
	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodGet, url, nil, headerMap, c.Log)
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
		msg := fmt.Sprintf("Get Clouds call returned with status: %s", response.Status)
		c.Log.Error(errors.New("Invalid Status"), msg)
		return false
	}

	c.Log.Info("Cloud Found: " + cloudID)
	return true
}

func (c clusterRegisterUtil) ListClouds(astraHost, apiToken string) (*http.Response, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds", astraHost, c.AstraConnector.Spec.Astra.AccountId)

	c.Log.Info("Getting clouds")
	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", apiToken)}
	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodGet, url, nil, headerMap, c.Log)
	defer cancel()

	if err != nil {
		return nil, err
	}

	return response, nil
}

func (c clusterRegisterUtil) GetCloudId(astraHost, cloudType, apiToken string, retryTimeout ...time.Duration) (string, string, error) {
	// TODO: This function assumes that only ONE cloud instance of a given cloud type would be present in the persistence.
	// TODO: If we ever choose to support multiple cloud instances of type "private" this function wouldn't support that and an enhancement would be needed.

	success := false
	var response *http.Response
	timeout := time.Second * 30
	if len(retryTimeout) > 0 {
		timeout = retryTimeout[0]
	}
	timeExpire := time.Now().Add(timeout)

	for time.Now().Before(timeExpire) {
		var err error
		response, err = c.ListClouds(astraHost, apiToken)
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
		return "", "Failed to Get Clouds", fmt.Errorf("timed out querying Astra API")
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
		return "", "Failed to read response from Get Clouds", err
	}
	resp := respData{}
	err = json.Unmarshal(bodyBytes, &resp)
	if err != nil {
		return "", "Failed to unmarshal response from Get Clouds", err
	}

	var cloudId string
	for _, cloudInfo := range resp.Items {
		if cloudInfo.CloudType == cloudType {
			cloudId = cloudInfo.Id
			break
		}
	}

	return cloudId, "", nil
}

func (c clusterRegisterUtil) CreateCloud(astraHost, cloudType, apiToken string) (string, string, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds", astraHost, c.AstraConnector.Spec.Astra.AccountId)
	payLoad := map[string]string{
		"type":      "application/astra-cloud",
		"version":   "1.0",
		"name":      common.AstraPrivateCloudName,
		"cloudType": cloudType,
	}

	reqBodyBytes, err := json.Marshal(payLoad)
	if err != nil {
		return "", fmt.Sprintf("Failed to marshal request body payload for POST %v", url), err
	}

	c.Log.WithValues("cloudType", cloudType).Info("Creating cloud")
	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", apiToken)}
	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodPost, url, reqBodyBytes, headerMap, c.Log)
	defer cancel()

	if err != nil {
		if response != nil {
			return "", CreateErrorMsg("CreateCloud", "make POST call", url, response.Status, "", err), err
		}
		return "", CreateErrorMsg("CreateCloud", "make POST call", url, "", "", err), err
	}

	type CloudResp struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	respBody, err := c.readResponseBody(response)
	if err != nil {
		return "", CreateErrorMsg("CreateCloud", "read response from POST call", url, response.Status, "", err), err
	}

	cloudResp := &CloudResp{}
	err = json.Unmarshal(respBody, &cloudResp)
	if err != nil {
		return "", CreateErrorMsg("CreateCloud", "unmarshal response from POST call", url, response.Status, string(respBody), err), err
	}

	if cloudResp.ID == "" {
		c.Log.WithValues("response", string(respBody)).Error(errors.New("got empty cloud id"), "invalid response")
	}

	return cloudResp.ID, "", nil
}

func (c clusterRegisterUtil) GetOrCreateCloud(astraHost, cloudType, apiToken string) (string, string, error) {
	// If a cloudId is specified in the CR Spec, validate its existence.
	// If the provided cloudId is valid, return the same.
	// If it is not a valid cloudId i.e., provided cloudId doesn't exist in the DB, return an error
	cloudId := c.AstraConnector.Spec.Astra.CloudId
	if cloudId != "" {
		c.Log.WithValues("cloudID", cloudId).Info("Validating the provided CloudId")
		if !c.CloudExists(astraHost, cloudId, apiToken) {
			errMsg := fmt.Sprintf("Invalid CloudId %v provided in the Spec", cloudId)
			return "", errMsg, errors.New(errMsg)
		}

		c.Log.WithValues("cloudID", cloudId).Info("CloudId exists in the system")
		return cloudId, "", nil
	}

	// When a cloudId is not specified in the CR Spec, check if a cloud of type "private"
	// exists in the system. If it exists, return the CloudId of the "private" cloud.
	// Otherwise, proceed to create a cloud of type "private" and the return the CloudId
	// of the newly created cloud.
	c.Log.WithValues("cloudType", cloudType).Info("Fetching Cloud Id")

	cloudId, errorReason, err := c.GetCloudId(astraHost, cloudType, apiToken)
	if err != nil {
		c.Log.Error(err, "Error fetching cloud ID")
		return "", errorReason, err
	}

	if cloudId == "" {
		c.Log.Info("Cloud doesn't seem to exist, creating the cloud", "cloudType", cloudType)
		cloudId, errorReason, err = c.CreateCloud(astraHost, cloudType, apiToken)
		if err != nil {
			c.Log.Error(err, "Failed to create cloud", "cloudType", cloudType)
			return "", errorReason, err
		}
		if cloudId == "" {
			return "", "Got empty Cloud Id from POST call to clouds", fmt.Errorf("could not create cloud of type %s", cloudType)
		}
	}

	c.Log.WithValues("cloudID", cloudId).Info("Found/Created Cloud")

	return cloudId, "", nil
}

func (c clusterRegisterUtil) IsClusterManaged() (bool, string, error) {
	apiToken, errorReason, err := c.GetAPITokenFromSecret(c.AstraConnector.Spec.Astra.TokenRef)
	if err != nil {
		return false, errorReason, err
	}

	astraHost := GetAstraHostURL(c.AstraConnector)
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/managedClusters/%s", astraHost, c.AstraConnector.Spec.Astra.AccountId, c.AstraConnector.Spec.Astra.ClusterId)

	c.Log.WithValues("ClusterId", c.AstraConnector.Spec.Astra.ClusterId).
		Info("Checking if cluster is managed")

	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", apiToken)}
	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodGet, url, nil, headerMap, c.Log)
	defer cancel()
	if err != nil {
		return false, CreateErrorMsg("IsClusterManaged", "GET /managedCluster error", url, "", "", err), err
	}
	if response.StatusCode != 200 {
		return false, CreateErrorMsg("IsClusterManaged", "GET /managedCluster non 200 response", url, response.Status, "", err), err
	}

	respBody, err := c.readResponseBody(response)
	if err != nil {
		return false, CreateErrorMsg("IsClusterManaged", "parse GET /managedCluster response", url, response.Status, "", err), err
	}

	type ManagedClusterResp struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		ManagedState string `json:"managedState"`
	}

	clusterResp := &ManagedClusterResp{}
	err = json.Unmarshal(respBody, &clusterResp)
	if err != nil {
		return false, CreateErrorMsg("IsClusterManaged", "unmarshal response from GET call", url, response.Status, string(respBody), err), err
	}

	if clusterResp == nil || clusterResp.ManagedState != "managed" {
		return false, "", nil
	}

	return true, "", nil

}

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
	ConnectorInstall           string   `json:"connectorInstall,omitempty"`
	TridentManagedStateDesired string   `json:"tridentManagedStateDesired,omitempty"`
	ApiServiceID               string   `json:"apiServiceID,omitempty"`
}

type GetClustersResponse struct {
	Items []Cluster `json:"items"`
}

type ClusterInfo struct {
	ID               string
	Name             string
	ManagedState     string
	ConnectorInstall string
}

// GetClusters Returns a list of existing clusters
func (c clusterRegisterUtil) GetClusters(astraHost, cloudId, apiToken string) (GetClustersResponse, string, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s/clusters", astraHost, c.AstraConnector.Spec.Astra.AccountId, cloudId)
	var clustersRespJson GetClustersResponse

	c.Log.Info("Getting Clusters")

	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", apiToken)}
	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodGet, url, nil, headerMap, c.Log)
	defer cancel()

	if err != nil {
		if response != nil {
			return clustersRespJson, CreateErrorMsg("GetClusters", "make GET call", url, response.Status, "", err), err
		}
		return clustersRespJson, CreateErrorMsg("GetClusters", "make GET call", url, "", "", err), err
	}

	if response.StatusCode != http.StatusOK {
		errorMsg := CreateErrorMsg("GetClusters", "make GET call", url, response.Status, "", err)
		return clustersRespJson, errorMsg, errors.New(errorMsg)
	}

	respBody, err := io.ReadAll(response.Body)
	if err != nil {
		return clustersRespJson, CreateErrorMsg("GetClusters", "read response from GET call", url, response.Status, string(respBody), err), err
	}

	err = json.Unmarshal(respBody, &clustersRespJson)
	if err != nil {
		return clustersRespJson, CreateErrorMsg("GetClusters", "unmarshal response from GET call", url, response.Status, string(respBody), err), err
	}

	return clustersRespJson, "", nil
}

// pollForClusterToBeInDesiredState Polls until a given cluster is in desired state (or until timeout)
func (c clusterRegisterUtil) pollForClusterToBeInDesiredState(astraHost, cloudId, clusterId, desiredState, apiToken string) error {
	for i := 1; i <= getClusterPollCount; i++ {
		time.Sleep(15 * time.Second)
		getCluster, errorMsg, getClusterErr := c.GetCluster(astraHost, cloudId, clusterId, apiToken)

		if getClusterErr != nil {
			return errors.New(errorMsg)
		}

		if getCluster.ManagedState == desiredState {
			return nil
		}
	}
	return errors.New(CreateErrorMsg("pollForClusterToBeInDesiredState", "check cluster state", astraHost, "", "", errors.New("cluster state not changed to desired state: "+clusterId)))
}

// GetCluster Returns the details of the given clusterID (if it exists)
func (c clusterRegisterUtil) GetCluster(astraHost, cloudId, clusterId, apiToken string) (Cluster, string, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s/clusters/%s", astraHost, c.AstraConnector.Spec.Astra.AccountId, cloudId, clusterId)
	var clustersRespJson Cluster

	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", apiToken)}
	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodGet, url, nil, headerMap, c.Log)
	defer cancel()

	if err != nil {
		if response != nil {
			return Cluster{}, CreateErrorMsg("GetCluster", "make GET call", url, response.Status, "", err), err
		}
		return Cluster{}, CreateErrorMsg("GetCluster", "make GET call", url, "", "", err), err
	}

	if response.StatusCode != http.StatusOK {
		errorMsg := CreateErrorMsg("GetCluster", "make GET call", url, response.Status, "", nil)
		return clustersRespJson, errorMsg, errors.New(errorMsg)
	}

	respBody, err := io.ReadAll(response.Body)
	if err != nil {
		return clustersRespJson, CreateErrorMsg("GetCluster", "read response from GET call", url, response.Status, string(respBody), err), err
	}

	err = json.Unmarshal(respBody, &clustersRespJson)
	if err != nil {
		return Cluster{}, CreateErrorMsg("GetCluster", "unmarshal response from GET call", url, response.Status, string(respBody), err), err
	}

	return clustersRespJson, "", nil
}

// CreateCluster Creates a cluster with the provided details
func (c clusterRegisterUtil) CreateCluster(astraHost, cloudId, astraConnectorId, apiToken string) (ClusterInfo, string, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s/clusters", astraHost, c.AstraConnector.Spec.Astra.AccountId, cloudId)
	var clustersRespJson Cluster

	clusterTypeChecker := k8s.ClusterTypeChecker{K8sUtil: c.K8sUtil, Log: c.Log}
	clusterType := clusterTypeChecker.DetermineClusterType()

	clustersBody := Cluster{
		Type:                  "application/astra-cluster",
		Version:               common.AstraClustersAPIVersion,
		Name:                  c.AstraConnector.Spec.Astra.ClusterName,
		ConnectorCapabilities: common.GetConnectorCapabilities(),
		PrivateRouteID:        astraConnectorId,
		ConnectorInstall:      connectorInstallPending,
		ClusterType:           clusterType,
	}

	clustersBodyJson, _ := json.Marshal(clustersBody)
	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", apiToken)}
	clustersResp, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodPost, url, clustersBodyJson, headerMap, c.Log, 3)
	defer cancel()

	if err != nil {
		errorMsg := ""
		if clustersResp != nil {
			errorMsg = CreateErrorMsg("CreateCluster", "make POST call", url, clustersResp.Status, "", err)
		} else {
			errorMsg = CreateErrorMsg("CreateCluster", "make POST call", url, "", "", err)
		}

		return ClusterInfo{}, errorMsg, fmt.Errorf("%s: %w", errorMsg, err)
	}

	respBody, err := io.ReadAll(clustersResp.Body)
	if err != nil {
		errorMsg := CreateErrorMsg("CreateCluster", "read response from POST call", url, clustersResp.Status, "", err)
		return ClusterInfo{}, errorMsg, fmt.Errorf("%s", errorMsg)
	}

	if clustersResp.StatusCode != http.StatusCreated {
		errorMsg := CreateErrorMsg("CreateCluster", "make POST call", url, clustersResp.Status, string(respBody), nil)
		return ClusterInfo{}, errorMsg, fmt.Errorf("%s", errorMsg)
	}

	err = json.Unmarshal(respBody, &clustersRespJson)
	if err != nil {
		errorMsg := CreateErrorMsg("CreateCluster", "unmarshal response from POST call", url, clustersResp.Status, "", err)
		return ClusterInfo{}, errorMsg, fmt.Errorf("%s: %w", errorMsg, err)
	}

	if clustersRespJson.ID == "" {
		errorMsg := CreateErrorMsg("CreateCluster", "get clusterId in response from POST call", url, clustersResp.Status, string(respBody), nil)
		return ClusterInfo{}, errorMsg, fmt.Errorf("%s", errorMsg)
	}

	if clustersRespJson.ManagedState == clusterUnManagedState {
		c.Log.Info("Cluster added to Astra", "clusterId", clustersRespJson.ID)
		return ClusterInfo{ID: clustersRespJson.ID, ManagedState: clustersRespJson.ManagedState, Name: clustersRespJson.Name}, "", nil
	}

	err = c.pollForClusterToBeInDesiredState(astraHost, cloudId, clustersRespJson.ID, clusterUnManagedState, apiToken)
	if err == nil {
		c.Log.Info("Cluster added to Astra", "clusterId", clustersRespJson.ID)
		return ClusterInfo{ID: clustersRespJson.ID, ManagedState: clustersRespJson.ManagedState, Name: clustersRespJson.Name}, "", nil
	}

	return ClusterInfo{}, "Cluster State not changed to desired state", errors.New("cluster state not changed to desired state")
}

// UpdateCluster Updates an existing cluster with the provided details
func (c clusterRegisterUtil) UpdateCluster(astraHost, cloudId, clusterId, astraConnectorId, apiToken string) (string, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s/clusters/%s", astraHost, c.AstraConnector.Spec.Astra.AccountId, cloudId, clusterId)

	clusterTypeChecker := &k8s.ClusterTypeChecker{K8sUtil: c.K8sUtil, Log: c.Log}
	clusterType := clusterTypeChecker.DetermineClusterType()

	clustersBody := Cluster{
		Type:                  "application/astra-cluster",
		Version:               common.AstraClustersAPIVersion,
		Name:                  c.AstraConnector.Spec.Astra.ClusterName,
		ConnectorCapabilities: common.GetConnectorCapabilities(),
		PrivateRouteID:        astraConnectorId,
		ClusterType:           clusterType,
	}

	clustersBodyJson, _ := json.Marshal(clustersBody)
	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", apiToken)}
	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodPut, url, clustersBodyJson, headerMap, c.Log, 3)
	defer cancel()

	if err != nil {
		if response != nil {
			return CreateErrorMsg("UpdateCluster", "make PUT call", url, response.Status, "", err), err
		}
		return CreateErrorMsg("UpdateCluster", "make PUT call", url, "", "", err), err
	}

	if response.StatusCode > http.StatusNoContent {
		errorMsg := CreateErrorMsg("UpdateCluster", "make PUT call", url, response.Status, "", nil)
		return errorMsg, errors.New(errorMsg)
	}

	c.Log.WithValues("clusterId", clusterId).Info("Cluster updated")
	return "", nil
}

func (c clusterRegisterUtil) CreateOrUpdateCluster(astraHost, cloudId, clusterId, astraConnectorId, connectorInstall, clustersMethod, apiToken string) (ClusterInfo, string, error) {
	if clustersMethod == http.MethodPut {
		c.Log.WithValues("clusterId", clusterId).Info("Updating cluster")

		errorReason, err := c.UpdateCluster(astraHost, cloudId, clusterId, astraConnectorId, apiToken)
		if err != nil {
			return ClusterInfo{}, errorReason, errors.Wrap(err, "error updating cluster")
		}

		return ClusterInfo{ID: clusterId, ConnectorInstall: connectorInstall}, "", nil
	}

	if clustersMethod == http.MethodPost {
		c.Log.Info("Creating Cluster")

		clusterInfo, errorReason, err := c.CreateCluster(astraHost, cloudId, astraConnectorId, apiToken)
		if err != nil {
			return ClusterInfo{}, errorReason, errors.Wrap(err, "error creating cluster")
		}

		return clusterInfo, "", nil
	}

	c.Log.Info("Create/Update cluster not required!")
	return ClusterInfo{ID: clusterId}, "", nil
}

// UpdateManagedCluster Updates the persisted record of the given managed cluster
func (c clusterRegisterUtil) UpdateManagedCluster(astraHost, clusterId, astraConnectorId, connectorInstall, apiToken string) (string, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/managedClusters/%s", astraHost, c.AstraConnector.Spec.Astra.AccountId, clusterId)

	manageClustersBody := Cluster{
		Type:                  "application/astra-managedCluster",
		Version:               common.AstraManagedClustersAPIVersion,
		ConnectorCapabilities: common.GetConnectorCapabilities(),
		PrivateRouteID:        astraConnectorId,
		ConnectorInstall:      connectorInstall,
	}
	manageClustersBodyJson, _ := json.Marshal(manageClustersBody)

	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", apiToken)}
	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodPut, url, manageClustersBodyJson, headerMap, c.Log, 3)
	defer cancel()

	if err != nil {
		if response != nil {
			return CreateErrorMsg("UpdateManagedCluster", "make PUT call", url, response.Status, "", err), err
		}
		return CreateErrorMsg("UpdateManagedCluster", "make PUT call", url, "", "", err), err
	}

	if response.StatusCode > http.StatusNoContent {
		errorMsg := CreateErrorMsg("UpdateManagedCluster", "make PUT call", url, response.Status, "", errors.New("update managed cluster failed with: "+strconv.Itoa(response.StatusCode)))
		return errorMsg, errors.New(errorMsg)
	}

	c.Log.WithValues("clusterId", clusterId).Info("Managed Cluster updated")
	return "", nil
}

// CreateManagedCluster Transitions a cluster from unmanaged state to managed state
func (c clusterRegisterUtil) CreateManagedCluster(astraHost, cloudId, clusterID, connectorInstall, apiToken string) (string, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/managedClusters", astraHost, c.AstraConnector.Spec.Astra.AccountId)
	var manageClustersRespJson Cluster

	manageClustersBody := Cluster{
		Type:                       "application/astra-managedCluster",
		Version:                    common.AstraManagedClustersAPIVersion,
		ID:                         clusterID,
		TridentManagedStateDesired: clusterManagedState,
		ConnectorInstall:           connectorInstall,
	}
	manageClustersBodyJson, _ := json.Marshal(manageClustersBody)

	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", apiToken)}
	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodPost, url, manageClustersBodyJson, headerMap, c.Log, 3)
	defer cancel()

	if err != nil {
		if response != nil {
			return CreateErrorMsg("CreateManagedCluster", "make POST call", url, response.Status, "", err), err
		}
		return CreateErrorMsg("CreateManagedCluster", "make POST call", url, "", "", err), err
	}

	if response.StatusCode != http.StatusCreated {
		errorMsg := CreateErrorMsg("CreateManagedCluster", "make POST call", url, response.Status, "", errors.New("manage cluster failed with: "+strconv.Itoa(response.StatusCode)))
		return errorMsg, errors.New(errorMsg)
	}

	respBody, err := io.ReadAll(response.Body)
	if err != nil {
		return CreateErrorMsg("CreateManagedCluster", "read response from POST call", url, response.Status, "", err), err
	}

	err = json.Unmarshal(respBody, &manageClustersRespJson)
	if err != nil {
		return CreateErrorMsg("CreateManagedCluster", "unmarshal response from POST call", url, response.Status, string(respBody), err), err
	}

	err = c.pollForClusterToBeInDesiredState(astraHost, cloudId, clusterID, clusterManagedState, apiToken)
	if err == nil {
		return "", nil
	}

	return "Cluster State not changed to managed", errors.New(CreateErrorMsg("CreateManagedCluster", "check cluster state", astraHost, "", "", errors.New("cluster state not changed to managed")))
}

func (c clusterRegisterUtil) CreateOrUpdateManagedCluster(astraHost, cloudId, clusterId, astraConnectorId, managedClustersMethod, apiToken string) (ClusterInfo, string, error) {
	if managedClustersMethod == http.MethodPut {
		c.Log.Info("Updating Managed Cluster")

		errorReason, err := c.UpdateManagedCluster(astraHost, clusterId, astraConnectorId, connectorInstalled, apiToken)
		if err != nil {
			return ClusterInfo{ID: clusterId}, errorReason, errors.Wrap(err, "error updating managed cluster")
		}

		return ClusterInfo{ID: clusterId, ManagedState: clusterManagedState}, "", nil
	}

	if managedClustersMethod == http.MethodPost {
		c.Log.Info("Creating Managed Cluster")

		// Note: we no longer set storageClass for arch3.0 clusters
		errorReason, err := c.CreateManagedCluster(astraHost, cloudId, clusterId, connectorInstalled, apiToken)
		if err != nil {
			return ClusterInfo{ID: clusterId}, errorReason, errors.Wrap(err, "error creating managed cluster")
		}

		return ClusterInfo{ID: clusterId, ManagedState: clusterManagedState}, "", nil
	}

	c.Log.Info("Create/Update managed cluster not required!")
	return ClusterInfo{ID: clusterId}, "", nil
}

func (c clusterRegisterUtil) ValidateAndGetCluster(astraHost, cloudId, apiToken, clusterId string) (ClusterInfo, string, error) {
	// If a clusterId is known (from CR Spec or CR Status), validate its existence.
	// If the provided clusterId exists in the DB, return the details of that cluster, otherwise return an error

	if clusterId != "" {
		c.Log.WithValues("cloudID", cloudId, "clusterID", clusterId).Info("Validating the provided ClusterId")
		getClusterResp, errorReason, err := c.GetCluster(astraHost, cloudId, clusterId, apiToken)
		if err != nil {
			return ClusterInfo{}, errorReason, errors.Wrap(err, "error on get cluster")
		}

		if getClusterResp.ID == "" {
			errMsg := fmt.Sprintf("Invalid ClusterId %v provided in the Spec", clusterId)
			return ClusterInfo{}, errMsg, errors.New(errMsg)
		}

		c.Log.WithValues("cloudID", cloudId, "clusterID", clusterId).Info("ClusterId exists in the system")
		return ClusterInfo{ID: clusterId, Name: getClusterResp.Name, ManagedState: getClusterResp.ManagedState, ConnectorInstall: getClusterResp.ConnectorInstall}, "", nil
	}

	// Check whether a cluster exists with a matching "apiServiceID"
	// Get all clusters and validate whether any of the response matches with the current cluster's "ServiceUUID"
	k8sService := &coreV1.Service{}
	err := c.K8sClient.Get(c.Ctx, types.NamespacedName{Name: "kubernetes", Namespace: "default"}, k8sService)
	if err != nil {
		errMsg := "Failed to get kubernetes service from default namespace"
		c.Log.Error(err, errMsg)
		return ClusterInfo{}, errMsg, err
	}
	k8sServiceUUID := string(k8sService.ObjectMeta.UID)
	c.Log.Info(fmt.Sprintf("Kubernetes service UUID is %s", k8sServiceUUID))

	// Check whether a cluster exists with the above "k8sServiceUUID" as "apiServiceID"
	getClustersResp, errorReason, err := c.GetClusters(astraHost, cloudId, apiToken)
	if err != nil {
		return ClusterInfo{}, errorReason, errors.Wrap(err, "error on get clusters")
	}

	c.Log.WithValues("cloudID", cloudId).Info("Checking existing records for current cluster's record")
	for _, value := range getClustersResp.Items {
		// We want to allow dual management of a cluster across different architectures to support migration from v2 to v3. Only reuse existing clusterID if v3 i.e. ConnectorInstall is true
		if value.ApiServiceID == k8sServiceUUID && value.ConnectorInstall == "installed" {
			c.Log.WithValues("ClusterId", value.ID, "Name", value.Name, "ManagedState", value.ManagedState).Info("Cluster Info found in the existing records")
			return ClusterInfo{ID: value.ID, Name: value.Name, ManagedState: value.ManagedState}, "", nil
		}
	}

	// This is the case for creation of cluster with POST calls to /clusters and /managedClusters
	c.Log.WithValues("cloudID", cloudId).Info("ClusterId not specified in CR Spec and an existing cluster doesn't exist in the system")
	return ClusterInfo{}, "", nil
}

// UnmanageCluster unmanages and removes a cluster from Astra.
// It accomplishes this by sending two DELETE requests to the Astra platform cluster:
// one to unmanage the cluster and another to remove the cluster record.
func (c clusterRegisterUtil) UnmanageCluster(clusterID string) error {
	astraHost := GetAstraHostURL(c.AstraConnector)
	c.Log.WithValues("URL", astraHost).Info("Astra Host Info")

	apiToken, _, err := c.GetAPITokenFromSecret(c.AstraConnector.Spec.Astra.TokenRef)
	if err != nil {
		c.Log.Error(err, "Failed to get API token")
		return err
	}

	cloudId, _, err := c.GetCloudId(astraHost, common.AstraPrivateCloudType, apiToken)
	if err != nil {
		c.Log.Error(err, "Failed to get Cloud ID")
		return err
	}

	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", apiToken)}

	// Un-managing cluster
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/managedClusters/%s", astraHost, c.AstraConnector.Spec.Astra.AccountId, clusterID)
	resp, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodDelete, url, nil, headerMap, c.Log)
	if err != nil {
		c.Log.Error(err, "Failed to unmanage cluster")
		return errors.New(CreateErrorMsg("UnmanageCluster", "make DELETE call", url, "", "", err))
	}
	if resp != nil {
		defer cancel()
		if resp.StatusCode != http.StatusNoContent {
			c.Log.Error(err, "Failed to unmanage cluster, received non-OK response")
			return errors.New(CreateErrorMsg("UnmanageCluster", "make DELETE call", url, resp.Status, "", err))
		}
	}

	// Removing cluster
	url = fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s/clusters/%s", astraHost, c.AstraConnector.Spec.Astra.AccountId, cloudId, clusterID)
	resp, err, cancel = DoRequest(c.Ctx, c.Client, http.MethodDelete, url, nil, headerMap, c.Log)
	defer cancel()

	if err != nil {
		c.Log.Error(err, "Failed to remove cluster")
		return errors.New(CreateErrorMsg("UnmanageCluster", "make DELETE call", url, "", "", err))
	}

	if resp != nil {
		defer cancel()
		if resp.StatusCode != http.StatusNoContent {
			c.Log.Error(err, "Failed to remove cluster, received non-OK response")
			return errors.New(CreateErrorMsg("UnmanageCluster", "make DELETE call", url, resp.Status, "", err))
		}
	}

	return nil
}

// GetAPITokenFromSecret Gets Secret provided in the ACC Spec and returns api token string of the data in secret
func (c clusterRegisterUtil) GetAPITokenFromSecret(secretName string) (string, string, error) {
	secret := &coreV1.Secret{}

	err := c.K8sClient.Get(c.Ctx, types.NamespacedName{Name: secretName, Namespace: c.AstraConnector.Namespace}, secret)
	if err != nil {
		c.Log.WithValues("namespace", c.AstraConnector.Namespace, "secret", secretName).Error(err, "failed to get kubernetes secret")
		return "", fmt.Sprintf("Failed to get secret %s", secretName), err
	}

	// Extract the value of the 'apiToken' key from the secret
	apiToken, ok := secret.Data["apiToken"]
	if !ok {
		c.Log.WithValues("namespace", c.AstraConnector.Namespace, "secret", secretName).Error(err, "failed to extract apiToken key from secret")
		return "", fmt.Sprintf("Failed to extract 'apiToken' key from secret %s", secretName), errors.New("failed to extract apiToken key from secret")
	}

	// Convert the value to a string
	apiTokenStr := string(apiToken)
	return apiTokenStr, "", nil
}

// RegisterClusterWithAstra Registers/Adds the cluster to Astra
func (c clusterRegisterUtil) RegisterClusterWithAstra(astraConnectorId string, clusterId string) (string, string, error) {
	astraHost := GetAstraHostURL(c.AstraConnector)
	c.Log.WithValues("URL", astraHost).Info("Astra Host Info")

	err := c.SetHttpClient(c.AstraConnector.Spec.Astra.SkipTLSValidation, astraHost)
	if err != nil {
		return "", "Failed to set TLS Config", err
	}

	// Extract the apiToken from the secret provided in the CR Spec via "tokenRef" field
	// This is needed to make calls to the Astra
	apiToken, errorReason, err := c.GetAPITokenFromSecret(c.AstraConnector.Spec.Astra.TokenRef)
	if err != nil {
		return "", errorReason, err
	}

	// 1. Checks the existence of cloud in the system with the cloudId (if it was specified in the CR Spec)
	//    If the CloudId was specified and the cloud exists in the system, the same cloudId is returned.
	//    If the CloudId was specified and the cloud doesn't exist in the system, an error is returned.
	// 2. If the CloudId was not specified in the CR Spec, checks whether a cloud of type "private"
	//    exists in the system, if so returns the cloudId of the "private" cloud. Otherwise, a new cloud of
	//    type "private" is created and the cloudId is returned.
	cloudId, errorReason, err := c.GetOrCreateCloud(astraHost, common.AstraPrivateCloudType, apiToken)
	if err != nil {
		return "", errorReason, err
	}

	// 1. Checks the existence of cluster in the system with the clusterId (if it was specified in the CR Spec)
	//    If the ClusterId was specified and the cluster exists in the system, details related to that cluster are returned.
	//    If the ClusterId was specified and the cluster doesn't exist in the system, an error is returned.
	// 2. If the ClusterId was not specified in the CR Spec, checks the existence of a cluster in the system (happens on reinstall)
	//    with "K8s Service UUID" of the current cluster as "ApiServiceID" field value. If there exists such a record,
	//    details related to that cluster will be returned. Otherwise, empty cluster details will be returned
	clusterInfo, errorReason, err := c.ValidateAndGetCluster(astraHost, cloudId, apiToken, clusterId)
	if err != nil {
		return "", errorReason, err
	}

	var clustersMethod, managedClustersMethod string
	if clusterInfo.ID != "" {
		// clusterInfo.ID != "" ====>
		// 1. ClusterId specified in the CR Status or CR Spec AND it is present in the system
		// 							OR
		// 2. A cluster record with matching "apiServiceID" is present in the system (happens on re-install)
		c.Log.WithValues(
			"cloudID", cloudId,
			"clusterID", clusterInfo.ID,
			"clusterManagedState", clusterInfo.ManagedState,
			"connectorInstall", clusterInfo.ConnectorInstall,
		).Info("Cluster exists in the system, updating the existing cluster")

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
	clusterInfo, errorReason, err = c.CreateOrUpdateCluster(astraHost, cloudId, clusterInfo.ID, astraConnectorId, clusterInfo.ConnectorInstall, clustersMethod, apiToken)
	if err != nil {
		return "", errorReason, err
	}

	// Adding or Updating Managed Cluster based on the status from above
	clusterInfo, errorReason, err = c.CreateOrUpdateManagedCluster(astraHost, cloudId, clusterInfo.ID, astraConnectorId, managedClustersMethod, apiToken)
	if err != nil {
		return clusterInfo.ID, errorReason, err
	}

	c.Log.WithValues("clusterId", clusterInfo.ID, "clusterName", clusterInfo.Name).Info("Cluster managed by Astra!!!!")
	return clusterInfo.ID, "", nil
}

// CreateErrorMsg creates a standardized error message for HTTP requests.
// This should be used in all cases that we want to format an error message for CR status updates.
func CreateErrorMsg(functionName, action, url, status, responseBody string, err error) string {
	errMessage := ""
	if err != nil {
		errMessage = fmt.Sprintf(": %s", err.Error())
	}

	respBodyMessage := ""
	if responseBody != "" {
		respBodyMessage = fmt.Sprintf("; Response Body: %s", responseBody)
	}

	errorMsg := fmt.Sprintf("%s: Failed to %s to %v with status %s%s%s", functionName, action, url, status, errMessage, respBodyMessage)

	return errorMsg
}
