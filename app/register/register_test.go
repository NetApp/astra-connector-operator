/*
 * Copyright (c) 2024. NetApp, Inc. All Rights Reserved.
 */

package register_test

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"

	"github.com/NetApp-Polaris/astra-connector-operator/app/register"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
	"github.com/NetApp-Polaris/astra-connector-operator/mocks"
	testutil "github.com/NetApp-Polaris/astra-connector-operator/test/test-util"
)

const (
	testNamespace = "test-namespace"
	testCloudId   = "9876"
	testClusterId = "1234"
	testURL       = "test_url"
	testIP        = "test_ip"
)

var ctx = context.Background()

func setupTokenSecret(secretName string, k8sClient client.Client) {
	secretObj := &coreV1.Secret{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      secretName,
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"apiToken": []byte("auth-token"),
		},
	}

	_ = k8sClient.Create(ctx, secretObj)
}

type AstraConnectorInput struct {
	createTokenSecret  bool
	cloudId            bool
	clusterId          bool
	invalidHostDetails bool
}

func createClusterRegister(astraConnectorInput AstraConnectorInput) (register.ClusterRegisterUtil, *mocks.HTTPClient, string, client.Client) {
	log := testutil.CreateLoggerForTesting()
	mockHttpClient := &mocks.HTTPClient{}
	fakeClient := testutil.CreateFakeClient()
	k8sUtil := &mocks.K8sUtilInterface{}
	k8sUtil.On("RESTGet", mock.Anything).Return(nil, errors.New("test"))
	k8sUtil.On("VersionGet").Return("1.0.0", nil)
	k8sUtil.On("K8sClientset").Return(fake.NewSimpleClientset())
	apiTokenSecret := "astra-token"

	astraConnector := &v1.AstraConnector{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "test-astra-connector",
			Namespace: testNamespace,
		},
		Spec: v1.AstraConnectorSpec{
			Astra: v1.Astra{
				TokenRef: apiTokenSecret,
			},
			ImageRegistry: v1.ImageRegistry{
				Name:   "test-registry",
				Secret: "test-secret",
			},
			AutoSupport: v1.AutoSupport{
				Enrolled: true,
				URL:      "https://my-asup",
			},
			AstraConnect: v1.AstraConnect{
				Image:    "test-image",
				Replicas: 2,
			},
		},
	}

	if astraConnectorInput.createTokenSecret {
		apiTokenSecret = uuid.New().String()
		setupTokenSecret(apiTokenSecret, fakeClient)
		astraConnector.Spec.Astra.TokenRef = apiTokenSecret
	}

	if astraConnectorInput.cloudId {
		astraConnector.Spec.Astra.CloudId = testCloudId
	}

	if astraConnectorInput.clusterId {
		astraConnector.Spec.Astra.ClusterId = testClusterId
	}

	if astraConnectorInput.invalidHostDetails {
		astraConnector.Spec.Astra.AstraControlURL = testURL
		astraConnector.Spec.Astra.HostAliasIP = testIP
	}

	clusterRegisterUtil := register.NewClusterRegisterUtil(astraConnector, mockHttpClient, fakeClient, k8sUtil, log, context.Background())
	return clusterRegisterUtil, mockHttpClient, apiTokenSecret, fakeClient
}

// Tests

func TestGetAPITokenFromSecret(t *testing.T) {
	t.Run("GetAPITokenFromSecret__SecretNotPresentReturnsError", func(t *testing.T) {
		clusterRegisterUtil, _, _, _ := createClusterRegister(AstraConnectorInput{})
		apiToken, errorReason, err := clusterRegisterUtil.GetAPITokenFromSecret("astra-token")

		assert.Equal(t, apiToken, "")
		assert.Equal(t, "Failed to get secret astra-token", errorReason)
		assert.EqualError(t, err, "secrets \"astra-token\" not found")
	})

	t.Run("GetAPITokenFromSecret__SecretInvalidReturnsError", func(t *testing.T) {
		clusterRegisterUtil, _, apiTokenSecret, fakeClient := createClusterRegister(AstraConnectorInput{})

		secret := &coreV1.Secret{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      apiTokenSecret,
				Namespace: testNamespace,
			},
			Data: map[string][]byte{
				"api-token": []byte("auth-token"),
			},
		}

		// creating secret
		err := fakeClient.Create(ctx, secret)
		assert.NoError(t, err)

		apiToken, errorReason, err := clusterRegisterUtil.GetAPITokenFromSecret(apiTokenSecret)

		assert.Equal(t, apiToken, "")
		assert.Equal(t, "Failed to extract 'apiToken' key from secret astra-token", errorReason)
		assert.EqualError(t, err, "failed to extract apiToken key from secret")
	})

	t.Run("GetAPITokenFromSecret__ReturnsApiToken", func(t *testing.T) {
		clusterRegisterUtil, _, apiTokenSecret, _ := createClusterRegister(AstraConnectorInput{createTokenSecret: true})

		apiToken, errorReason, err := clusterRegisterUtil.GetAPITokenFromSecret(apiTokenSecret)
		assert.Equal(t, apiToken, "auth-token")
		assert.Equal(t, "", errorReason)
		assert.NoError(t, err)
	})
}
