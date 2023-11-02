// Code generated by mockery v2.19.0. DO NOT EDIT.

package mocks

import (
	context "context"

	client "sigs.k8s.io/controller-runtime/pkg/client"

	mock "github.com/stretchr/testify/mock"

	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

// Deployer is an autogenerated mock type for the Deployer type
type Deployer struct {
	mock.Mock
}

// GetClusterRoleBindingObjects provides a mock function with given fields: m, ctx
func (_m *Deployer) GetClusterRoleBindingObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	ret := _m.Called(m, ctx)

	var r0 []client.Object
	if rf, ok := ret.Get(0).(func(*v1.AstraConnector, context.Context) []client.Object); ok {
		r0 = rf(m, ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]client.Object)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*v1.AstraConnector, context.Context) error); ok {
		r1 = rf(m, ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetClusterRoleObjects provides a mock function with given fields: m, ctx
func (_m *Deployer) GetClusterRoleObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	ret := _m.Called(m, ctx)

	var r0 []client.Object
	if rf, ok := ret.Get(0).(func(*v1.AstraConnector, context.Context) []client.Object); ok {
		r0 = rf(m, ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]client.Object)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*v1.AstraConnector, context.Context) error); ok {
		r1 = rf(m, ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetConfigMapObjects provides a mock function with given fields: m, ctx
func (_m *Deployer) GetConfigMapObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	ret := _m.Called(m, ctx)

	var r0 []client.Object
	if rf, ok := ret.Get(0).(func(*v1.AstraConnector, context.Context) []client.Object); ok {
		r0 = rf(m, ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]client.Object)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*v1.AstraConnector, context.Context) error); ok {
		r1 = rf(m, ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetDeploymentObjects provides a mock function with given fields: m, ctx
func (_m *Deployer) GetDeploymentObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	ret := _m.Called(m, ctx)

	var r0 []client.Object
	if rf, ok := ret.Get(0).(func(*v1.AstraConnector, context.Context) []client.Object); ok {
		r0 = rf(m, ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]client.Object)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*v1.AstraConnector, context.Context) error); ok {
		r1 = rf(m, ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetRoleBindingObjects provides a mock function with given fields: m, ctx
func (_m *Deployer) GetRoleBindingObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	ret := _m.Called(m, ctx)

	var r0 []client.Object
	if rf, ok := ret.Get(0).(func(*v1.AstraConnector, context.Context) []client.Object); ok {
		r0 = rf(m, ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]client.Object)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*v1.AstraConnector, context.Context) error); ok {
		r1 = rf(m, ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetRoleObjects provides a mock function with given fields: m, ctx
func (_m *Deployer) GetRoleObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	ret := _m.Called(m, ctx)

	var r0 []client.Object
	if rf, ok := ret.Get(0).(func(*v1.AstraConnector, context.Context) []client.Object); ok {
		r0 = rf(m, ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]client.Object)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*v1.AstraConnector, context.Context) error); ok {
		r1 = rf(m, ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetServiceAccountObjects provides a mock function with given fields: m, ctx
func (_m *Deployer) GetServiceAccountObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	ret := _m.Called(m, ctx)

	var r0 []client.Object
	if rf, ok := ret.Get(0).(func(*v1.AstraConnector, context.Context) []client.Object); ok {
		r0 = rf(m, ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]client.Object)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*v1.AstraConnector, context.Context) error); ok {
		r1 = rf(m, ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetServiceObjects provides a mock function with given fields: m, ctx
func (_m *Deployer) GetServiceObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	ret := _m.Called(m, ctx)

	var r0 []client.Object
	if rf, ok := ret.Get(0).(func(*v1.AstraConnector, context.Context) []client.Object); ok {
		r0 = rf(m, ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]client.Object)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*v1.AstraConnector, context.Context) error); ok {
		r1 = rf(m, ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetStatefulSetObjects provides a mock function with given fields: m, ctx
func (_m *Deployer) GetStatefulSetObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	ret := _m.Called(m, ctx)

	var r0 []client.Object
	if rf, ok := ret.Get(0).(func(*v1.AstraConnector, context.Context) []client.Object); ok {
		r0 = rf(m, ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]client.Object)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*v1.AstraConnector, context.Context) error); ok {
		r1 = rf(m, ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewDeployer interface {
	mock.TestingT
	Cleanup(func())
}

// NewDeployer creates a new instance of Deployer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewDeployer(t mockConstructorTestingTNewDeployer) *Deployer {
	mock := &Deployer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}