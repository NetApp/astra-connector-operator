package deployer

import (
	"context"

	rbacv1 "k8s.io/api/rbac/v1"

	cachev1 "github.com/NetApp/astraagent-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

type Deployer interface {
	GetDeploymentObject(m *cachev1.AstraAgent, ctx context.Context) (*appsv1.Deployment, error)
	GetStatefulsetObject(m *cachev1.AstraAgent, ctx context.Context) (*appsv1.StatefulSet, error)
	GetServiceObject(m *cachev1.AstraAgent, serviceName string) (*corev1.Service, error)
	GetConfigMapObject(m *cachev1.AstraAgent) (*corev1.ConfigMap, error)
	GetServiceAccountObject(m *cachev1.AstraAgent) (*corev1.ServiceAccount, error)
	GetRoleObject(m *cachev1.AstraAgent) (*rbacv1.Role, error)
	GetRoleBindingObject(m *cachev1.AstraAgent) (*rbacv1.RoleBinding, error)
}
