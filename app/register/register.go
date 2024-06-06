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

// ************************************************
//  FUNCTIONS TO REGISTER CLUSTER WITH ASTRA
// ************************************************

func GetAstraHostURL(astraConnector *v1.AstraConnector) string {
	var astraHost string
	if astraConnector.Spec.Astra.AstraControlURL != "" {
		astraHost = astraConnector.Spec.Astra.AstraControlURL
		astraHost = strings.TrimSuffix(astraHost, "/")
	} else {
		astraHost = common.DefaultCloudAstraControlURL
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

	if c.AstraConnector.Spec.Astra.HostAliasIP != "" {
		c.Log.WithValues("HostAliasIP", c.AstraConnector.Spec.Astra.HostAliasIP).Info("Using the HostAlias IP")
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
				addr = c.AstraConnector.Spec.Astra.HostAliasIP + ":443"
			}
			if addr == cloudBridgeHost+":80" {
				addr = c.AstraConnector.Spec.Astra.HostAliasIP + ":80"
			}
			return dialer.DialContext(ctx, network, addr)
		}
	}

	c.Client = &http.Client{}
	return nil
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
