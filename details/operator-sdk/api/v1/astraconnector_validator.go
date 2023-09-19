// Copyright (c) 2023 NetApp, Inc. All Rights Reserved.

package v1

import (
	"context"
	"fmt"
	"github.com/NetApp-Polaris/astra-connector-operator/common"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/kubernetes"
	"net"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/NetApp-Polaris/astra-connector-operator/util"
)

var log = ctrllog.FromContext(context.TODO())

func (ai *AstraConnector) ValidateCreateAstraConnector() field.ErrorList {
	var allErrs field.ErrorList

	if err := ai.ValidateNamespace(); err != nil {
		log.V(3).Info("error while creating AstraConnector Instance", "namespace", ai.Namespace, "err", err)
		allErrs = append(allErrs, err)
	}

	if err := ai.ValidateTokenAndAccountID(); err != nil {
		log.V(3).Info("error while creating AstraConnector Instance", "namespace", ai.Namespace, "err", err)
		allErrs = append(allErrs, err)
	}

	return allErrs
}

func (ai *AstraConnector) ValidateUpdateAstraConnector() field.ErrorList {
	// TODO - Add validations here
	astraConnectorLog.Info("Updating AstraConnector resource")
	return nil
}

// ValidateNamespace Validates the namespace that AstraConnector should be deployed to.
func (ai *AstraConnector) ValidateNamespace() *field.Error {
	namespaceJsonField := util.GetJSONFieldName(&ai.ObjectMeta, &ai.ObjectMeta.Namespace)
	if ai.GetNamespace() == "default" {
		log.Info("Deploying to default namespace is not allowed. Please select a different namespace.")
		return field.Invalid(field.NewPath(namespaceJsonField), ai.Name, "default namespace not allowed")
	}
	return nil
}

// ValidateTokenAndAccountID Validates the token and AccoundID provided that AstraConnector should be deployed to.
func (ai *AstraConnector) ValidateTokenAndAccountID() *field.Error {
	cloudBridgeJsonField := util.GetJSONFieldName(&ai.Spec, &ai.Spec.NatsSyncClient.CloudBridgeURL)
	tokenRefBridgeJsonField := util.GetJSONFieldName(&ai.Spec, &ai.Spec.Astra.TokenRef)
	accountJsonField := util.GetJSONFieldName(&ai.Spec, &ai.Spec.Astra.AccountId)
	astraHost := getAstraHostURL(ai.Spec.NatsSyncClient.CloudBridgeURL)
	accountId := ai.Spec.Astra.AccountId

	config, _ := ctrl.GetConfig()
	clientset, _ := kubernetes.NewForConfig(config)
	apiToken, err := getSecret(clientset, ai.Spec.Astra.TokenRef, ai.ObjectMeta.Namespace)
	if err != nil {
		log.Info("Check TokenRef, make sure Kubernetes secret was created.")
		return field.NotFound(field.NewPath(tokenRefBridgeJsonField), ai.Name)
	}

	httpClient, err := util.SetHttpClient(ai.Spec.Astra.SkipTLSValidation,
		astraHost, ai.Spec.NatsSyncClient.HostAliasIP, log)
	if err != nil {
		log.Info(fmt.Sprintf("invalid cloudBridgeURL provided: %s, format - https://hostname", ai.Spec.NatsSyncClient.CloudBridgeURL))
		return field.Invalid(field.NewPath(cloudBridgeJsonField), ai.Name, "CloudBridgeURL invalid format")
	}

	url := fmt.Sprintf("%s/accounts/%s", astraHost, accountId)

	headerMap := util.HeaderMap{Authorization: fmt.Sprintf("Bearer %s", apiToken)}
	response, err, cancel := util.DoRequest(context.Background(), httpClient, http.MethodGet, url, nil, headerMap)
	defer cancel()

	var dnsError *net.DNSError
	if errors.As(err, &dnsError) {
		log.Info("Please check CloudBridgeURL provided")
		return field.Invalid(field.NewPath(cloudBridgeJsonField), ai.Name, "CloudBridgeURL not reachable")
	}

	// We got a 200 from the GET Account Looks good! no errors
	if response.StatusCode == 200 {
		return nil
	}

	// error handling below
	if response.StatusCode == 401 {
		log.Info("Please check token provided.. 401 Unauthorized response status code from Astra Control")
		return field.Invalid(field.NewPath(tokenRefBridgeJsonField), ai.Name, "Unauthorized request with Token Provided")
	}

	if response.StatusCode == 404 {
		println("Please check account id provided.. 404 account not found in Astra Control")
		return field.Invalid(field.NewPath(accountJsonField), ai.Name, "Account not found")
	}
	return nil
}

func getAstraHostURL(cloudBridgeURL string) string {
	var astraHost string
	if cloudBridgeURL != "" {
		astraHost = cloudBridgeURL
	} else {
		astraHost = common.NatsSyncClientDefaultCloudBridgeURL
	}
	return astraHost
}

func getSecret(clientset *kubernetes.Clientset, secretName string, namespace string) (string, error) {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		log.WithValues("namespace", namespace, "secret", secretName).Error(err, "failed to get kubernetes secret")
		return "", err
	}
	// Extract the value of the 'apiToken' key from the secret
	apiToken, ok := secret.Data["apiToken"]
	if !ok {
		log.WithValues("namespace", namespace, "secret", secretName).Error(err, "failed to extract apiToken key from secret")
		return "", errors.New("failed to extract apiToken key from secret")
	}

	// Convert the value to a string
	apiTokenStr := string(apiToken)
	return apiTokenStr, nil
}
