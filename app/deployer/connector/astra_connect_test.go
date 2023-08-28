package connector_test

import (
	"context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"testing"

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
			ImageRegistry: v1.ImageRegistry{
				Name:   "test-registry",
				Secret: "test-secret",
			},

			AstraConnect: v1.AstraConnect{
				Image:    "test-image",
				Replicas: 3,
			},
		},
	}

	objects, err := deployer.GetDeploymentObjects(astraConnector, ctx)
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
	assert.Equal(t, "test-registry/test-image", container.Image)
	assert.Equal(t, common.AstraConnectName, container.Name)

	assert.Equal(t, 4, len(container.Env))
	assert.Equal(t, "NATS_SERVER_URL", container.Env[0].Name)
	assert.Equal(t, "LOG_LEVEL", container.Env[1].Name)
	assert.Equal(t, "trace", container.Env[1].Value)

	assert.Equal(t, 1, len(deployment.Spec.Template.Spec.ImagePullSecrets))
	assert.Equal(t, "test-secret", deployment.Spec.Template.Spec.ImagePullSecrets[0].Name)
}

func TestAstraConnectGetDeploymentObjectsUsingDefaults(t *testing.T) {
	deployer := connector.NewAstraConnectorDeployer()
	ctx := context.Background()

	astraConnector := &v1.AstraConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-astra-connector",
			Namespace: "test-namespace",
		},
		Spec: v1.AstraConnectorSpec{
			AstraConnect: v1.AstraConnect{
				Replicas: -3,
			},
		},
	}

	objects, err := deployer.GetDeploymentObjects(astraConnector, ctx)
	assert.NoError(t, err)
	assert.NotNil(t, objects)
	assert.Equal(t, 1, len(objects))

	deployment, ok := objects[0].(*appsv1.Deployment)
	assert.True(t, ok)

	assert.Equal(t, "test-namespace", deployment.Namespace)
	assert.Equal(t, common.AstraConnectName, deployment.Name)

	assert.Equal(t, int32(1), *deployment.Spec.Replicas)
	assert.Equal(t, common.AstraConnectName, deployment.Spec.Template.Spec.ServiceAccountName)

	container := deployment.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "netappdownloads.jfrog.io/docker-astra-control-staging/arch30/neptune/astra-connector:1.0.202308281710", container.Image)
	assert.Equal(t, common.AstraConnectName, container.Name)
	assert.Equal(t, 4, len(container.Env))
	assert.Equal(t, "NATS_SERVER_URL", container.Env[0].Name)
	assert.Equal(t, "LOG_LEVEL", container.Env[1].Name)
	assert.Equal(t, "trace", container.Env[1].Value)

	assert.Equal(t, 0, len(deployment.Spec.Template.Spec.ImagePullSecrets))
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
				Image:    "test-image",
				Replicas: 1,
			},
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

	objects, err := deployer.GetConfigMapObjects(m, ctx)
	assert.NoError(t, err)
	assert.NotNil(t, objects)
	assert.Equal(t, 1, len(objects))

	configMap, ok := objects[0].(*corev1.ConfigMap)
	assert.True(t, ok)
	assert.Equal(t, "test-namespace", configMap.Namespace)
	assert.Equal(t, common.AstraConnectName, configMap.Name)

	expectedData := map[string]string{
		"nats_url":            "nats://nats.test-namespace:4222",
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

	objects, err := deployer.GetConfigMapObjects(m, ctx)
	assert.NoError(t, err)
	assert.NotNil(t, objects)
	assert.Equal(t, 1, len(objects))

	configMap, ok := objects[0].(*corev1.ConfigMap)
	assert.True(t, ok)
	assert.Equal(t, "something", configMap.Namespace)
	assert.Equal(t, common.AstraConnectName, configMap.Name)

	expectedData := map[string]string{
		"nats_url":            "nats://nats.something:4222",
		"skip_tls_validation": "false",
	}
	assert.Equal(t, expectedData, configMap.Data)
}

func TestAstraConnectGetServiceAccountObjects(t *testing.T) {
	deployer := connector.NewAstraConnectorDeployer()
	ctx := context.Background()

	m := DummyAstraConnector()

	objects, err := deployer.GetServiceAccountObjects(&m, ctx)
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

	objects, err := deployer.GetClusterRoleObjects(&m, ctx)
	assert.NoError(t, err)
	assert.NotNil(t, objects)
	assert.Equal(t, 1, len(objects))

	clusterRole, ok := objects[0].(*rbacv1.ClusterRole)
	assert.True(t, ok)
	assert.Equal(t, common.AstraConnectName, clusterRole.Name)

	// TODO look at rules
}

func TestAstraConnectGetClusterRoleBindingObjects(t *testing.T) {
	deployer := connector.NewAstraConnectorDeployer()
	ctx := context.Background()

	m := DummyAstraConnector()

	objects, err := deployer.GetClusterRoleBindingObjects(&m, ctx)
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

// Below are all the nil objects

func TestAstraConnectK8sObjectsNotCreated(t *testing.T) {
	deployer := connector.NewAstraConnectorDeployer()
	ctx := context.Background()

	m := DummyAstraConnector()

	objects, err := deployer.GetServiceObjects(&m, ctx)
	assert.Nil(t, objects)
	assert.Nil(t, err)

	objects, err = deployer.GetStatefulSetObjects(&m, ctx)
	assert.Nil(t, objects)
	assert.Nil(t, err)

	objects, err = deployer.GetRoleObjects(&m, ctx)
	assert.Nil(t, objects)
	assert.Nil(t, err)

	objects, err = deployer.GetRoleBindingObjects(&m, ctx)
	assert.Nil(t, objects)
	assert.Nil(t, err)
}
