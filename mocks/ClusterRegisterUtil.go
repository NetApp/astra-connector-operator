// Code generated by mockery v2.19.0. DO NOT EDIT.

package mocks

import (
	http "net/http"

	register "github.com/NetApp-Polaris/astra-connector-operator/app/register"
	mock "github.com/stretchr/testify/mock"

	time "time"
)

// ClusterRegisterUtil is an autogenerated mock type for the ClusterRegisterUtil type
type ClusterRegisterUtil struct {
	mock.Mock
}

// CloudExists provides a mock function with given fields: astraHost, cloudID, apiToken
func (_m *ClusterRegisterUtil) CloudExists(astraHost string, cloudID string, apiToken string) bool {
	ret := _m.Called(astraHost, cloudID, apiToken)

	var r0 bool
	if rf, ok := ret.Get(0).(func(string, string, string) bool); ok {
		r0 = rf(astraHost, cloudID, apiToken)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// CreateCloud provides a mock function with given fields: astraHost, cloudType, apiToken
func (_m *ClusterRegisterUtil) CreateCloud(astraHost string, cloudType string, apiToken string) (string, error) {
	ret := _m.Called(astraHost, cloudType, apiToken)

	var r0 string
	if rf, ok := ret.Get(0).(func(string, string, string) string); ok {
		r0 = rf(astraHost, cloudType, apiToken)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string) error); ok {
		r1 = rf(astraHost, cloudType, apiToken)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateCluster provides a mock function with given fields: astraHost, cloudId, astraConnectorId, apiToken
func (_m *ClusterRegisterUtil) CreateCluster(astraHost string, cloudId string, astraConnectorId string, apiToken string) (register.ClusterInfo, error) {
	ret := _m.Called(astraHost, cloudId, astraConnectorId, apiToken)

	var r0 register.ClusterInfo
	if rf, ok := ret.Get(0).(func(string, string, string, string) register.ClusterInfo); ok {
		r0 = rf(astraHost, cloudId, astraConnectorId, apiToken)
	} else {
		r0 = ret.Get(0).(register.ClusterInfo)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string, string) error); ok {
		r1 = rf(astraHost, cloudId, astraConnectorId, apiToken)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateManagedCluster provides a mock function with given fields: astraHost, cloudId, clusterID, storageClass, apiToken
func (_m *ClusterRegisterUtil) CreateManagedCluster(astraHost string, cloudId string, clusterID string, storageClass string, apiToken string) error {
	ret := _m.Called(astraHost, cloudId, clusterID, storageClass, apiToken)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, string, string, string) error); ok {
		r0 = rf(astraHost, cloudId, clusterID, storageClass, apiToken)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateOrUpdateCluster provides a mock function with given fields: astraHost, cloudId, clusterId, astraConnectorId, clustersMethod, apiToken
func (_m *ClusterRegisterUtil) CreateOrUpdateCluster(astraHost string, cloudId string, clusterId string, astraConnectorId string, clustersMethod string, apiToken string) (register.ClusterInfo, error) {
	ret := _m.Called(astraHost, cloudId, clusterId, astraConnectorId, clustersMethod, apiToken)

	var r0 register.ClusterInfo
	if rf, ok := ret.Get(0).(func(string, string, string, string, string, string) register.ClusterInfo); ok {
		r0 = rf(astraHost, cloudId, clusterId, astraConnectorId, clustersMethod, apiToken)
	} else {
		r0 = ret.Get(0).(register.ClusterInfo)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string, string, string, string) error); ok {
		r1 = rf(astraHost, cloudId, clusterId, astraConnectorId, clustersMethod, apiToken)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateOrUpdateManagedCluster provides a mock function with given fields: astraHost, cloudId, clusterId, astraConnectorId, managedClustersMethod, apiToken
func (_m *ClusterRegisterUtil) CreateOrUpdateManagedCluster(astraHost string, cloudId string, clusterId string, astraConnectorId string, managedClustersMethod string, apiToken string) (register.ClusterInfo, error) {
	ret := _m.Called(astraHost, cloudId, clusterId, astraConnectorId, managedClustersMethod, apiToken)

	var r0 register.ClusterInfo
	if rf, ok := ret.Get(0).(func(string, string, string, string, string, string) register.ClusterInfo); ok {
		r0 = rf(astraHost, cloudId, clusterId, astraConnectorId, managedClustersMethod, apiToken)
	} else {
		r0 = ret.Get(0).(register.ClusterInfo)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string, string, string, string) error); ok {
		r1 = rf(astraHost, cloudId, clusterId, astraConnectorId, managedClustersMethod, apiToken)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetAPITokenFromSecret provides a mock function with given fields: secretName
func (_m *ClusterRegisterUtil) GetAPITokenFromSecret(secretName string) (string, error) {
	ret := _m.Called(secretName)

	var r0 string
	if rf, ok := ret.Get(0).(func(string) string); ok {
		r0 = rf(secretName)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(secretName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetCloudId provides a mock function with given fields: astraHost, cloudType, apiToken, retryTimeout
func (_m *ClusterRegisterUtil) GetCloudId(astraHost string, cloudType string, apiToken string, retryTimeout ...time.Duration) (string, error) {
	_va := make([]interface{}, len(retryTimeout))
	for _i := range retryTimeout {
		_va[_i] = retryTimeout[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, astraHost, cloudType, apiToken)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 string
	if rf, ok := ret.Get(0).(func(string, string, string, ...time.Duration) string); ok {
		r0 = rf(astraHost, cloudType, apiToken, retryTimeout...)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string, ...time.Duration) error); ok {
		r1 = rf(astraHost, cloudType, apiToken, retryTimeout...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetCluster provides a mock function with given fields: astraHost, cloudId, clusterId, apiToken
func (_m *ClusterRegisterUtil) GetCluster(astraHost string, cloudId string, clusterId string, apiToken string) (register.Cluster, error) {
	ret := _m.Called(astraHost, cloudId, clusterId, apiToken)

	var r0 register.Cluster
	if rf, ok := ret.Get(0).(func(string, string, string, string) register.Cluster); ok {
		r0 = rf(astraHost, cloudId, clusterId, apiToken)
	} else {
		r0 = ret.Get(0).(register.Cluster)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string, string) error); ok {
		r1 = rf(astraHost, cloudId, clusterId, apiToken)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetClusters provides a mock function with given fields: astraHost, cloudId, apiToken
func (_m *ClusterRegisterUtil) GetClusters(astraHost string, cloudId string, apiToken string) (register.GetClustersResponse, error) {
	ret := _m.Called(astraHost, cloudId, apiToken)

	var r0 register.GetClustersResponse
	if rf, ok := ret.Get(0).(func(string, string, string) register.GetClustersResponse); ok {
		r0 = rf(astraHost, cloudId, apiToken)
	} else {
		r0 = ret.Get(0).(register.GetClustersResponse)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string) error); ok {
		r1 = rf(astraHost, cloudId, apiToken)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetConnectorIDFromConfigMap provides a mock function with given fields: cmData
func (_m *ClusterRegisterUtil) GetConnectorIDFromConfigMap(cmData map[string]string) (string, error) {
	ret := _m.Called(cmData)

	var r0 string
	if rf, ok := ret.Get(0).(func(map[string]string) string); ok {
		r0 = rf(cmData)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(map[string]string) error); ok {
		r1 = rf(cmData)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetNatsSyncClientRegistrationURL provides a mock function with given fields:
func (_m *ClusterRegisterUtil) GetNatsSyncClientRegistrationURL() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// GetNatsSyncClientUnregisterURL provides a mock function with given fields:
func (_m *ClusterRegisterUtil) GetNatsSyncClientUnregisterURL() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// GetOrCreateCloud provides a mock function with given fields: astraHost, cloudType, apiToken
func (_m *ClusterRegisterUtil) GetOrCreateCloud(astraHost string, cloudType string, apiToken string) (string, error) {
	ret := _m.Called(astraHost, cloudType, apiToken)

	var r0 string
	if rf, ok := ret.Get(0).(func(string, string, string) string); ok {
		r0 = rf(astraHost, cloudType, apiToken)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string) error); ok {
		r1 = rf(astraHost, cloudType, apiToken)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetStorageClass provides a mock function with given fields: astraHost, cloudId, clusterId, apiToken
func (_m *ClusterRegisterUtil) GetStorageClass(astraHost string, cloudId string, clusterId string, apiToken string) (string, error) {
	ret := _m.Called(astraHost, cloudId, clusterId, apiToken)

	var r0 string
	if rf, ok := ret.Get(0).(func(string, string, string, string) string); ok {
		r0 = rf(astraHost, cloudId, clusterId, apiToken)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string, string) error); ok {
		r1 = rf(astraHost, cloudId, clusterId, apiToken)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListClouds provides a mock function with given fields: astraHost, apiToken
func (_m *ClusterRegisterUtil) ListClouds(astraHost string, apiToken string) (*http.Response, error) {
	ret := _m.Called(astraHost, apiToken)

	var r0 *http.Response
	if rf, ok := ret.Get(0).(func(string, string) *http.Response); ok {
		r0 = rf(astraHost, apiToken)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*http.Response)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string) error); ok {
		r1 = rf(astraHost, apiToken)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RegisterClusterWithAstra provides a mock function with given fields: astraConnectorId
func (_m *ClusterRegisterUtil) RegisterClusterWithAstra(astraConnectorId string) error {
	ret := _m.Called(astraConnectorId)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(astraConnectorId)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RegisterNatsSyncClient provides a mock function with given fields:
func (_m *ClusterRegisterUtil) RegisterNatsSyncClient() (string, error) {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UnRegisterNatsSyncClient provides a mock function with given fields:
func (_m *ClusterRegisterUtil) UnRegisterNatsSyncClient() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateCluster provides a mock function with given fields: astraHost, cloudId, clusterId, astraConnectorId, apiToken
func (_m *ClusterRegisterUtil) UpdateCluster(astraHost string, cloudId string, clusterId string, astraConnectorId string, apiToken string) error {
	ret := _m.Called(astraHost, cloudId, clusterId, astraConnectorId, apiToken)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, string, string, string) error); ok {
		r0 = rf(astraHost, cloudId, clusterId, astraConnectorId, apiToken)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateManagedCluster provides a mock function with given fields: astraHost, clusterId, astraConnectorId, apiToken
func (_m *ClusterRegisterUtil) UpdateManagedCluster(astraHost string, clusterId string, astraConnectorId string, apiToken string) error {
	ret := _m.Called(astraHost, clusterId, astraConnectorId, apiToken)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, string, string) error); ok {
		r0 = rf(astraHost, clusterId, astraConnectorId, apiToken)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ValidateAndGetCluster provides a mock function with given fields: astraHost, cloudId, apiToken
func (_m *ClusterRegisterUtil) ValidateAndGetCluster(astraHost string, cloudId string, apiToken string) (register.ClusterInfo, error) {
	ret := _m.Called(astraHost, cloudId, apiToken)

	var r0 register.ClusterInfo
	if rf, ok := ret.Get(0).(func(string, string, string) register.ClusterInfo); ok {
		r0 = rf(astraHost, cloudId, apiToken)
	} else {
		r0 = ret.Get(0).(register.ClusterInfo)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string, string) error); ok {
		r1 = rf(astraHost, cloudId, apiToken)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewClusterRegisterUtil interface {
	mock.TestingT
	Cleanup(func())
}

// NewClusterRegisterUtil creates a new instance of ClusterRegisterUtil. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewClusterRegisterUtil(t mockConstructorTestingTNewClusterRegisterUtil) *ClusterRegisterUtil {
	mock := &ClusterRegisterUtil{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}