package connector_test

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/connector"
	"github.com/NetApp-Polaris/astra-connector-operator/common"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAstraConnectGetDeploymentObjects(t *testing.T) {
	deployer := connector.NewAstraConnectorDeployer()
	ctx := context.Background()

	astraConnector := &v1.AstraConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-astra-connector",
			Namespace: "test-namespace",
		},
		Spec: v1.AstraConnectorSpec{
			AutoSupport: v1.AutoSupport{Enrolled: true,
				URL: "https://my-asup"},

			ImageRegistry: v1.ImageRegistry{
				Name:   "test-registry",
				Secret: "test-secret",
			},

			AstraConnect: v1.AstraConnect{
				Image:    "test-image",
				Replicas: 3,
			},
			Astra: v1.Astra{
				ClusterId: "123",
			},
			Labels: map[string]string{"Label1": "Value1"},
		},
	}

	objects, _, err := deployer.GetDeploymentObjects(astraConnector, ctx)
	assert.NoError(t, err)
	assert.NotNil(t, objects)
	assert.Equal(t, 1, len(objects))

	deployment, ok := objects[0].(*appsv1.Deployment)
	assert.True(t, ok)

	assert.Equal(t, "test-namespace", deployment.Namespace)
	assert.Equal(t, common.AstraConnectName, deployment.Name)

	assert.Equal(t, int32(3), *deployment.Spec.Replicas)
	assert.Equal(t, common.AstraConnectName, deployment.Spec.Template.Spec.ServiceAccountName)

	container := deployment.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "test-registry/astra-connector:test-image", container.Image)
	assert.Equal(t, common.AstraConnectName, container.Name)

	assert.Equal(t, 10, len(container.Env))
	assert.Equal(t, "LOG_LEVEL", container.Env[0].Name)
	assert.Equal(t, "true", container.Env[1].Value)

	assert.Equal(t, 1, len(deployment.Spec.Template.Spec.ImagePullSecrets))
	assert.Equal(t, "test-secret", deployment.Spec.Template.Spec.ImagePullSecrets[0].Name)
}

func TestAstraConnect_ClusterIDAndNameEmpty(t *testing.T) {
	deployer := connector.NewAstraConnectorDeployer()
	ctx := context.Background()

	astraConnector := &v1.AstraConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-astra-connector",
			Namespace: "test-namespace",
		},
		Spec: v1.AstraConnectorSpec{
			AutoSupport: v1.AutoSupport{Enrolled: true,
				URL: "https://my-asup"},

			ImageRegistry: v1.ImageRegistry{
				Name:   "test-registry",
				Secret: "test-secret",
			},

			AstraConnect: v1.AstraConnect{
				Image:    "test-image",
				Replicas: 3,
			},
			Astra:  v1.Astra{},
			Labels: map[string]string{"Label1": "Value1"},
		},
	}

	objects, f, err := deployer.GetDeploymentObjects(astraConnector, ctx)
	assert.Error(t, err)
	assert.Nil(t, objects)
	assert.Nil(t, f)
}

func DummyAstraConnector() v1.AstraConnector {
	return v1.AstraConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-astraconnector",
			Namespace: "test-namespace",
		},
		Spec: v1.AstraConnectorSpec{
			ImageRegistry: v1.ImageRegistry{
				Name:   "test-registry",
				Secret: "test-secret",
			},
			AstraConnect: v1.AstraConnect{
				Image: "test-image",
			},
			AutoSupport: v1.AutoSupport{
				Enrolled: true,
				URL:      "https://my-asup"},
			Labels: map[string]string{"Label1": "Value1"},
		},
	}
}

func TestAstraConnectGetConfigMapObjectsSkipTLSValidationTrue(t *testing.T) {
	deployer := connector.NewAstraConnectorDeployer()
	ctx := context.Background()

	m := &v1.AstraConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-astraconnector",
			Namespace: "test-namespace",
		},
		Spec: v1.AstraConnectorSpec{
			Astra: v1.Astra{
				SkipTLSValidation: true,
			},
		},
	}

	objects, _, err := deployer.GetConfigMapObjects(m, ctx)
	assert.NoError(t, err)
	assert.NotNil(t, objects)
	assert.Equal(t, 1, len(objects))

	configMap, ok := objects[0].(*corev1.ConfigMap)
	assert.True(t, ok)
	assert.Equal(t, "test-namespace", configMap.Namespace)
	assert.Equal(t, common.AstraConnectName, configMap.Name)

	expectedData := map[string]string{
		"skip_tls_validation": "true",
	}
	assert.Equal(t, expectedData, configMap.Data)
}

func TestAstraConnectGetConfigMapObjectsSkipTLSValidationFalse(t *testing.T) {
	deployer := connector.NewAstraConnectorDeployer()
	ctx := context.Background()

	m := &v1.AstraConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-astraconnector",
			Namespace: "something",
		},
		Spec: v1.AstraConnectorSpec{
			Astra: v1.Astra{
				SkipTLSValidation: false,
			},
		},
	}

	objects, _, err := deployer.GetConfigMapObjects(m, ctx)
	assert.NoError(t, err)
	assert.NotNil(t, objects)
	assert.Equal(t, 1, len(objects))

	configMap, ok := objects[0].(*corev1.ConfigMap)
	assert.True(t, ok)
	assert.Equal(t, "something", configMap.Namespace)
	assert.Equal(t, common.AstraConnectName, configMap.Name)

	expectedData := map[string]string{
		"skip_tls_validation": "false",
	}
	assert.Equal(t, expectedData, configMap.Data)
}

func TestAstraConnectGetServiceAccountObjects(t *testing.T) {
	deployer := connector.NewAstraConnectorDeployer()
	ctx := context.Background()

	m := DummyAstraConnector()

	objects, _, err := deployer.GetServiceAccountObjects(&m, ctx)
	assert.NoError(t, err)
	assert.NotNil(t, objects)
	assert.Equal(t, 1, len(objects))

	serviceAccount, ok := objects[0].(*corev1.ServiceAccount)
	assert.True(t, ok)
	assert.Equal(t, "test-namespace", serviceAccount.Namespace)
	assert.Equal(t, common.AstraConnectName, serviceAccount.Name)
}

func TestAstraConnectGetClusterRoleObjects(t *testing.T) {
	deployer := connector.NewAstraConnectorDeployer()
	ctx := context.Background()

	m := DummyAstraConnector()

	objects, _, err := deployer.GetClusterRoleObjects(&m, ctx)
	assert.NoError(t, err)
	assert.NotNil(t, objects)
	assert.Equal(t, 1, len(objects))

	clusterRole, ok := objects[0].(*rbacv1.ClusterRole)
	assert.True(t, ok)
	assert.Equal(t, common.AstraConnectName, clusterRole.Name)

	// The expected rules may change over time so we will instead ensure we never have an overly permissive rule added
	for _, rule := range clusterRole.Rules {
		isOverlyPermissive := len(rule.APIGroups) == 1 && rule.APIGroups[0] == "*" &&
			len(rule.Resources) == 1 && rule.Resources[0] == "*" &&
			len(rule.Verbs) == 1 && rule.Verbs[0] == "*"
		assert.False(t, isOverlyPermissive, "Overly permissive rule detected")
	}
}

func TestAstraConnectGetClusterRoleBindingObjects(t *testing.T) {
	deployer := connector.NewAstraConnectorDeployer()
	ctx := context.Background()

	m := DummyAstraConnector()

	objects, _, err := deployer.GetClusterRoleBindingObjects(&m, ctx)
	assert.NoError(t, err)
	assert.NotNil(t, objects)
	assert.Equal(t, 1, len(objects))

	clusterRoleBinding, ok := objects[0].(*rbacv1.ClusterRoleBinding)
	assert.True(t, ok)
	assert.Equal(t, common.AstraConnectName, clusterRoleBinding.Name)

	expectedSubjects := []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      common.AstraConnectName,
			Namespace: m.Namespace,
		},
	}
	assert.Equal(t, expectedSubjects, clusterRoleBinding.Subjects)

	expectedRoleRef := rbacv1.RoleRef{
		Kind:     "ClusterRole",
		Name:     common.AstraConnectName,
		APIGroup: "rbac.authorization.k8s.io",
	}
	assert.Equal(t, expectedRoleRef, clusterRoleBinding.RoleRef)
}

func TestAstraConnectGetRoleObjects(t *testing.T) {
	deployer := connector.NewAstraConnectorDeployer()
	ctx := context.Background()

	m := DummyAstraConnector()

	objects, _, err := deployer.GetRoleObjects(&m, ctx)
	assert.NoError(t, err)
	assert.NotNil(t, objects)
	assert.Equal(t, 1, len(objects))

	role, ok := objects[0].(*rbacv1.Role)
	assert.True(t, ok)
	assert.Equal(t, common.AstraConnectName, role.Name)
	assert.Equal(t, m.Namespace, role.Namespace)

	// The expected rules may change over time so we will instead ensure we never have an overly permissive rule added
	for _, rule := range role.Rules {
		isOverlyPermissive := len(rule.APIGroups) == 1 && rule.APIGroups[0] == "*" &&
			len(rule.Resources) == 1 && rule.Resources[0] == "*" &&
			len(rule.Verbs) == 1 && rule.Verbs[0] == "*"
		assert.False(t, isOverlyPermissive, "Overly permissive rule detected")
	}
}

func TestAstraConnectGetRoleBindingObjects(t *testing.T) {
	deployer := connector.NewAstraConnectorDeployer()
	ctx := context.Background()

	m := DummyAstraConnector()

	objects, _, err := deployer.GetRoleBindingObjects(&m, ctx)
	assert.NoError(t, err)
	assert.NotNil(t, objects)
	assert.Equal(t, 1, len(objects))

	roleBinding, ok := objects[0].(*rbacv1.RoleBinding)
	assert.True(t, ok)
	assert.Equal(t, common.AstraConnectName, roleBinding.Name)
	assert.Equal(t, m.Namespace, roleBinding.Namespace)

	assert.Equal(t, 1, len(roleBinding.Subjects))
	assert.Equal(t, "ServiceAccount", roleBinding.Subjects[0].Kind)
	assert.Equal(t, common.AstraConnectName, roleBinding.Subjects[0].Name)
	assert.Equal(t, m.Namespace, roleBinding.Subjects[0].Namespace)

	assert.Equal(t, "Role", roleBinding.RoleRef.Kind)
	assert.Equal(t, common.AstraConnectName, roleBinding.RoleRef.Name)
	assert.Equal(t, "rbac.authorization.k8s.io", roleBinding.RoleRef.APIGroup)
}

// Below are all the nil objects

func TestAstraConnectK8sObjectsNotCreated(t *testing.T) {
	deployer := connector.NewAstraConnectorDeployer()
	ctx := context.Background()

	m := DummyAstraConnector()

	objects, fn, err := deployer.GetServiceObjects(&m, ctx)
	assert.Nil(t, objects)
	assert.NotNil(t, fn)
	assert.Nil(t, err)

	objects, fn, err = deployer.GetStatefulSetObjects(&m, ctx)
	assert.Nil(t, objects)
	assert.NotNil(t, fn)
	assert.Nil(t, err)
}
