/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package deployer

import (
	"context"

	rbacv1 "k8s.io/api/rbac/v1"

	v1 "github.com/NetApp/astraagent-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

type Deployer interface {
	GetDeploymentObject(m *v1.AstraAgent, ctx context.Context) (*appsv1.Deployment, error)
	GetStatefulsetObject(m *v1.AstraAgent, ctx context.Context) (*appsv1.StatefulSet, error)
	GetServiceObject(m *v1.AstraAgent, serviceName string) (*corev1.Service, error)
	GetConfigMapObject(m *v1.AstraAgent) (*corev1.ConfigMap, error)
	GetServiceAccountObject(m *v1.AstraAgent) (*corev1.ServiceAccount, error)
	GetRoleObject(m *v1.AstraAgent) (*rbacv1.Role, error)
	GetRoleBindingObject(m *v1.AstraAgent) (*rbacv1.RoleBinding, error)
}
