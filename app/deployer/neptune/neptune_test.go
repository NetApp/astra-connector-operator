package neptune_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/neptune"
	"github.com/NetApp-Polaris/astra-connector-operator/common"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

func createNeptuneDeployer() (neptune.NeptuneClientDeployer, *v1.AstraConnector, context.Context) { // Create a new NeptuneClientDeployer instance
	// Create a new NeptuneClientDeployer instance
	n := neptune.NeptuneClientDeployer{}

	// Create a new AstraConnector instance
	m := v1.AstraConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-astraconnector",
			Namespace: "test-namespace",
		},
		Spec: v1.AstraConnectorSpec{
			ImageRegistry: v1.ImageRegistry{
				Secret: "test-secret",
			},
		},
	}

	// Create a new context
	ctx := context.Background()

	return n, &m, ctx
}

func createNeptuneDeployerV2() (neptune.NeptuneClientDeployerV2, *v1.AstraConnector, context.Context) { // Create a new NeptuneClientDeployer instance
	// Create a new NeptuneClientDeployer instance
	n := neptune.NeptuneClientDeployerV2{}

	// Create a new AstraConnector instance
	m := v1.AstraConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-astraconnector",
			Namespace: "test-namespace",
		},
		Spec: v1.AstraConnectorSpec{
			ImageRegistry: v1.ImageRegistry{
				Secret: "test-secret",
			},
		},
	}

	// Create a new context
	ctx := context.Background()

	return n, &m, ctx
}

func TestGetDeploymentObjects(t *testing.T) {
	n, m, ctx := createNeptuneDeployer()

	// Call the GetDeploymentObjects method
	deploymentObjects, err := n.GetDeploymentObjects(m, ctx)

	// Check if there is no error
	assert.NoError(t, err)

	// Check if the returned deploymentObjects slice has the expected length
	assert.Equal(t, 1, len(deploymentObjects))

	// Check if the returned deploymentObjects contains the expected Deployment object
	deployment, ok := deploymentObjects[0].(*appsv1.Deployment)
	assert.True(t, ok)

	// Check if the Deployment object has the expected properties
	assert.Equal(t, "test-namespace", deployment.Namespace)
	assert.Equal(t, "neptune-controller-manager", deployment.Spec.Template.Spec.ServiceAccountName)

	// Check if the ImagePullSecrets are set correctly
	assert.Equal(t, 1, len(deployment.Spec.Template.Spec.ImagePullSecrets))
	assert.Equal(t, "test-secret", deployment.Spec.Template.Spec.ImagePullSecrets[0].Name)
}

func TestGetDeploymentObjectsV2(t *testing.T) {
	n, m, ctx := createNeptuneDeployerV2()

	// Call the GetDeploymentObjects method
	deploymentObjects, err := n.GetDeploymentObjects(m, ctx)

	// Check if there is no error
	assert.NoError(t, err)

	// Check if the returned deploymentObjects slice has the expected length
	assert.Equal(t, 1, len(deploymentObjects))

	// Check if the returned deploymentObjects contains the expected Deployment object
	deployment, ok := deploymentObjects[0].(*appsv1.Deployment)
	assert.True(t, ok)

	// Check if the Deployment object has the expected properties
	assert.Equal(t, "test-namespace", deployment.Namespace)
	assert.Equal(t, "neptune-controller-manager", deployment.Spec.Template.Spec.ServiceAccountName)

	// Check if the ImagePullSecrets are set correctly
	assert.Equal(t, 1, len(deployment.Spec.Template.Spec.ImagePullSecrets))
	assert.Equal(t, "test-secret", deployment.Spec.Template.Spec.ImagePullSecrets[0].Name)
}

func TestUnimplemenyeObjectsV2(t *testing.T) {
	n, m, ctx := createNeptuneDeployerV2()

	// Call the GetDeploymentObjects method
	ret, err := n.GetStatefulSetObjects(m, ctx)
	assert.Nil(t, ret)
	assert.NoError(t, err)

	ret, err = n.GetServiceObjects(m, ctx)
	assert.Nil(t, ret)
	assert.NoError(t, err)

	ret, err = n.GetConfigMapObjects(m, ctx)
	assert.Nil(t, ret)
	assert.NoError(t, err)

	ret, err = n.GetServiceAccountObjects(m, ctx)
	assert.Nil(t, ret)
	assert.NoError(t, err)

	ret, err = n.GetClusterRoleObjects(m, ctx)
	assert.Nil(t, ret)
	assert.NoError(t, err)

	ret, err = n.GetRoleBindingObjects(m, ctx)
	assert.Nil(t, ret)
	assert.NoError(t, err)

	ret, err = n.GetRoleObjects(m, ctx)
	assert.Nil(t, ret)
	assert.NoError(t, err)

	ret, err = n.GetClusterRoleBindingObjects(m, ctx)
	assert.Nil(t, ret)
	assert.NoError(t, err)

}

func TestGetServiceObjects(t *testing.T) {
	n, m, ctx := createNeptuneDeployer()

	// Call the GetServiceObjects method
	serviceObjects, err := n.GetServiceObjects(m, ctx)

	// Check if there is no error
	assert.NoError(t, err)

	// Check if the returned serviceObjects slice has the expected length
	assert.Equal(t, 1, len(serviceObjects))

	// Check if the returned serviceObjects contains the expected Service object
	service, ok := serviceObjects[0].(*corev1.Service)
	assert.True(t, ok)

	// Check if the Service object has the expected properties
	assert.Equal(t, "test-namespace", service.Namespace)
	assert.Equal(t, "neptune-controller-manager-metrics-service", service.Name)

	// Check if the Service object has the expected port configuration
	assert.Equal(t, 1, len(service.Spec.Ports))
	assert.Equal(t, "https", service.Spec.Ports[0].Name)
	assert.Equal(t, common.NeptuneMetricServicePort, int(service.Spec.Ports[0].Port))
	assert.Equal(t, common.NeptuneMetricServiceProtocol, string(service.Spec.Ports[0].Protocol))

	// Check if the Service object has the expected selector
	assert.Equal(t, map[string]string{"control-plane": "controller-manager"}, service.Spec.Selector)
}

func TestGetServiceAccountObjects(t *testing.T) {
	n, m, ctx := createNeptuneDeployer()

	// Call the GetServiceAccountObjects method
	serviceAccountObjects, err := n.GetServiceAccountObjects(m, ctx)

	// Check if there is no error
	assert.NoError(t, err)

	// Check if the returned serviceAccountObjects slice has the expected length
	assert.Equal(t, 1, len(serviceAccountObjects))

	// Check if the returned serviceAccountObjects contains the expected ServiceAccount object
	serviceAccount, ok := serviceAccountObjects[0].(*corev1.ServiceAccount)
	assert.True(t, ok)

	// Check if the ServiceAccount object has the expected properties
	assert.Equal(t, "test-namespace", serviceAccount.Namespace)
	assert.Equal(t, common.NeptuneName, serviceAccount.Name)
}

func TestGetRoleObjects(t *testing.T) {
	n, m, ctx := createNeptuneDeployer()

	// Call the GetRoleObjects method
	roleObjects, err := n.GetRoleObjects(m, ctx)

	// Check if there is no error
	assert.NoError(t, err)

	// Check if the returned roleObjects slice has the expected length
	assert.Equal(t, 1, len(roleObjects))

	// Check if the returned roleObjects contains the expected Role object
	role, ok := roleObjects[0].(*rbacv1.Role)
	assert.True(t, ok)

	// Check if the Role object has the expected properties
	assert.Equal(t, "test-namespace", role.Namespace)
	assert.Equal(t, common.NeptuneLeaderElectionRoleName, role.Name)

	// Check if the Role object has the expected rules
	expectedRules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
		},
		{
			APIGroups: []string{"coordination.k8s.io"},
			Resources: []string{"leases"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"create", "patch"},
		},
	}
	assert.Equal(t, expectedRules, role.Rules)
}

func TestGetClusterRoleObjects(t *testing.T) {
	n, m, ctx := createNeptuneDeployer()

	// Call the GetClusterRoleObjects method
	clusterRoleObjects, err := n.GetClusterRoleObjects(m, ctx)

	// Check if there is no error
	assert.NoError(t, err)

	// Check if the returned clusterRoleObjects slice has the expected length
	assert.Equal(t, 3, len(clusterRoleObjects))

	// Check if the returned clusterRoleObjects contains the expected ClusterRole objects
	neptuneClusterRole, ok := clusterRoleObjects[0].(*rbacv1.ClusterRole)
	assert.True(t, ok)
	neptuneMetricsReaderCR, ok := clusterRoleObjects[1].(*rbacv1.ClusterRole)
	assert.True(t, ok)
	neptuneProxyCR, ok := clusterRoleObjects[2].(*rbacv1.ClusterRole)
	assert.True(t, ok)

	// Check if the ClusterRole objects have the expected properties
	assert.Equal(t, common.NeptuneClusterRoleName, neptuneClusterRole.Name)
	assert.Equal(t, "neptune-metrics-reader", neptuneMetricsReaderCR.Name)
	assert.Equal(t, "neptune-proxy-role", neptuneProxyCR.Name)

	expectedNeptuneMetricsReaderCRRules := []rbacv1.PolicyRule{
		{
			NonResourceURLs: []string{"/metrics"},
			Verbs:           []string{"get"},
		},
	}

	expectedNeptuneProxyCRRules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"authentication.k8s.io"},
			Resources: []string{"tokenreviews"},
			Verbs:     []string{"create"},
		},
		{
			APIGroups: []string{"authorization.k8s.io"},
			Resources: []string{"subjectaccessreviews"},
			Verbs:     []string{"create"},
		},
	}

	assert.Equal(t, expectedNeptuneMetricsReaderCRRules, neptuneMetricsReaderCR.Rules)
	assert.Equal(t, expectedNeptuneProxyCRRules, neptuneProxyCR.Rules)
}

func TestGetRoleBindingObjects(t *testing.T) {
	n, m, ctx := createNeptuneDeployer()

	// Call the GetRoleBindingObjects method
	roleBindingObjects, err := n.GetRoleBindingObjects(m, ctx)

	// Check if there is no error
	assert.NoError(t, err)

	// Check if the returned roleBindingObjects slice has the expected length
	assert.Equal(t, 1, len(roleBindingObjects))

	// Check if the returned roleBindingObjects contains the expected RoleBinding object
	roleBinding, ok := roleBindingObjects[0].(*rbacv1.RoleBinding)
	assert.True(t, ok)

	// Check if the RoleBinding object has the expected properties
	assert.Equal(t, "test-namespace", roleBinding.Namespace)
	assert.Equal(t, common.NeptuneLeaderElectionRoleBindingName, roleBinding.Name)

	// Check if the RoleBinding object has the expected subjects
	expectedSubjects := []rbacv1.Subject{
		{
			Kind: "ServiceAccount",
			Name: common.NeptuneName,
		},
	}
	assert.Equal(t, expectedSubjects, roleBinding.Subjects)

	// Check if the RoleBinding object has the expected role reference
	expectedRoleRef := rbacv1.RoleRef{
		Kind:     "Role",
		Name:     common.NeptuneLeaderElectionRoleName,
		APIGroup: "rbac.authorization.k8s.io",
	}
	assert.Equal(t, expectedRoleRef, roleBinding.RoleRef)
}

func TestGetClusterRoleBindingObjects(t *testing.T) {
	n, m, ctx := createNeptuneDeployer()

	// Call the GetClusterRoleBindingObjects method
	clusterRoleBindingObjects, err := n.GetClusterRoleBindingObjects(m, ctx)

	// Check if there is no error
	assert.NoError(t, err)

	// Check if the returned clusterRoleBindingObjects slice has the expected length
	assert.Equal(t, 2, len(clusterRoleBindingObjects))

	// Check if the returned clusterRoleBindingObjects contains the expected ClusterRoleBinding objects
	neptuneManagerRB, ok := clusterRoleBindingObjects[0].(*rbacv1.ClusterRoleBinding)
	assert.True(t, ok)
	neptuneProxyRB, ok := clusterRoleBindingObjects[1].(*rbacv1.ClusterRoleBinding)
	assert.True(t, ok)

	// Check if the ClusterRoleBinding objects have the expected properties
	assert.Equal(t, "neptune-manager-rolebinding", neptuneManagerRB.Name)
	assert.Equal(t, "neptune-proxy-rolebinding", neptuneProxyRB.Name)

	// Check if the ClusterRoleBinding objects have the expected subjects
	expectedSubjects := []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      common.NeptuneName,
			Namespace: m.Namespace,
		},
	}
	assert.Equal(t, expectedSubjects, neptuneManagerRB.Subjects)
	assert.Equal(t, expectedSubjects, neptuneProxyRB.Subjects)

	// Check if the ClusterRoleBinding objects have the expected role references
	expectedManagerRoleRef := rbacv1.RoleRef{
		Kind:     "ClusterRole",
		Name:     common.NeptuneClusterRoleName,
		APIGroup: "rbac.authorization.k8s.io",
	}
	expectedProxyRoleRef := rbacv1.RoleRef{
		Kind:     "ClusterRole",
		Name:     "neptune-proxy-role",
		APIGroup: "rbac.authorization.k8s.io",
	}
	assert.Equal(t, expectedManagerRoleRef, neptuneManagerRB.RoleRef)
	assert.Equal(t, expectedProxyRoleRef, neptuneProxyRB.RoleRef)
}
