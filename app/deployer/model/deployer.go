/*
 * Copyright (c) 2023. NetApp, Inc. All Rights Reserved.
 */

package model

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Deployer interface {
	GetDeploymentObjects(ctx context.Context) ([]client.Object, controllerutil.MutateFn, error)
	GetStatefulSetObjects(ctx context.Context) ([]client.Object, controllerutil.MutateFn, error)
	GetServiceObjects(ctx context.Context) ([]client.Object, controllerutil.MutateFn, error)
	GetConfigMapObjects(ctx context.Context) ([]client.Object, controllerutil.MutateFn, error)
	GetServiceAccountObjects(ctx context.Context) ([]client.Object, controllerutil.MutateFn, error)
	GetRoleObjects(ctx context.Context) ([]client.Object, controllerutil.MutateFn, error)
	GetClusterRoleObjects(ctx context.Context) ([]client.Object, controllerutil.MutateFn, error)
	GetRoleBindingObjects(ctx context.Context) ([]client.Object, controllerutil.MutateFn, error)
	GetClusterRoleBindingObjects(ctx context.Context) ([]client.Object, controllerutil.MutateFn, error)

	// UpdateStatus of CR
	UpdateStatus(ctx context.Context, status string, statusWriter client.StatusWriter) error
	IsSpecModified(ctx context.Context, k8sClient client.Client) bool
}

// Define the MutateFn function
func NonMutateFn() error {
	// TODO https://jira.ngage.netapp.com/browse/ASTRACTL-27555
	// Apply any desired changes to the deployment object here
	// For example, you can update the environment variables, container image, etc.
	// want to remove duplicated code like each deployer setting image and secret can be do once here
	return nil
}
