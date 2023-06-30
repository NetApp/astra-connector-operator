/*
 * Copyright (c) 2023. NetApp, Inc. All Rights Reserved.
 */

package model

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

type Deployer interface {
	GetDeploymentObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error)
	GetStatefulSetObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error)
	GetServiceObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error)
	GetConfigMapObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error)
	GetServiceAccountObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error)
	GetRoleObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error)
	GetClusterRoleObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error)
	GetRoleBindingObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error)
	GetClusterRoleBindingObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error)
}
