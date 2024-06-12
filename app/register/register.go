/*
 * Copyright (c) 2024. NetApp, Inc. All Rights Reserved.
 */

package register

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
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

type ConnectorInstallState string
type ClusterState string
type ClusterHeartbeatState string
type ClusterManagedState string
type CloudType string

const (
	ConnectorInstallStatePending   ConnectorInstallState = "pending"
	ConnectorInstallStateInstalled ConnectorInstallState = "installed"

	ClusterStatePending ClusterState = "pending"

	ClusterManagedStateManaged ClusterManagedState = "managed"

	ApiCloudType    = "application/astra-cloud"
	ApiCloudVersion = "1.1"

	CloudTypePrivate CloudType = "private"
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
	GetAPITokenFromSecret(secretName string) (string, string, error)
	IsClusterManaged(clusterId string) (bool, string, error)
	SetHttpClient(disableTls bool, astraHost string) error
	RegisterCloud() (string, string, error)
	RegisterCluster(cloudId string, k8sServiceId string) (string, string, error)
}

type clusterRegisterUtil struct {
	AstraConnector *v1.AstraConnector
	Client         HTTPClient
	K8sClient      client.Client
	K8sUtil        k8s.K8sUtilInterface
	Ctx            context.Context
	Log            logr.Logger
	ApiToken       string
	AstraHostUrl   string
}

func NewClusterRegisterUtil(astraConnector *v1.AstraConnector, client HTTPClient, k8sClient client.Client, k8sUtil k8s.K8sUtilInterface, log logr.Logger, ctx context.Context) (ClusterRegisterUtil, error) {
	c := clusterRegisterUtil{
		AstraConnector: astraConnector,
		Client:         client,
		K8sClient:      k8sClient,
		K8sUtil:        k8sUtil,
		Log:            log,
		Ctx:            ctx,
	}
	var err error
	c.ApiToken, _, err = c.GetAPITokenFromSecret(c.AstraConnector.Spec.Astra.TokenRef)
	if err != nil {
		return nil, err
	}
	c.AstraHostUrl = GetAstraHostURL(astraConnector)
	err = c.SetHttpClient(astraConnector.Spec.Astra.SkipTLSValidation, c.AstraHostUrl)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ******************************
//  FUNCTIONS TO REGISTER NATS
// ******************************

type AstraConnector struct {
	Id string `json:"locationID"`
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

func (c clusterRegisterUtil) IsClusterManaged(clusterId string) (bool, string, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/managedClusters/%s", c.AstraHostUrl, c.AstraConnector.Spec.Astra.AccountId, clusterId)

	c.Log.WithValues("ClusterId", c.AstraConnector.Spec.Astra.ClusterId).
		Info("Checking if cluster is managed")

	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", c.ApiToken)}
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
	Type                       string                `json:"type,omitempty"`
	Version                    string                `json:"version,omitempty"`
	ID                         string                `json:"id,omitempty"`
	Name                       string                `json:"name,omitempty"`
	State                      ClusterState          `json:"state,omitempty"`
	ManagedState               ClusterManagedState   `json:"managedState,omitempty"`
	ClusterType                string                `json:"clusterType,omitempty"`
	CloudID                    string                `json:"cloudID,omitempty"`
	PrivateRouteID             string                `json:"privateRouteID,omitempty"`
	ConnectorCapabilities      []string              `json:"connectorCapabilities,omitempty"`
	ConnectorInstall           ConnectorInstallState `json:"connectorInstall,omitempty"`
	TridentManagedStateDesired string                `json:"tridentManagedStateDesired,omitempty"`
	ApiServiceID               string                `json:"apiServiceID,omitempty"`
	ClusterVersion             string                `json:"clusterVersion,omitempty"`
	ClusterVersionString       string                `json:"clusterVersionString,omitempty"`
	LastHeartbeat              string                `json:"lastHeartbeat,omitempty"`
	HeartbeatState             string                `json:"heartbeatState,omitempty"`
	ConnectorNamespace         string                `json:"connectorNamespace,omitempty"`
	AstraControlURL            string                `json:"astraControlURL,omitempty"`
	HostAliasIP                string                `json:"hostAliasIP,omitempty"`
	APITokenRef                string                `json:"apiTokenRef,omitempty"`
}

type ClusterList struct {
	Items []Cluster `json:"items"`
}

type ClusterInfo struct {
	ID               string
	Name             string
	ManagedState     string
	ConnectorInstall string
}

type Cloud struct {
	ID        string    `json:"id,omitempty"`
	Type      string    `json:"type"`
	Version   string    `json:"version"`
	Name      string    `json:"name"`
	CloudType CloudType `json:"cloudType"`
}

type CloudList struct {
	Items []Cloud `json:"items"`
}

func (c clusterRegisterUtil) CreateManagedCluster(managedCluster *Cluster) (string, error) {
	if managedCluster == nil {
		errMsg := "the given managedCluster is nil"
		return errMsg, errors.New(errMsg)
	}
	if managedCluster.ID == "" {
		errMsg := "error creating managed cluster, no clusterId provided"
		return errMsg, errors.New(errMsg)
	}

	managedCluster.Type = "application/astra-managedCluster"
	managedCluster.Version = "1.2"

	body, err := json.Marshal(managedCluster)
	if err != nil {
		errMsg := fmt.Sprintf("failed to marshal ManagedCluster: %s", err)
		return errMsg, errors.New(errMsg)
	}

	url := fmt.Sprintf("%s/accounts/%s/topology/v1/managedClusters", c.AstraHostUrl, c.AstraConnector.Spec.Astra.AccountId)

	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", c.ApiToken)}
	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodPost, url, body, headerMap, c.Log)
	defer cancel()
	if err != nil {
		return CreateErrorMsg("CreateManagedCluster", "POST /managedClusters error", url, "", "", err), err
	}
	if response.StatusCode != http.StatusCreated {
		return CreateErrorMsg("CreateManagedCluster", "POST /managedClusters non 201 response", url, response.Status, "", err), err
	}
	return "", nil
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

func (c clusterRegisterUtil) CloudExists() (bool, string, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s", c.AstraHostUrl, c.AstraConnector.Spec.Astra.AccountId, c.AstraConnector.Spec.Astra.CloudId)

	c.Log.WithValues("CloudId", c.AstraConnector.Spec.Astra.CloudId).
		Info("Checking if cloud exists")

	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", c.ApiToken)}
	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodGet, url, nil, headerMap, c.Log)
	defer cancel()
	if err != nil {
		return false, CreateErrorMsg("CloudExists", "GET /clouds error", url, "", "", err), err
	}
	if response.StatusCode != 200 {
		return false, CreateErrorMsg("CloudExists", "GET /clouds non 200 response", url, response.Status, "", err), err
	}

	return true, "", nil
}

func (c clusterRegisterUtil) GetClouds() ([]Cloud, string, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds", c.AstraHostUrl, c.AstraConnector.Spec.Astra.AccountId)

	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", c.ApiToken)}
	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodGet, url, nil, headerMap, c.Log)
	defer cancel()
	if err != nil {
		return nil, CreateErrorMsg("PrivateCloudExists", "GET /clouds error", url, "", "", err), err
	}
	if response.StatusCode != 200 {
		return nil, CreateErrorMsg("PrivateCloudExists", "GET /clouds non 200 response", url, response.Status, "", err), err
	}

	respBody, err := c.readResponseBody(response)
	if err != nil {
		return nil, CreateErrorMsg("PrivateCloudExists", "parse GET /clouds response", url, response.Status, "", err), err
	}

	clouds := &CloudList{}
	err = json.Unmarshal(respBody, &clouds)
	if err != nil {
		return nil, CreateErrorMsg("PrivateCloudExists", "unmarshal response from GET call", url, response.Status, string(respBody), err), err
	}
	return clouds.Items, "", nil
}

func (c clusterRegisterUtil) PrivateCloudExists() (*Cloud, string, error) {
	clouds, errMsg, err := c.GetClouds()
	if err != nil {
		return nil, errMsg, err
	}
	for _, cloud := range clouds {
		if cloud.CloudType == CloudTypePrivate && cloud.Name == "private" {
			return &cloud, "", nil
		}
	}
	return nil, "", nil
}

func (c clusterRegisterUtil) CreateCloud(cloud *Cloud) (*Cloud, string, error) {
	if cloud.ID != "" {
		errMsg := "cannot specify ID for post"
		return nil, errMsg, fmt.Errorf(errMsg)
	}
	body, err := json.Marshal(cloud)
	if err != nil {
		errMsg := "failed to marshall cloud object"
		return nil, errMsg, errors.New(errMsg)
	}

	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds", c.AstraHostUrl, c.AstraConnector.Spec.Astra.AccountId)

	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", c.ApiToken)}
	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodPost, url, body, headerMap, c.Log)
	defer cancel()
	if err != nil {
		return nil, CreateErrorMsg("CreateCloud", "POST /clouds error", url, "", "", err), err
	}
	if response.StatusCode != http.StatusCreated {
		return nil, CreateErrorMsg("CreateCloud", "POST /clouds non 201 response", url, response.Status, "", err), err
	}
	respBody, err := c.readResponseBody(response)
	if err != nil {
		return nil, CreateErrorMsg("CreateCloud", "parse POST /clouds response", url, response.Status, "", err), err
	}

	newCloud := &Cloud{}
	err = json.Unmarshal(respBody, &newCloud)
	if err != nil {
		return nil, CreateErrorMsg("CreateCloud", "unmarshal response from POST call", url, response.Status, string(respBody), err), err
	}

	return newCloud, "", nil
}

func (c clusterRegisterUtil) RegisterCloud() (string, string, error) {
	// If CloudID was included in the spec, check if it exists
	if c.AstraConnector.Spec.Astra.CloudId != "" {
		exists, errMsg, err := c.CloudExists()
		if err != nil {
			c.Log.WithValues("cloudId", c.AstraConnector.Spec.Astra.CloudId).Error(err, "error checking if cloud exists")
			return "", errMsg, err
		}
		if !exists {
			errMsg := fmt.Sprintf("cloud '%s' does not exist", c.AstraConnector.Spec.Astra.CloudId)
			return "", errMsg, errors.New(errMsg)
		}
		if exists {
			return c.AstraConnector.Spec.Astra.CloudId, "", nil
		}
	}

	// If cloudID wasn't included in the spec, check if "private" cloud exists
	existingCloud, errMsg, err := c.PrivateCloudExists()
	if err != nil {
		return "", errMsg, fmt.Errorf("cannot determine if private cloud exists: %w", err)
	}
	if existingCloud != nil {
		c.Log.Info("Private cloud already exists", "cloudID", existingCloud.ID)
		return existingCloud.ID, "", nil
	}

	// If the private cloud type doesn't exist, create one.
	cloud := Cloud{
		Type:      ApiCloudType,
		Version:   ApiCloudVersion,
		Name:      "private",
		CloudType: CloudTypePrivate,
	}
	createdCloud, errMsg, err := c.CreateCloud(&cloud)
	if err != nil {
		return "", errMsg, fmt.Errorf("cannot create cloud: %w", err)
	}

	c.Log.WithValues("cloudId", createdCloud.ID).Info("Created private cloud")
	return createdCloud.ID, "", nil
}

func (c clusterRegisterUtil) ListClusters() ([]Cluster, string, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clusters", c.AstraHostUrl, c.AstraConnector.Spec.Astra.AccountId)
	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", c.ApiToken)}
	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodGet, url, nil, headerMap, c.Log)
	defer cancel()
	if err != nil {
		return nil, CreateErrorMsg("ListClusters", "GET /clusters error", url, "", "", err), err
	}
	if response.StatusCode != http.StatusOK {
		return nil, CreateErrorMsg("ListClusters", "GET /clusters non 201 response", url, response.Status, "", err), err
	}
	respBody, err := c.readResponseBody(response)
	if err != nil {
		return nil, CreateErrorMsg("ListClusters", "GET /clusters response", url, response.Status, "", err), err
	}

	clusters := &ClusterList{}
	err = json.Unmarshal(respBody, &clusters)
	if err != nil {
		return nil, CreateErrorMsg("ListClusters", "unmarshal response from GET call", url, response.Status, string(respBody), err), err
	}
	return clusters.Items, "", nil
}

func (c clusterRegisterUtil) GetCluster(cloudId, clusterId string) (*Cluster, string, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s/clusters/%s", c.AstraHostUrl, c.AstraConnector.Spec.Astra.AccountId, cloudId, clusterId)
	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", c.ApiToken)}
	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodGet, url, nil, headerMap, c.Log)
	defer cancel()
	if err != nil {
		return nil, CreateErrorMsg("GetCluster", "GET /clusters error", url, "", "", err), err
	}
	if response.StatusCode != http.StatusOK {
		return nil, CreateErrorMsg("GetCluster", "GET /clusters non 201 response", url, response.Status, "", err), err
	}
	respBody, err := c.readResponseBody(response)
	if err != nil {
		return nil, CreateErrorMsg("GetCluster", "GET /clusters response", url, response.Status, "", err), err
	}

	cluster := &Cluster{}
	err = json.Unmarshal(respBody, &cluster)
	if err != nil {
		return nil, CreateErrorMsg("GetCluster", "unmarshal response from GET call", url, response.Status, string(respBody), err), err
	}
	return cluster, "", nil
}

func (c clusterRegisterUtil) CreateCluster(inputCluster *Cluster, cloudId string) (*Cluster, string, error) {
	url := fmt.Sprintf("%s/accounts/%s/topology/v1/clouds/%s/clusters", c.AstraHostUrl, c.AstraConnector.Spec.Astra.AccountId, cloudId)
	headerMap := HeaderMap{Authorization: fmt.Sprintf("Bearer %s", c.ApiToken)}
	body, err := json.Marshal(inputCluster)
	if err != nil {
		errMsg := "failed to marshall cluster object"
		return nil, errMsg, errors.New(errMsg)
	}
	response, err, cancel := DoRequest(c.Ctx, c.Client, http.MethodPost, url, body, headerMap, c.Log)
	defer cancel()
	if err != nil {
		return nil, CreateErrorMsg("CreateCluster", "POST /clusters error", url, "", "", err), err
	}
	if response.StatusCode != http.StatusCreated {
		return nil, CreateErrorMsg("CreateCluster", "POST /clusters non 201 response", url, response.Status, "", err), err
	}
	respBody, err := c.readResponseBody(response)
	if err != nil {
		return nil, CreateErrorMsg("CreateCluster", "POST /clusters response", url, response.Status, "", err), err
	}

	cluster := &Cluster{}
	err = json.Unmarshal(respBody, &cluster)
	if err != nil {
		return nil, CreateErrorMsg("CreateCluster", "unmarshal response from POST call", url, response.Status, string(respBody), err), err
	}
	return cluster, "", nil
}

func (c clusterRegisterUtil) checkForDuplicateApiServiceIDs(k8sApiServiceId string) (*Cluster, string, error) {
	clusters, errMsg, err := c.ListClusters()
	if err != nil {
		return nil, errMsg, fmt.Errorf("error listing clusters from Astra Control: %w", err)
	}

	for _, cluster := range clusters {
		sameApiServiceID := cluster.ApiServiceID == k8sApiServiceId
		isV2Cluster := cluster.ConnectorInstall == ""

		if sameApiServiceID {
			if isV2Cluster {
				c.Log.WithValues("matchingClusterId", cluster.ID).
					WithValues("matchingClusterName", cluster.Name).
					Info("Found matching v2 cluster, letting it through")
			} else {
				c.Log.WithValues("matchingClusterId", cluster.ID).
					WithValues("matchingClusterName", cluster.Name).
					Info("Found duplicate v3 cluster")
				return &cluster, "", nil
			}
		}
	}
	return nil, "", nil
}

func (c clusterRegisterUtil) checkClusterRegistrationStatus(cloudId, clusterId string, k8sApiServiceId string) (isAlreadyRegistered bool, err error) {
	cluster, _, err := c.GetCluster(cloudId, clusterId)
	if err != nil {
		return false, fmt.Errorf("error getting cluster from Astra Control: %w", err)
	}
	return c.checkRegistrationStatus(cluster, k8sApiServiceId)
}

func (c clusterRegisterUtil) checkRegistrationStatus(cluster *Cluster, k8sApiServiceID string) (isAlreadyRegistered bool, err error) {
	if cluster.ApiServiceID != "" && cluster.ApiServiceID != k8sApiServiceID {
		return false, fmt.Errorf("cluster is incompatible: " +
			"cluster record's apiServiceID does not match current cluster's apiServiceID")
	}

	if cluster.ApiServiceID == "" && cluster.ConnectorInstall != ConnectorInstallStatePending {
		return false, fmt.Errorf("cluster is incompatible: "+
			"cluster record's apiServiceID is empty but connectorInstall is not '%s'",
			ConnectorInstallStatePending)
	}

	if cluster.ConnectorInstall == "" {
		return false, fmt.Errorf("cluster is incompatible: " +
			"connectorInstall field cannot be empty")
	}

	if cluster.ConnectorInstall == ConnectorInstallStateInstalled {
		if cluster.ManagedState != ClusterManagedStateManaged {
			return false, fmt.Errorf("cluster is incompatible: "+
				"connectorInstall is '%s' but cluster state is not '%s'",
				ConnectorInstallStateInstalled, ClusterManagedStateManaged)
		}
		return true, nil
	}

	if cluster.ConnectorInstall != ConnectorInstallStatePending {
		return false, fmt.Errorf("cluster is incompatible: "+
			"expected connectorInstall field to be '%s' or '%s' but got '%s'",
			ConnectorInstallStatePending, ConnectorInstallStateInstalled, cluster.ConnectorInstall)
	}

	if cluster.State != ClusterStatePending {
		err := fmt.Errorf("cluster is incompatible: "+
			"expected cluster state field to be '%s' but got '%s'",
			ClusterStatePending, cluster.State)

		return false, err
	}

	return false, nil
}

// RegisterCluster runs through the necessary steps to establish a connection with Astra Control and ensure
// both the cluster and its record in Astra Control are valid, in which case the cluster will be managed
// (if it hasn't been already).
func (c clusterRegisterUtil) RegisterCluster(cloudId string, k8sApiServiceId string) (string, string, error) {

	// If this connector is started without a particular clusterID, create a pending cluster
	// record with the provided clusterName.
	clusterId := ""
	if c.AstraConnector.Spec.Astra.ClusterId == "" {
		// Check for a duplicate cluster (by ApiServiceID) before continuing
		c.Log.Info("Checking for duplicate cluster records")

		dupeCluster, errMsg, err := c.checkForDuplicateApiServiceIDs(k8sApiServiceId)
		if dupeCluster == nil && err != nil {
			// Legit error
			return "", errMsg, fmt.Errorf("error checking for duplicate apiServiceID records: %w", err)
		} else if dupeCluster != nil && err == nil {
			// Found a duplicate cluster, check if the state of the cluster is already managed.
			registered, err := c.checkRegistrationStatus(dupeCluster, k8sApiServiceId)
			if err != nil {
				errMsg := fmt.Sprintf("unable to check duplicate cluster registration status")
				return "", errMsg, fmt.Errorf("%s: %w", errMsg, err)
			}
			if registered {
				c.Log.WithValues("clusterId", dupeCluster.ID).Info("Found duplicate cluster record already registered")
				return dupeCluster.ID, "", nil
			}
			clusterId = dupeCluster.ID
		} else {
			// Cluster not found, go create a new pending cluster record
			c.Log.WithValues("clusterName", c.AstraConnector.Spec.Astra.ClusterName).
				Info("Creating a new pending cluster record")

			pendingCluster := &Cluster{}
			pendingCluster.Type = "application/astra-cluster"
			pendingCluster.Version = "1.6"
			pendingCluster.CloudID = cloudId
			pendingCluster.Name = c.AstraConnector.Spec.Astra.ClusterName
			pendingCluster.ConnectorInstall = ConnectorInstallStatePending
			pendingCluster.ApiServiceID = k8sApiServiceId

			cluster, errMsg, err := c.CreateCluster(pendingCluster, cloudId)
			if err != nil {
				return "", errMsg, fmt.Errorf("unable to create pending cluster record: %w", err)
			}
			clusterId = cluster.ID
		}
	} else {
		clusterId = c.AstraConnector.Spec.Astra.ClusterId
		// Check that the provided cluster exists
		cluster, _, err := c.GetCluster(cloudId, clusterId)
		if cluster == nil {
			errMsg := fmt.Sprintf("Cluster with ID '%s' not found, please check that the provided ID is correct", clusterId)
			return "", errMsg, errors.New(errMsg)
		}

		c.Log.Info("Checking if Astra Control cluster is already registered")
		isAlreadyRegistered, err := c.checkClusterRegistrationStatus(cloudId, c.AstraConnector.Spec.Astra.ClusterId, k8sApiServiceId)
		if err != nil {
			errMsg := fmt.Sprintf("error checking if cluster is already registered: %s", err)
			return "", errMsg, errors.New(errMsg)
		}
		if isAlreadyRegistered {
			c.Log.Info("Cluster is already registered, skipping registration.")
			return c.AstraConnector.Spec.Astra.ClusterId, "", nil
		}

		// Check for a duplicate cluster (by ApiServiceID) before continuing
		c.Log.Info("Checking for duplicate cluster records")
		dupeCluster, errMsg, err := c.checkForDuplicateApiServiceIDs(k8sApiServiceId)
		if dupeCluster == nil && err != nil {
			// Legit error
			return "", errMsg, fmt.Errorf("error checking for duplicate apiServiceID records: %w", err)
		}
		if dupeCluster != nil {
			errMsg = "duplicate cluster found in Astra Control: apiServiceID matches but the " +
				"clusterId does not"
			return "", errMsg, fmt.Errorf(errMsg)
		}
	}

	fullVersion, semanticVersion, err := c.K8sUtil.VersionGet()
	if err != nil {
		errMsg := "failed to get k8s version of host cluster"
		c.Log.Error(err, "failed to get k8s version of host cluster")
		return "", errMsg, fmt.Errorf("%s: %w", errMsg, err)
	}

	clusterTypeChecker := k8s.NewClusterTypeChecker(c.K8sUtil, c.Log)
	clusterType := clusterTypeChecker.DetermineClusterType()

	c.Log.WithValues("K8sDistro", clusterType).Info("Detected Kubernetes distro")

	c.Log.WithValues(
		"K8sApiServiceId", k8sApiServiceId,
		"clusterType", clusterType,
		"clusterK8sVersion", fullVersion,
		"astraURL", c.AstraConnector.Spec.NatsSyncClient.CloudBridgeURL,
		"connectorNamespace", c.AstraConnector.Namespace,
	).Info("Managing cluster in Astra Control")

	managedCluster := &Cluster{
		ConnectorInstall:      ConnectorInstallStateInstalled,
		ConnectorCapabilities: common.GetConnectorCapabilities(),
		ApiServiceID:          k8sApiServiceId,
		ConnectorNamespace:    c.AstraConnector.Namespace,
		AstraControlURL:       c.AstraConnector.Spec.NatsSyncClient.CloudBridgeURL,
		APITokenRef:           c.AstraConnector.Spec.Astra.TokenRef,
		ClusterType:           clusterType,
		ClusterVersion:        semanticVersion,
		ClusterVersionString:  fullVersion,
		ID:                    clusterId,
	}
	if errMsg, err := c.CreateManagedCluster(managedCluster); err != nil {
		return "", errMsg, err
	}

	c.Log.WithValues(
		"clusterId", clusterId,
		"clusterName", managedCluster.Name,
	).Info("Registration completed successfully")
	return clusterId, "", nil
}
