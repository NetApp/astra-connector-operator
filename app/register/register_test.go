/*
 * Copyright (c) 2023. NetApp, Inc. All Rights Reserved.
 */

package register_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NetApp-Polaris/astra-connector-operator/app/register"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
	"github.com/NetApp-Polaris/astra-connector-operator/mocks"
	testutil "github.com/NetApp-Polaris/astra-connector-operator/test/test-util"
)

const (
	testNamespace        = "test-namespace"
	testStorageClassName = "test-sc"
	testCloudId          = "9876"
	testClusterId        = "1234"
	testURL              = "test_url"
	testIP               = "test_ip"
)

var ctx = context.Background()

type mockRead struct {
	mock.Mock
}

func (m *mockRead) Read(in []byte) (n int, err error) {
	return m.Called(in).Int(0), m.Called(in).Error(1)
}

func (m *mockRead) Close() error {
	return m.Called().Error(0)
}

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
	storageClassName   bool
	cloudId            bool
	clusterId          bool
	invalidHostDetails bool
}

func createClusterRegister(astraConnectorInput AstraConnectorInput) (register.ClusterRegisterUtil, *mocks.HTTPClient, string, client.Client) {
	log := testutil.CreateLoggerForTesting()
	mockHttpClient := &mocks.HTTPClient{}
	fakeClient := testutil.CreateFakeClient()
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

	if astraConnectorInput.storageClassName {
		astraConnector.Spec.Astra.StorageClassName = testStorageClassName
	}

	if astraConnectorInput.cloudId {
		astraConnector.Spec.Astra.CloudId = testCloudId
	}

	if astraConnectorInput.clusterId {
		astraConnector.Spec.Astra.ClusterId = testClusterId
	}

	if astraConnectorInput.invalidHostDetails {
		astraConnector.Spec.NatsSyncClient.CloudBridgeURL = testURL
		astraConnector.Spec.NatsSyncClient.HostAliasIP = testIP
	}

	clusterRegisterUtil := register.NewClusterRegisterUtil(astraConnector, mockHttpClient, fakeClient, log, context.Background())
	return clusterRegisterUtil, mockHttpClient, apiTokenSecret, fakeClient
}

// Tests

func TestGetConnectorIDFromConfigMap(t *testing.T) {
	t.Run("TestGetConnectorIDFromConfigMap__ReturnsConnectorID", func(t *testing.T) {
		cmData := map[string]string{
			"cloud-master_locationData.json": "",
			"validKey":                       `{"locationID":"testConnectorID"}`,
		}

		clusterRegisterUtil, _, _, _ := createClusterRegister(AstraConnectorInput{})
		id, err := clusterRegisterUtil.GetConnectorIDFromConfigMap(cmData)

		assert.NoError(t, err)
		assert.Equal(t, "testConnectorID", id)
	})

	t.Run("TestGetConnectorIDFromConfigMap__UnmarshallError", func(t *testing.T) {
		cmData := map[string]string{
			"invalidKey": `{"name":"Jane","age":25`,
		}

		clusterRegisterUtil, _, _, _ := createClusterRegister(AstraConnectorInput{})
		id, err := clusterRegisterUtil.GetConnectorIDFromConfigMap(cmData)

		assert.Equal(t, "", id)
		assert.EqualError(t, err, "unexpected end of JSON input")
	})
}

func TestGetNatsSyncClientRegistrationURL(t *testing.T) {
	t.Run("TestGetNatsSyncClientRegistrationURL__ReturnsValidURL", func(t *testing.T) {
		clusterRegisterUtil, _, _, _ := createClusterRegister(AstraConnectorInput{})
		url := clusterRegisterUtil.GetNatsSyncClientRegistrationURL()

		expectedURL := "http://natssync-client.test-namespace:8080/bridge-client/1/register"
		assert.Equal(t, expectedURL, url)
	})
}

func TestGetNatsSyncClientUnregisterURL(t *testing.T) {
	t.Run("TestGetNatsSyncClientUnregisterURL__ReturnsValidURL", func(t *testing.T) {
		clusterRegisterUtil, _, _, _ := createClusterRegister(AstraConnectorInput{})
		url := clusterRegisterUtil.GetNatsSyncClientUnregisterURL()

		expectedURL := "http://natssync-client.test-namespace:8080/bridge-client/1/unregister"
		assert.Equal(t, expectedURL, url)
	})
}

func TestUnRegisterNatsSyncClient(t *testing.T) {
	t.Run("TestUnRegisterNatsSyncClient__InvalidAuthPayloadReturnsError", func(t *testing.T) {
		clusterRegisterUtil, _, _, _ := createClusterRegister(AstraConnectorInput{})
		err := clusterRegisterUtil.UnRegisterNatsSyncClient()

		assert.EqualError(t, err, "secrets \"astra-token\" not found")
	})

	t.Run("TestUnRegisterNatsSyncClient__HTTPPostRequestFailsReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{createTokenSecret: true})

		errorText := "error on post request"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		err := clusterRegisterUtil.UnRegisterNatsSyncClient()
		assert.EqualError(t, err, errorText)
	})

	t.Run("TestUnRegisterNatsSyncClient__HTTPPostRequestInvalidStatusCodeReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{createTokenSecret: true})

		ret := io.NopCloser(bytes.NewReader([]byte(`items:{"Name":"Joe","Body":"Hello","Time":1294706395881547069}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 400,
			Body:       ret,
		}, nil).Once()

		err := clusterRegisterUtil.UnRegisterNatsSyncClient()
		assert.ErrorContains(t, err, "Unexpected unregistration status code: 400")
	})

	t.Run("TestUnRegisterNatsSyncClient__ReadResponseBodyErrorReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{createTokenSecret: true})

		mockRead := mockRead{}
		mockRead.On("Read", mock.Anything).Return(0, errors.New("error reading"))
		mockRead.On("Close").Return(errors.New("error closing"))

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 400,
			Body:       &mockRead,
		}, nil).Once()

		err := clusterRegisterUtil.UnRegisterNatsSyncClient()
		assert.EqualError(t, err, "error reading")
	})

	t.Run("TestUnRegisterNatsSyncClient__OnSuccessReturnNil", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{createTokenSecret: true})

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 204,
			Body:       nil,
		}, nil).Once()

		err := clusterRegisterUtil.UnRegisterNatsSyncClient()
		assert.Nil(t, err)
	})
}

func TestRegisterNatsSyncClient(t *testing.T) {
	t.Run("TestRegisterNatsSyncClient__InvalidAuthPayloadReturnsError", func(t *testing.T) {
		clusterRegisterUtil, _, _, _ := createClusterRegister(AstraConnectorInput{})
		connectorId, err := clusterRegisterUtil.RegisterNatsSyncClient()

		assert.Equal(t, "", connectorId)
		assert.EqualError(t, err, "secrets \"astra-token\" not found")
	})

	t.Run("TestRegisterNatsSyncClient__HTTPPostRequestFailsReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{createTokenSecret: true})

		errorText := "error on post request create"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		connectorId, err := clusterRegisterUtil.RegisterNatsSyncClient()

		assert.Equal(t, "", connectorId)
		assert.EqualError(t, err, errorText)
	})

	t.Run("TestUnRegisterNatsSyncClient__HTTPPostRequestInvalidStatusCodeReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{createTokenSecret: true})

		ret := io.NopCloser(bytes.NewReader([]byte(`items:{"Name":"Joe","Body":"Hello","Time":1294706395881547069}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 400,
			Body:       ret,
		}, nil).Once()

		connectorId, err := clusterRegisterUtil.RegisterNatsSyncClient()

		assert.Equal(t, "", connectorId)
		assert.ErrorContains(t, err, "Unexpected registration status code: 400")
	})

	t.Run("TestUnRegisterNatsSyncClient__ReadResponseBodyErrorReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{createTokenSecret: true})

		mockRead := mockRead{}
		mockRead.On("Read", mock.Anything).Return(0, errors.New("error reading"))
		mockRead.On("Close").Return(errors.New("error closing"))

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 400,
			Body:       &mockRead,
		}, nil).Once()

		connectorId, err := clusterRegisterUtil.RegisterNatsSyncClient()

		assert.Equal(t, "", connectorId)
		assert.EqualError(t, err, "error reading")
	})

	t.Run("TestUnRegisterNatsSyncClient__OnSuccessReturnConnectorId", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{createTokenSecret: true})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"locationID":"test-connectorID"}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       ret,
		}, nil).Once()

		connectorId, err := clusterRegisterUtil.RegisterNatsSyncClient()

		assert.Equal(t, "test-connectorID", connectorId)
		assert.Nil(t, err)
	})

	t.Run("TestUnRegisterNatsSyncClient__OnSuccessButInvalidJSONBodyReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{createTokenSecret: true})

		ret := io.NopCloser(bytes.NewReader([]byte(`"locationID":"test-connectorID"}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       ret,
		}, nil).Once()

		connectorId, err := clusterRegisterUtil.RegisterNatsSyncClient()

		assert.Equal(t, "", connectorId)
		assert.NotNil(t, err)
	})
}

func TestCloudExists(t *testing.T) {
	host, cloudId, apiToken := "test_host", "test_cloudId", "test_apiToken"

	t.Run("TestCloudExists__HTTPGetRequestFailsReturnFalse", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		errorText := "error on get request"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		cloudExists := clusterRegisterUtil.CloudExists(host, cloudId, apiToken)
		assert.Equal(t, false, cloudExists)
	})

	t.Run("TestCloudExists__HTTPStatusNotFoundCodeReturnFalse", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 404,
			Body:       nil,
		}, nil).Once()

		cloudExists := clusterRegisterUtil.CloudExists(host, cloudId, apiToken)
		assert.Equal(t, false, cloudExists)
	})

	t.Run("TestCloudExists__HTTPStatusInvalidStatusCodeReturnFalse", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 202,
			Body:       nil,
		}, nil).Once()

		cloudExists := clusterRegisterUtil.CloudExists(host, cloudId, apiToken)
		assert.Equal(t, false, cloudExists)
	})

	t.Run("TestCloudExists__HTTPStatusValidStatusCodeReturnTrue", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       nil,
		}, nil).Once()

		cloudExists := clusterRegisterUtil.CloudExists(host, cloudId, apiToken)
		assert.Equal(t, true, cloudExists)
	})
}

func TestListClouds(t *testing.T) {
	host, apiToken := "test_host", "test_apiToken"

	t.Run("TestListClouds__HTTPGetRequestFailsReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		errorText := "error on get request"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		cloudsResp, err := clusterRegisterUtil.ListClouds(host, apiToken)
		assert.Nil(t, cloudsResp)
		assert.EqualError(t, err, "error on get request")
	})

	t.Run("TestListClouds__ReturnCloudsResponse", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{storageClassName: true})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"items":[{"id":"1234","name":"cloud1"}, {"id":"5678","name":"cloud2"}]}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		cloudsResp, err := clusterRegisterUtil.ListClouds(host, apiToken)
		assert.NotNil(t, cloudsResp)
		assert.Nil(t, err)
	})
}

func TestGetCloudId(t *testing.T) {
	host, cloudType, apiToken := "test_host", "private", "test_apiToken"

	t.Run("TestGetCloudId__ListCloudsFailReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		errorText := "error on get request"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		cloudId, err := clusterRegisterUtil.GetCloudId(host, cloudType, apiToken, 3*time.Second)

		assert.Equal(t, "", cloudId)
		assert.EqualError(t, err, "timed out querying Astra API")
	})

	t.Run("TestGetCloudId__ListCloudsInvalidStatusCodeReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"items":[{"id":"1234","name":"cloud1", "cloudType":"private"}, {"id":"5678","name":"cloud2","cloudType":"not-private"}]}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 401,
			Body:       ret,
		}, nil)

		cloudId, err := clusterRegisterUtil.GetCloudId(host, cloudType, apiToken, 3*time.Second)

		assert.Equal(t, "", cloudId)
		assert.EqualError(t, err, "timed out querying Astra API")
	})

	t.Run("TestGetCloudId__ReadResponseBodyErrorReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		mockRead := mockRead{}
		mockRead.On("Read", mock.Anything).Return(0, errors.New("error reading"))
		mockRead.On("Close").Return(errors.New("error closing"))

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       &mockRead,
		}, nil).Once()

		cloudId, err := clusterRegisterUtil.GetCloudId(host, cloudType, apiToken)

		assert.Equal(t, "", cloudId)
		assert.EqualError(t, err, "error reading")
	})

	t.Run("TestGetCloudId__UnmarshalBodyErrorReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`items:{"Name":"Joe","Body":"Hello","Time":1294706395881547069`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		cloudId, err := clusterRegisterUtil.GetCloudId(host, cloudType, apiToken)

		assert.Equal(t, "", cloudId)
		assert.EqualError(t, err, "invalid character 'i' looking for beginning of value")
	})

	t.Run("TestGetCloudId__ReturnEmptyCloudIdWhenNoPrivateCloudType", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{storageClassName: true})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"items":[{"id":"1234","name":"cloud1", "cloudType":"not-private"}, {"id":"5678","name":"cloud2","cloudType":"not-private"}]}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		cloudId, err := clusterRegisterUtil.GetCloudId(host, cloudType, apiToken)

		assert.Equal(t, "", cloudId)
		assert.Nil(t, err)
	})

	t.Run("TestGetCloudId__ReturnCloudIdOfTypePrivate", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{storageClassName: true})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"items":[{"id":"1234","name":"cloud1", "cloudType":"private"}, {"id":"5678","name":"cloud2","cloudType":"not-private"}]}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		cloudId, err := clusterRegisterUtil.GetCloudId(host, cloudType, apiToken)

		assert.Equal(t, "1234", cloudId)
		assert.Nil(t, err)
	})
}

func TestCreateCloud(t *testing.T) {
	host, cloudType, apiToken := "test_host", "private", "test_apiToken"

	t.Run("TestCreateCloud__HTTPPostRequestFailsReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		errorText := "error on post request create"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		cloudId, err := clusterRegisterUtil.CreateCloud(host, cloudType, apiToken)

		assert.Equal(t, "", cloudId)
		assert.EqualError(t, err, "error on post request create")
	})

	t.Run("TestCreateCloud__ReadResponseBodyErrorReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		mockRead := mockRead{}
		mockRead.On("Read", mock.Anything).Return(0, errors.New("error reading"))
		mockRead.On("Close").Return(errors.New("error closing"))

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       &mockRead,
		}, nil).Once()

		cloudId, err := clusterRegisterUtil.CreateCloud(host, cloudType, apiToken)

		assert.Equal(t, "", cloudId)
		assert.EqualError(t, err, "error reading response: error reading")
	})

	t.Run("TestCreateCloud__UnmarshalBodyErrorReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`items:{"Name":"Joe","Body":"Hello","Time":1294706395881547069`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       ret,
		}, nil).Once()

		cloudId, err := clusterRegisterUtil.CreateCloud(host, cloudType, apiToken)

		assert.Equal(t, "", cloudId)
		assert.ErrorContains(t, err, "error unmarshalling response")
	})

	t.Run("TestCreateCloud__GotEmptyCloudIDReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"id":"","name":""}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       ret,
		}, nil).Once()

		cloudId, err := clusterRegisterUtil.CreateCloud(host, cloudType, apiToken)

		assert.Equal(t, "", cloudId)
		assert.Nil(t, err)
	})

	t.Run("TestCreateCloud__CloudCreatedReturnCloudId", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"id":"1234","name":"test-cloud"}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       ret,
		}, nil).Once()

		cloudId, err := clusterRegisterUtil.CreateCloud(host, cloudType, apiToken)

		assert.Equal(t, "1234", cloudId)
		assert.Nil(t, err)
	})
}

func TestGetOrCreateCloud(t *testing.T) {
	host, cloudType, apiToken := "test_host", "private", "test_apiToken"

	t.Run("TestGetOrCreateCloud__InvalidCloudIdProvidedInTheSpecReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{cloudId: true})

		errorText := "error on get request"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		cloudId, err := clusterRegisterUtil.GetOrCreateCloud(host, cloudType, apiToken)

		assert.Equal(t, "", cloudId)
		assert.EqualError(t, err, "Invalid CloudId provided in the Spec : 9876")
	})

	t.Run("TestGetOrCreateCloud__ValidCloudIdProvidedInTheSpecReturnCloudId", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{cloudId: true})

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       nil,
		}, nil).Once()

		cloudId, err := clusterRegisterUtil.GetOrCreateCloud(host, cloudType, apiToken)

		assert.Equal(t, "9876", cloudId)
		assert.Nil(t, err)
	})

	t.Run("TestGetOrCreateCloud__GetCloudIdReturnsErrorReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`items:{"Name":"Joe","Body":"Hello","Time":1294706395881547069`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		cloudId, err := clusterRegisterUtil.GetOrCreateCloud(host, cloudType, apiToken)

		assert.Equal(t, "", cloudId)
		assert.EqualError(t, err, "invalid character 'i' looking for beginning of value")
	})

	t.Run("TestGetOrCreateCloud__GetCloudIdReturnsCloudReturnCloudId", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"items":[{"id":"1234","name":"cloud1", "cloudType":"private"}, {"id":"5678","name":"cloud2","cloudType":"not-private"}]}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		cloudId, err := clusterRegisterUtil.GetOrCreateCloud(host, cloudType, apiToken)

		assert.Equal(t, "1234", cloudId)
		assert.Nil(t, err)
	})

	t.Run("TestGetOrCreateCloud__CreateCloudReturnsEmptyCloudIdReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"items":[{"id":"1234","name":"cloud1", "cloudType":"not-private"}, {"id":"5678","name":"cloud2","cloudType":"not-private"}]}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		errorText := "error on post request create"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		cloudId, err := clusterRegisterUtil.GetOrCreateCloud(host, cloudType, apiToken)

		assert.Equal(t, "", cloudId)
		assert.EqualError(t, err, "error on post request create")
	})

	t.Run("TestGetOrCreateCloud__CreateCloudReturnsErrorReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"items":[{"id":"1234","name":"cloud1", "cloudType":"not-private"}, {"id":"5678","name":"cloud2","cloudType":"not-private"}]}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		ret = io.NopCloser(bytes.NewReader([]byte(`{"id":"","name":""}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       ret,
		}, nil).Once()

		cloudId, err := clusterRegisterUtil.GetOrCreateCloud(host, cloudType, apiToken)

		assert.Equal(t, "", cloudId)
		assert.EqualError(t, err, "could not create cloud of type private")
	})

	t.Run("TestGetOrCreateCloud__CloudCreatedReturnCloudId", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"items":[{"id":"1234","name":"cloud1", "cloudType":"not-private"}, {"id":"5678","name":"cloud2","cloudType":"not-private"}]}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		ret = io.NopCloser(bytes.NewReader([]byte(`{"id":"1234","name":"test-cloud"}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       ret,
		}, nil).Once()

		cloudId, err := clusterRegisterUtil.GetOrCreateCloud(host, cloudType, apiToken)

		assert.Equal(t, "1234", cloudId)
		assert.Nil(t, err)
	})
}

func TestGetClusters(t *testing.T) {
	host, cloudId, apiToken := "test_host", "test_cloudId", "test_apiToken"

	t.Run("TestGetClusters__HTTPGetRequestFailsReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		errorText := "error on get request"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		clusters, err := clusterRegisterUtil.GetClusters(host, cloudId, apiToken)

		assert.Equal(t, 0, len(clusters.Items))
		assert.EqualError(t, err, "error on request get clusters: error on get request")
	})

	t.Run("TestGetClusters__HTTPGetRequestInvalidStatusCodeReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 401,
			Body:       nil,
		}, nil).Once()

		clusters, err := clusterRegisterUtil.GetClusters(host, cloudId, apiToken)

		assert.Equal(t, 0, len(clusters.Items))
		assert.EqualError(t, err, "get clusters failed 401")
	})

	t.Run("TestGetClusters__ReadResponseBodyErrorReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		mockRead := mockRead{}
		mockRead.On("Read", mock.Anything).Return(0, errors.New("error reading"))
		mockRead.On("Close").Return(errors.New("error closing"))

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       &mockRead,
		}, nil).Once()

		clusters, err := clusterRegisterUtil.GetClusters(host, cloudId, apiToken)

		assert.Equal(t, 0, len(clusters.Items))
		assert.EqualError(t, err, "error reading response from get clusters: error reading")
	})

	t.Run("TestGetClusters__UnmarshalBodyErrorReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`items:{"Name":"Joe","Body":"Hello","Time":1294706395881547069`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		clusters, err := clusterRegisterUtil.GetClusters(host, cloudId, apiToken)

		assert.Equal(t, 0, len(clusters.Items))
		assert.EqualError(t, err, "unmarshall error when getting clusters: invalid character 'i' looking for beginning of value")
	})

	t.Run("TestGetClusters__ReturnClusterResponse", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{storageClassName: true})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"items":[{"id":"1234","name":"cluster1"}, {"id":"5678","name":"cluster2"}]}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		clusters, err := clusterRegisterUtil.GetClusters(host, cloudId, apiToken)

		assert.Equal(t, 2, len(clusters.Items))
		assert.Nil(t, err)
	})
}

func TestGetCluster(t *testing.T) {
	host, cloudId, clusterId, apiToken := "test_host", "test_cloudId", "test_clusterId", "test_apiToken"

	t.Run("TestGetCluster__HTTPGetRequestFailsReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		errorText := "error on get request"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		cluster, err := clusterRegisterUtil.GetCluster(host, cloudId, clusterId, apiToken)

		assert.Equal(t, "", cluster.ID)
		assert.Equal(t, "", cluster.Name)
		assert.EqualError(t, err, "error on request get clusters: error on get request")
	})

	t.Run("TestGetCluster__HTTPGetRequestInvalidStatusCodeReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 401,
			Body:       nil,
		}, nil).Once()

		cluster, err := clusterRegisterUtil.GetCluster(host, cloudId, clusterId, apiToken)

		assert.Equal(t, "", cluster.ID)
		assert.Equal(t, "", cluster.Name)
		assert.EqualError(t, err, "get clusters failed with: 401")
	})

	t.Run("TestGetCluster__ReadResponseBodyErrorReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		mockRead := mockRead{}
		mockRead.On("Read", mock.Anything).Return(0, errors.New("error reading"))
		mockRead.On("Close").Return(errors.New("error closing"))

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       &mockRead,
		}, nil).Once()

		cluster, err := clusterRegisterUtil.GetCluster(host, cloudId, clusterId, apiToken)

		assert.Equal(t, "", cluster.ID)
		assert.Equal(t, "", cluster.Name)
		assert.EqualError(t, err, "error reading response from get clusters: error reading")
	})

	t.Run("TestGetCluster__UnmarshalBodyErrorReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`items:{"Name":"Joe","Body":"Hello","Time":1294706395881547069`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		cluster, err := clusterRegisterUtil.GetCluster(host, cloudId, clusterId, apiToken)

		assert.Equal(t, "", cluster.ID)
		assert.Equal(t, "", cluster.Name)
		assert.EqualError(t, err, "unmarshall error when parsing get clusters response: invalid character 'i' looking for beginning of value")
	})

	t.Run("TestGetCluster__ReturnClusterResponse", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{storageClassName: true})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"id":"1234","name":"this is a cluster"}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		cluster, err := clusterRegisterUtil.GetCluster(host, cloudId, clusterId, apiToken)

		assert.Equal(t, "1234", cluster.ID)
		assert.Equal(t, "this is a cluster", cluster.Name)
		assert.Nil(t, err)
	})
}

func TestCreateCluster(t *testing.T) {
	host, cloudId, connectorId, apiToken := "test_host", "test_cloudId", "test_connectorId", "test_apiToken"

	t.Run("TestCreateCluster__HTTPPostRequestFailsReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		errorText := "error on post request create"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		clusterInfo, err := clusterRegisterUtil.CreateCluster(host, cloudId, connectorId, apiToken)

		assert.Equal(t, "", clusterInfo.ID)
		assert.Equal(t, "", clusterInfo.Name)
		assert.EqualError(t, err, "error on request post clusters: error on post request create")
	})

	t.Run("TestCreateCluster__ReadResponseBodyErrorReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		mockRead := mockRead{}
		mockRead.On("Read", mock.Anything).Return(0, errors.New("error reading"))
		mockRead.On("Close").Return(errors.New("error closing"))

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 400,
			Body:       &mockRead,
		}, nil).Once()

		clusterInfo, err := clusterRegisterUtil.CreateCluster(host, cloudId, connectorId, apiToken)

		assert.Equal(t, "", clusterInfo.ID)
		assert.Equal(t, "", clusterInfo.Name)
		assert.EqualError(t, err, "error reading response from post clusters: error reading")
	})

	t.Run("TestCreateCluster__HTTPPostRequestInvalidStatusCodeReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`items:{"Name":"Joe","Body":"Hello","Time":1294706395881547069}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 400,
			Body:       ret,
		}, nil).Once()

		clusterInfo, err := clusterRegisterUtil.CreateCluster(host, cloudId, connectorId, apiToken)

		assert.Equal(t, "", clusterInfo.ID)
		assert.Equal(t, "", clusterInfo.Name)
		assert.EqualError(t, err, "add cluster failed with: 400")
	})

	t.Run("TestCreateCluster__UnmarshalBodyErrorReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`items:{"Name":"Joe","Body":"Hello","Time":1294706395881547069`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       ret,
		}, nil).Once()

		clusterInfo, err := clusterRegisterUtil.CreateCluster(host, cloudId, connectorId, apiToken)

		assert.Equal(t, "", clusterInfo.ID)
		assert.Equal(t, "", clusterInfo.Name)
		assert.ErrorContains(t, err, "unmarshall error when parsing post clusters response")
	})

	t.Run("TestCreateCluster__GotEmptyClusterIDReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"id":"","name":"","managedState":""}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       ret,
		}, nil).Once()

		clusterInfo, err := clusterRegisterUtil.CreateCluster(host, cloudId, connectorId, apiToken)

		assert.Equal(t, "", clusterInfo.ID)
		assert.Equal(t, "", clusterInfo.Name)
		assert.EqualError(t, err, "got empty id from post clusters response")
	})

	t.Run("TestCreateCluster__ClusterAddedReturnClusterInfo", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"id":"1234","name":"test-cluster","managedState":"unmanaged"}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       ret,
		}, nil).Once()

		clusterInfo, err := clusterRegisterUtil.CreateCluster(host, cloudId, connectorId, apiToken)

		assert.Equal(t, "1234", clusterInfo.ID)
		assert.Equal(t, "test-cluster", clusterInfo.Name)
		assert.Equal(t, "unmanaged", clusterInfo.ManagedState)
		assert.Nil(t, err)
	})
}

func TestUpdateCluster(t *testing.T) {
	host, cloudId, clusterId, connectorId, apiToken := "test_host", "test_cloudId", "test_clusterId", "test_connectorId", "test_apiToken"

	t.Run("TestUpdateCluster__HTTPPutRequestFailsReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		errorText := "error on put request update"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		err := clusterRegisterUtil.UpdateCluster(host, cloudId, clusterId, connectorId, apiToken)
		assert.EqualError(t, err, "error on request put clusters: error on put request update")
	})

	t.Run("TestUpdateCluster__HTTPPutRequestInvalidStatusCodeReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 400,
			Body:       nil,
		}, nil).Once()

		err := clusterRegisterUtil.UpdateCluster(host, cloudId, clusterId, connectorId, apiToken)
		assert.EqualError(t, err, "update cluster failed with: 400")
	})

	t.Run("TestUpdateCluster__ClusterUpdatedReturnNil", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       nil,
		}, nil).Once()

		err := clusterRegisterUtil.UpdateCluster(host, cloudId, clusterId, connectorId, apiToken)
		assert.Nil(t, err)
	})
}

func TestCreateOrUpdateCluster(t *testing.T) {
	host, cloudId, clusterId, connectorId, apiToken, connectorInstall := "test_host", "test_cloudId", "test_clusterId", "test_connectorId", "test_apiToken", "pending"

	t.Run("TestCreateOrUpdateCluster__ReturnsErrorWhenCreateClusterFails", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		errorText := "this is an error"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		clusterInfo, err := clusterRegisterUtil.CreateOrUpdateCluster(host, cloudId, clusterId, connectorId, connectorInstall, http.MethodPost, apiToken)

		assert.Equal(t, "", clusterInfo.ID)
		assert.Equal(t, "", clusterInfo.Name)
		assert.EqualError(t, err, "error creating cluster: error on request post clusters: this is an error")
	})

	t.Run("TestCreateOrUpdateCluster__ReturnsClusterInfoWhenClusterGetsCreated", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"id":"1234","name":"test-cluster","managedState":"unmanaged"}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       ret,
		}, nil).Once()

		clusterInfo, err := clusterRegisterUtil.CreateOrUpdateCluster(host, cloudId, clusterId, connectorId, connectorInstall, http.MethodPost, apiToken)

		assert.Equal(t, "1234", clusterInfo.ID)
		assert.Equal(t, "test-cluster", clusterInfo.Name)
		assert.Equal(t, "unmanaged", clusterInfo.ManagedState)
		assert.Nil(t, err)
	})

	t.Run("TestCreateOrUpdateCluster__ReturnsErrorWhenUpdateClusterFails", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		errorText := "this is an error"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		clusterInfo, err := clusterRegisterUtil.CreateOrUpdateCluster(host, cloudId, clusterId, connectorId, connectorInstall, http.MethodPut, apiToken)

		assert.Equal(t, "", clusterInfo.ID)
		assert.Equal(t, "", clusterInfo.Name)
		assert.EqualError(t, err, "error updating cluster: error on request put clusters: this is an error")
	})

	t.Run("TestCreateOrUpdateCluster__ReturnsClusterInfoWhenClusterGetsUpdated", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       nil,
		}, nil).Once()

		clusterInfo, err := clusterRegisterUtil.CreateOrUpdateCluster(host, cloudId, clusterId, connectorId, connectorInstall, http.MethodPut, apiToken)

		assert.Equal(t, "test_clusterId", clusterInfo.ID)
		assert.Equal(t, "", clusterInfo.Name)
		assert.Equal(t, "", clusterInfo.ManagedState)
		assert.Nil(t, err)
	})
}

func TestGetStorageClass(t *testing.T) {
	host, cloudId, clusterId, apiToken := "test_host", "test_cloudId", "test_clusterId", "test_apiToken"

	t.Run("TestGetStorageClass__HTTPGetRequestFailsReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		errorText := "error on get request"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		storageClassId, err := clusterRegisterUtil.GetStorageClass(host, cloudId, clusterId, apiToken)

		assert.Equal(t, "", storageClassId)
		assert.EqualError(t, err, "error on request get storage classes: error on get request")
	})

	t.Run("TestGetStorageClass__HTTPGetRequestInvalidStatusCodeReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 401,
			Body:       nil,
		}, nil).Once()

		storageClassId, err := clusterRegisterUtil.GetStorageClass(host, cloudId, clusterId, apiToken)

		assert.Equal(t, "", storageClassId)
		assert.EqualError(t, err, "get storage classes failed 401")
	})

	t.Run("TestGetStorageClass__ReadResponseBodyErrorReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		mockRead := mockRead{}
		mockRead.On("Read", mock.Anything).Return(0, errors.New("error reading"))
		mockRead.On("Close").Return(errors.New("error closing"))

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       &mockRead,
		}, nil).Once()

		storageClassId, err := clusterRegisterUtil.GetStorageClass(host, cloudId, clusterId, apiToken)

		assert.Equal(t, "", storageClassId)
		assert.EqualError(t, err, "error reading response from get storage classes: error reading")
	})

	t.Run("TestGetStorageClass__UnmarshalBodyErrorReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`items:{"Name":"Joe","Body":"Hello","Time":1294706395881547069`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		storageClassId, err := clusterRegisterUtil.GetStorageClass(host, cloudId, clusterId, apiToken)

		assert.Equal(t, "", storageClassId)
		assert.EqualError(t, err, "unmarshall error when getting storage classes: invalid character 'i' looking for beginning of value")
	})

	t.Run("TestGetStorageClass__StorageClassSpecifiedInCRSpecAndIsValidReturnStorageClassId", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{storageClassName: true})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"items":[{"id":"1234","name":"test_sc1234"}, {"id":"5678","name":"test-sc"}]}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		storageClassId, err := clusterRegisterUtil.GetStorageClass(host, cloudId, clusterId, apiToken)

		assert.Equal(t, "5678", storageClassId)
		assert.Nil(t, err)
	})

	t.Run("TestGetStorageClass__StorageClassSpecifiedInCRSpecAndIsInvalidAndNoDefaultSCReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{storageClassName: true})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"items":[{"id":"1234","name":"test_sc1234"}, {"id":"5678","name":"test-sc1"}]}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		storageClassId, err := clusterRegisterUtil.GetStorageClass(host, cloudId, clusterId, apiToken)

		assert.Equal(t, "", storageClassId)
		assert.EqualError(t, err, "no default storage class in the system")
	})

	t.Run("TestGetStorageClass__StorageClassNotSpecifiedInCRSpecAndDefaultSCPresentReturnStorageClassId", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"items":[{"id":"1234","name":"test_sc1234"}, {"id":"5678","name":"test-sc1","isDefault":"true"}]}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		storageClassId, err := clusterRegisterUtil.GetStorageClass(host, cloudId, clusterId, apiToken)

		assert.Equal(t, "5678", storageClassId)
		assert.Nil(t, err)
	})

	t.Run("TestGetStorageClass__StorageClassNotSpecifiedInCRSpecAndNoDefaultSCReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"items":[{"id":"1234","name":"test_sc1234"}, {"id":"5678","name":"test-sc1"}]}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		storageClassId, err := clusterRegisterUtil.GetStorageClass(host, cloudId, clusterId, apiToken)

		assert.Equal(t, "", storageClassId)
		assert.EqualError(t, err, "no default storage class in the system")
	})
}

func TestUpdateManagedCluster(t *testing.T) {
	host, clusterId, connectorId, apiToken, connectorInstall := "test_host", "test_clusterId", "test_connectorId", "test_apiToken", "installed"

	t.Run("TestUpdateManagedCluster__HTTPPutRequestFailsReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		errorText := "error on put request update"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		err := clusterRegisterUtil.UpdateManagedCluster(host, clusterId, connectorId, connectorInstall, apiToken)
		assert.EqualError(t, err, "error on request put manage clusters: error on put request update")
	})

	t.Run("TestUpdateManagedCluster__HTTPPutRequestInvalidStatusCodeReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 400,
			Body:       nil,
		}, nil).Once()

		err := clusterRegisterUtil.UpdateManagedCluster(host, clusterId, connectorId, connectorInstall, apiToken)
		assert.EqualError(t, err, "manage cluster failed with: 400")
	})

	t.Run("TestUpdateManagedCluster__ClusterUpdatedReturnNil", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       nil,
		}, nil).Once()

		err := clusterRegisterUtil.UpdateManagedCluster(host, clusterId, connectorId, connectorInstall, apiToken)
		assert.Nil(t, err)
	})
}

func TestCreateManagedCluster(t *testing.T) {
	host, cloudId, clusterId, storageClass, apiToken, connectorInstalled := "test_host", "test_cloudId", "test_clusterId", "test_sc", "test_apiToken", "installed"

	t.Run("TestCreateManagedCluster__HTTPPostRequestFailsReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		errorText := "error on post request create"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		err := clusterRegisterUtil.CreateManagedCluster(host, cloudId, clusterId, storageClass, connectorInstalled, apiToken)
		assert.EqualError(t, err, "error on request post manage clusters: error on post request create")
	})

	t.Run("TestCreateManagedCluster__HTTPPostRequestInvalidStatusCodeReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 400,
			Body:       nil,
		}, nil).Once()

		err := clusterRegisterUtil.CreateManagedCluster(host, cloudId, clusterId, storageClass, connectorInstalled, apiToken)
		assert.EqualError(t, err, "manage cluster failed with: 400")
	})

	t.Run("TestCreateManagedCluster__ReadResponseBodyErrorReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		mockRead := mockRead{}
		mockRead.On("Read", mock.Anything).Return(0, errors.New("error reading"))
		mockRead.On("Close").Return(errors.New("error closing"))

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       &mockRead,
		}, nil).Once()

		err := clusterRegisterUtil.CreateManagedCluster(host, cloudId, clusterId, storageClass, connectorInstalled, apiToken)
		assert.EqualError(t, err, "error reading response from post manage clusters: error reading")
	})

	t.Run("TestCreateManagedCluster__UnmarshalBodyErrorReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`items:{"Name":"Joe","Body":"Hello","Time":1294706395881547069`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       ret,
		}, nil).Once()

		err := clusterRegisterUtil.CreateManagedCluster(host, cloudId, clusterId, storageClass, connectorInstalled, apiToken)
		assert.ErrorContains(t, err, "unmarshall error when parsing post manage clusters response")
	})

	t.Run("TestCreateManagedCluster__ClusterManagedReturnNil", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"id":"1234","name":"test-cluster","managedState":"managed"}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       ret,
		}, nil).Once()

		err := clusterRegisterUtil.CreateManagedCluster(host, cloudId, clusterId, storageClass, connectorInstalled, apiToken)
		assert.Nil(t, err)
	})
}

func TestCreateOrUpdateManagedCluster(t *testing.T) {
	host, cloudId, clusterId, connectorId, apiToken := "test_host", "test_cloudId", "test_clusterId", "test_connectorId", "test_apiToken"

	t.Run("TestCreateOrUpdateManagedCluster__ReturnsErrorWhenUpdateManagedClusterFails", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		errorText := "this is an error"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		clusterInfo, err := clusterRegisterUtil.CreateOrUpdateManagedCluster(host, cloudId, clusterId, connectorId, http.MethodPut, apiToken)

		assert.Equal(t, "", clusterInfo.ID)
		assert.Equal(t, "", clusterInfo.Name)
		assert.EqualError(t, err, "error updating managed cluster: error on request put manage clusters: this is an error")
	})

	t.Run("TestCreateOrUpdateManagedCluster__ReturnsClusterInfoWhenManagedClusterGetsUpdated", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       nil,
		}, nil).Once()

		clusterInfo, err := clusterRegisterUtil.CreateOrUpdateManagedCluster(host, cloudId, clusterId, connectorId, http.MethodPut, apiToken)

		assert.Equal(t, "test_clusterId", clusterInfo.ID)
		assert.Equal(t, "", clusterInfo.Name)
		assert.Equal(t, "managed", clusterInfo.ManagedState)
		assert.Nil(t, err)
	})

	t.Run("TestCreateOrUpdateManagedCluster__ReturnsErrorWhenCreateManagedClusterFails", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		errorText := "this is an error"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		clusterInfo, err := clusterRegisterUtil.CreateOrUpdateManagedCluster(host, cloudId, clusterId, connectorId, http.MethodPost, apiToken)

		assert.Equal(t, "", clusterInfo.ID)
		assert.Equal(t, "", clusterInfo.Name)
		assert.EqualError(t, err, "error creating managed cluster: error on request post manage clusters: this is an error")
	})

	t.Run("TestCreateOrUpdateManagedCluster__ReturnsClusterInfoWhenClusterGetsManaged", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"id":"test_cluster","name":"test-cluster","managedState":"managed"}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       ret,
		}, nil).Once()

		clusterInfo, err := clusterRegisterUtil.CreateOrUpdateManagedCluster(host, cloudId, clusterId, connectorId, http.MethodPost, apiToken)

		assert.Equal(t, "test_clusterId", clusterInfo.ID)
		assert.Equal(t, "", clusterInfo.Name)
		assert.Equal(t, "managed", clusterInfo.ManagedState)
		assert.Nil(t, err)
	})
}

func TestValidateAndGetCluster(t *testing.T) {
	host, cloudId, apiToken := "test_host", "test_cloudId", "test_apiToken"

	t.Run("TestValidateAndGetCluster__GetClusterReturnsErrorReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{cloudId: true, clusterId: true})

		errorText := "error on get request"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		clusterInfo, err := clusterRegisterUtil.ValidateAndGetCluster(host, cloudId, apiToken, nil)

		assert.Equal(t, "", clusterInfo.ID)
		assert.Equal(t, "", clusterInfo.Name)
		assert.EqualError(t, err, "error on get cluster: error on request get clusters: error on get request")
	})

	t.Run("TestValidateAndGetCluster__GetClusterReturnsEmptyClusterInfoReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{cloudId: true, clusterId: true})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"id":"","name":"this is a cluster"}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		clusterInfo, err := clusterRegisterUtil.ValidateAndGetCluster(host, cloudId, apiToken, nil)

		assert.Equal(t, "", clusterInfo.ID)
		assert.Equal(t, "", clusterInfo.Name)
		assert.EqualError(t, err, "Invalid ClusterId provided in the Spec : 1234")
	})

	t.Run("TestValidateAndGetCluster__ValidClusterIdProvidedInTheSpecReturnCloudId", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{cloudId: true, clusterId: true})

		ret := io.NopCloser(bytes.NewReader([]byte(`{"id":"1234","name":"this is a cluster"}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		clusterInfo, err := clusterRegisterUtil.ValidateAndGetCluster(host, cloudId, apiToken, nil)

		assert.Equal(t, "1234", clusterInfo.ID)
		assert.Equal(t, "this is a cluster", clusterInfo.Name)
		assert.Nil(t, err)
	})

	t.Run("TestValidateAndGetCluster__GetDefaultServiceFailsReturnError", func(t *testing.T) {
		clusterRegisterUtil, _, _, _ := createClusterRegister(AstraConnectorInput{})

		clusterInfo, err := clusterRegisterUtil.ValidateAndGetCluster(host, cloudId, apiToken, nil)

		assert.Equal(t, "", clusterInfo.ID)
		assert.Equal(t, "", clusterInfo.Name)
		assert.EqualError(t, err, "services \"kubernetes\" not found")
	})

	t.Run("TestValidateAndGetCluster__GetClustersFailReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, fakeClient := createClusterRegister(AstraConnectorInput{})

		service := &coreV1.Service{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      "kubernetes",
				Namespace: "default",
				UID:       "svc-uid",
			},
		}

		// creating secret
		err := fakeClient.Create(ctx, service)
		assert.NoError(t, err)

		errorText := "error on get request"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		clusterInfo, err := clusterRegisterUtil.ValidateAndGetCluster(host, cloudId, apiToken, nil)

		assert.Equal(t, "", clusterInfo.ID)
		assert.Equal(t, "", clusterInfo.Name)
		assert.EqualError(t, err, "error on get clusters: error on request get clusters: error on get request")
	})

	t.Run("TestValidateAndGetCluster__ClusterWithMatchingUUIDFoundReturnClusterInfo", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, fakeClient := createClusterRegister(AstraConnectorInput{})

		service := &coreV1.Service{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      "kubernetes",
				Namespace: "default",
				UID:       "svc-uid",
			},
		}

		// creating secret
		err := fakeClient.Create(ctx, service)
		assert.NoError(t, err)

		ret := io.NopCloser(bytes.NewReader([]byte(`{"items":[{"id":"1234","name":"cluster1", "apiServiceID":"svc-uid"}, {"id":"5678","name":"cluster2"}]}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		clusterInfo, err := clusterRegisterUtil.ValidateAndGetCluster(host, cloudId, apiToken, nil)

		assert.Equal(t, "1234", clusterInfo.ID)
		assert.Equal(t, "cluster1", clusterInfo.Name)
		assert.Nil(t, err)
	})

	t.Run("TestValidateAndGetCluster__ClusterWithMatchingUUIDNotFoundReturnEmptyClusterInfo", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, fakeClient := createClusterRegister(AstraConnectorInput{})

		service := &coreV1.Service{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      "kubernetes",
				Namespace: "default",
				UID:       "svc-uid",
			},
		}

		// creating secret
		err := fakeClient.Create(ctx, service)
		assert.NoError(t, err)

		ret := io.NopCloser(bytes.NewReader([]byte(`{"items":[{"id":"1234","name":"cluster1", "apiServiceID":"svc-uid11"}, {"id":"5678","name":"cluster2"}]}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		clusterInfo, err := clusterRegisterUtil.ValidateAndGetCluster(host, cloudId, apiToken, nil)

		assert.Equal(t, "", clusterInfo.ID)
		assert.Equal(t, "", clusterInfo.Name)
		assert.Nil(t, err)
	})
}

func TestGetAPITokenFromSecret(t *testing.T) {
	t.Run("GetAPITokenFromSecret__SecretNotPresentReturnsError", func(t *testing.T) {
		clusterRegisterUtil, _, _, _ := createClusterRegister(AstraConnectorInput{})
		apiToken, err := clusterRegisterUtil.GetAPITokenFromSecret("astra-token")

		assert.Equal(t, apiToken, "")
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

		apiToken, err := clusterRegisterUtil.GetAPITokenFromSecret(apiTokenSecret)

		assert.Equal(t, apiToken, "")
		assert.EqualError(t, err, "failed to extract apiToken key from secret")
	})

	t.Run("GetAPITokenFromSecret__ReturnsApiToken", func(t *testing.T) {
		clusterRegisterUtil, _, apiTokenSecret, _ := createClusterRegister(AstraConnectorInput{createTokenSecret: true})

		apiToken, err := clusterRegisterUtil.GetAPITokenFromSecret(apiTokenSecret)
		assert.Equal(t, apiToken, "auth-token")
		assert.NoError(t, err)
	})
}

func TestRegisterClusterWithAstra(t *testing.T) {
	connectorId := "test_connectorId"

	t.Run("TestRegisterClusterWithAstra__SetHttpClientFailsReturnError", func(t *testing.T) {
		clusterRegisterUtil, _, _, _ := createClusterRegister(AstraConnectorInput{invalidHostDetails: true})

		err := clusterRegisterUtil.RegisterClusterWithAstra(connectorId, nil)
		assert.EqualError(t, err, "invalid cloudBridgeURL provided: test_url, format - https://hostname")
	})

	t.Run("TestRegisterClusterWithAstra__GetAPITokenFromSecretFailsReturnError", func(t *testing.T) {
		clusterRegisterUtil, _, _, _ := createClusterRegister(AstraConnectorInput{})

		err := clusterRegisterUtil.RegisterClusterWithAstra(connectorId, nil)
		assert.EqualError(t, err, "secrets \"astra-token\" not found")
	})

	t.Run("TestRegisterClusterWithAstra__GetOrCreateCloudFailsReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{createTokenSecret: true, cloudId: true})

		// For GetOrCreateCloud call
		errorText := "error on get or create cloud"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		err := clusterRegisterUtil.RegisterClusterWithAstra(connectorId, nil)
		assert.EqualError(t, err, "Invalid CloudId provided in the Spec : 9876")
	})

	t.Run("TestRegisterClusterWithAstra__ValidateAndGetClusterFailsReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, _ := createClusterRegister(AstraConnectorInput{createTokenSecret: true, cloudId: true})

		// For GetOrCreateCloud call
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       nil,
		}, nil).Once()

		// For ValidateAndGetCluster call
		errorText := "error on validate and get cluster"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		err := clusterRegisterUtil.RegisterClusterWithAstra(connectorId, nil)
		assert.EqualError(t, err, "services \"kubernetes\" not found")
	})

	t.Run("TestRegisterClusterWithAstra__CreateOrUpdateClusterFailsReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, fakeClient := createClusterRegister(AstraConnectorInput{createTokenSecret: true, cloudId: true})

		// For GetOrCreateCloud call
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       nil,
		}, nil).Once()

		// For ValidateAndGetCluster call
		service := &coreV1.Service{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      "kubernetes",
				Namespace: "default",
				UID:       "svc-uid",
			},
		}

		err := fakeClient.Create(ctx, service)
		assert.NoError(t, err)

		ret := io.NopCloser(bytes.NewReader([]byte(`{"items":[{"id":"1234","name":"cluster1", "apiServiceID":"svc-uid", "managedState":"unmanaged"}, {"id":"5678","name":"cluster2"}]}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		// For CreateOrUpdateCluster call
		errorText := "error on create or update cluster"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		err = clusterRegisterUtil.RegisterClusterWithAstra(connectorId, nil)
		assert.EqualError(t, err, "error updating cluster: error on request put clusters: error on create or update cluster")
	})

	t.Run("TestRegisterClusterWithAstra__CreateOrUpdateManagedClusterFailsReturnError", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, fakeClient := createClusterRegister(AstraConnectorInput{createTokenSecret: true, cloudId: true})

		// For GetOrCreateCloud call
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       nil,
		}, nil).Once()

		// For ValidateAndGetCluster call
		service := &coreV1.Service{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      "kubernetes",
				Namespace: "default",
				UID:       "svc-uid",
			},
		}

		err := fakeClient.Create(ctx, service)
		assert.NoError(t, err)

		ret := io.NopCloser(bytes.NewReader([]byte(`{"items":[{"id":"1234","name":"cluster1", "apiServiceID":"svc-uid", "managedState":"unmanaged"}, {"id":"5678","name":"cluster2"}]}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		// For CreateOrUpdateCluster call
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       nil,
		}, nil).Once()

		errorText := "this is an error"
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{}, errors.New(errorText))

		err = clusterRegisterUtil.RegisterClusterWithAstra(connectorId, nil)
		assert.EqualError(t, err, "error creating managed cluster: error on request post manage clusters: this is an error")
	})

	t.Run("TestRegisterClusterWithAstra__EverythingWorksReturnNil", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, fakeClient := createClusterRegister(AstraConnectorInput{createTokenSecret: true, cloudId: true})

		// For GetOrCreateCloud call
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       nil,
		}, nil).Once()

		// For ValidateAndGetCluster call
		service := &coreV1.Service{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      "kubernetes",
				Namespace: "default",
				UID:       "svc-uid",
			},
		}

		err := fakeClient.Create(ctx, service)
		assert.NoError(t, err)

		ret := io.NopCloser(bytes.NewReader([]byte(`{"items":[{"id":"1234","name":"cluster1", "apiServiceID":"svc-uid", "managedState":"unmanaged"}, {"id":"5678","name":"cluster2"}]}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		// For CreateOrUpdateCluster call
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       nil,
		}, nil).Once()

		ret = io.NopCloser(bytes.NewReader([]byte(`{"id":"test_cluster","name":"test-cluster","managedState":"managed"}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       ret,
		}, nil).Once()

		err = clusterRegisterUtil.RegisterClusterWithAstra(connectorId, nil)
		assert.Nil(t, err)
	})

	t.Run("TestRegisterClusterWithAstra__EverythingWorksWhenExistingClusterInManagedStateReturnNil", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, fakeClient := createClusterRegister(AstraConnectorInput{createTokenSecret: true, cloudId: true})

		// For GetOrCreateCloud call
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       nil,
		}, nil).Once()

		// For ValidateAndGetCluster call
		service := &coreV1.Service{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      "kubernetes",
				Namespace: "default",
				UID:       "svc-uid",
			},
		}

		err := fakeClient.Create(ctx, service)
		assert.NoError(t, err)

		ret := io.NopCloser(bytes.NewReader([]byte(`{"items":[{"id":"1234","name":"cluster1", "apiServiceID":"svc-uid", "managedState":"managed"}, {"id":"5678","name":"cluster2"}]}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		// For CreateOrUpdateCluster call, no call will be made

		// For CreateOrUpdateManagedCluster call
		ret = io.NopCloser(bytes.NewReader([]byte(`{"items":[{"id":"1234","name":"test_sc1234"}, {"id":"5678","name":"test-sc1","isDefault":"true"}]}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		ret = io.NopCloser(bytes.NewReader([]byte(`{"id":"test_cluster","name":"test-cluster","managedState":"managed"}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       ret,
		}, nil).Once()

		err = clusterRegisterUtil.RegisterClusterWithAstra(connectorId, nil)
		assert.Nil(t, err)
	})

	t.Run("TestRegisterClusterWithAstra__EverythingWorksWhenNoExistingClusterReturnNil", func(t *testing.T) {
		clusterRegisterUtil, mockHttpClient, _, fakeClient := createClusterRegister(AstraConnectorInput{createTokenSecret: true, cloudId: true})

		// For GetOrCreateCloud call
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       nil,
		}, nil).Once()

		// For ValidateAndGetCluster call
		service := &coreV1.Service{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      "kubernetes",
				Namespace: "default",
				UID:       "svc-uid",
			},
		}

		err := fakeClient.Create(ctx, service)
		assert.NoError(t, err)

		ret := io.NopCloser(bytes.NewReader([]byte(`{"items":[{"id":"1234","name":"cluster1", "apiServiceID":"svc-uid12", "managedState":"managed"}, {"id":"5678","name":"cluster2"}]}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       ret,
		}, nil).Once()

		// For CreateOrUpdateCluster call, post call will be made
		ret = io.NopCloser(bytes.NewReader([]byte(`{"id":"1234","name":"test-cluster","managedState":"unmanaged"}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       ret,
		}, nil).Once()

		ret = io.NopCloser(bytes.NewReader([]byte(`{"id":"test_cluster","name":"test-cluster","managedState":"managed"}`)))
		mockHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 201,
			Body:       ret,
		}, nil).Once()

		err = clusterRegisterUtil.RegisterClusterWithAstra(connectorId, nil)
		assert.Nil(t, err)
	})
}
