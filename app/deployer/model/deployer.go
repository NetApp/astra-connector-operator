/*
 * Copyright (c) 2023. NetApp, Inc. All Rights Reserved.
 */

package model

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

type Deployer interface {
	GetDeploymentObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error)
	GetStatefulSetObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error)
	GetServiceObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error)
	GetConfigMapObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error)
	GetServiceAccountObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error)
	GetRoleObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error)
	GetClusterRoleObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error)
	GetRoleBindingObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error)
	GetClusterRoleBindingObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error)
}

// Define the MutateFn function
func NonMutateFn() error {
	// TODO https://jira.ngage.netapp.com/browse/ASTRACTL-27555
	// Apply any desired changes to the deployment object here
	// For example, you can update the environment variables, container image, etc.
	// want to remove duplicated code like each deployer setting image and secret can be do once here
	return nil
}
