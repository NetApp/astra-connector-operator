package connector_test

import (
	"context"
	"github.com/NetApp-Polaris/astra-connector-operator/common"
	"github.com/NetApp-Polaris/astra-connector-operator/deployer/connector"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"testing"
)

func TestNatsSyncGetDeploymentObjects(t *testing.T) {
	mockAstraConnector := &v1.AstraConnector{
		Spec: v1.AstraConnectorSpec{
			ImageRegistry: v1.ImageRegistry{
				Name:   "test-registry",
				Secret: "test-secret",
			},
			NatsSyncClient: v1.NatsSyncClient{
				Image:       "test-image",
				Replicas:    2,
				HostAliasIP: "192.168.1.1",
			},
			Astra: v1.Astra{
				SkipTLSValidation: true,
			},
		},
	}

	deployer := connector.NewNatsSyncClientDeployer()

	objects, err := deployer.GetDeploymentObjects(mockAstraConnector, context.Background())
	assert.NoError(t, err)
	assert.Len(t, objects, 1)

	deployment, ok := objects[0].(*appsv1.Deployment)
	assert.True(t, ok)

	assert.Equal(t, int32(2), *deployment.Spec.Replicas)
	assert.Equal(t, "test-registry/test-image", deployment.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, "192.168.1.1", deployment.Spec.Template.Spec.HostAliases[0].IP)
	assert.Equal(t, "test-secret", deployment.Spec.Template.Spec.ImagePullSecrets[0].Name)
	assert.Equal(t, 1, len(deployment.Spec.Template.Spec.Containers))
	assert.Equal(t, "natssync-client", deployment.Spec.Template.Spec.Containers[0].Name)

}

func TestNatsSyncGetDeploymentObjectsDefault(t *testing.T) {
	mockAstraConnector := &v1.AstraConnector{
		Spec: v1.AstraConnectorSpec{
			NatsSyncClient: v1.NatsSyncClient{
				HostAliasIP: "192.168.1.1",
			},
			Astra: v1.Astra{
				SkipTLSValidation: false,
			},
		},
	}
	deployer := connector.NewNatsSyncClientDeployer()
	objects, err := deployer.GetDeploymentObjects(mockAstraConnector, context.Background())
	assert.NoError(t, err)
	assert.Len(t, objects, 1)

	deployment, ok := objects[0].(*appsv1.Deployment)
	assert.True(t, ok)

	assert.Equal(t, int32(1), *deployment.Spec.Replicas)
	assert.Equal(t, "docker.io/theotw/natssync-client:2.1.202306281610", deployment.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, "192.168.1.1", deployment.Spec.Template.Spec.HostAliases[0].IP)
	assert.Nil(t, deployment.Spec.Template.Spec.ImagePullSecrets)
	// TODO add more checks
}

func TestGetServiceObjects(t *testing.T) {
	mockAstraConnector := &v1.AstraConnector{
		Spec: v1.AstraConnectorSpec{
			ImageRegistry: v1.ImageRegistry{
				Name:   "test-registry",
				Secret: "test-secret",
			},
			NatsSyncClient: v1.NatsSyncClient{
				Image:       "test-image",
				Replicas:    2,
				HostAliasIP: "192.168.1.1",
			},
			Astra: v1.Astra{
				SkipTLSValidation: true,
			},
		},
	}

	deployer := connector.NewNatsSyncClientDeployer()
	objects, err := deployer.GetServiceObjects(mockAstraConnector, context.Background())
	assert.NoError(t, err)
	assert.Len(t, objects, 1)

	service, ok := objects[0].(*corev1.Service)
	assert.True(t, ok)

	assert.Equal(t, corev1.ServiceTypeClusterIP, service.Spec.Type)
	assert.Equal(t, int32(8080), service.Spec.Ports[0].Port)
	assert.Equal(t, corev1.Protocol("TCP"), service.Spec.Ports[0].Protocol)
	assert.Equal(t, common.NatsSyncClientName, service.Spec.Selector["app"])
}

func TestNatsSyncGetConfigMapObjects(t *testing.T) {
	mockAstraConnector := DummyAstraConnector()
	deployer := connector.NewNatsSyncClientDeployer()

	objects, err := deployer.GetConfigMapObjects(&mockAstraConnector, context.Background())
	assert.NoError(t, err)
	assert.Len(t, objects, 1)

	configMap, ok := objects[0].(*corev1.ConfigMap)
	assert.True(t, ok)

	assert.Equal(t, common.NatsSyncClientConfigMapName, configMap.Name)
}

func TestNatsSyncGetRoleObjects(t *testing.T) {
	mockAstraConnector := DummyAstraConnector()
	deployer := connector.NewNatsSyncClientDeployer()

	objects, err := deployer.GetRoleObjects(&mockAstraConnector, context.Background())
	assert.NoError(t, err)
	assert.Len(t, objects, 1)

	role, ok := objects[0].(*rbacv1.Role)
	assert.True(t, ok)

	assert.Equal(t, common.NatsSyncClientConfigMapRoleName, role.Name)
	assert.Len(t, role.Rules, 1)
	assert.Equal(t, []string{""}, role.Rules[0].APIGroups)
	assert.Equal(t, []string{"configmaps"}, role.Rules[0].Resources)
	assert.Equal(t, []string{"get", "list", "patch"}, role.Rules[0].Verbs)
}

func TestGetRoleBindingObjects(t *testing.T) {
	mockAstraConnector := DummyAstraConnector()
	deployer := connector.NewNatsSyncClientDeployer()
	objects, err := deployer.GetRoleBindingObjects(&mockAstraConnector, context.Background())
	assert.NoError(t, err)

	assert.Len(t, objects, 1)

	roleBinding, ok := objects[0].(*rbacv1.RoleBinding)
	assert.True(t, ok)

	assert.Equal(t, common.NatsSyncClientConfigMapRoleBindingName, roleBinding.Name)
	assert.Len(t, roleBinding.Subjects, 1)
	assert.Equal(t, "ServiceAccount", roleBinding.Subjects[0].Kind)
	assert.Equal(t, common.NatsSyncClientConfigMapServiceAccountName, roleBinding.Subjects[0].Name)
	assert.Equal(t, "Role", roleBinding.RoleRef.Kind)
	assert.Equal(t, common.NatsSyncClientConfigMapRoleName, roleBinding.RoleRef.Name)
	assert.Equal(t, "rbac.authorization.k8s.io", roleBinding.RoleRef.APIGroup)
}

func TestNatsSyncGetServiceAccountObjects(t *testing.T) {
	// Create a mock AstraConnector object
	m := DummyAstraConnector()
	deployer := connector.NewNatsSyncClientDeployer()
	objects, err := deployer.GetServiceAccountObjects(&m, context.Background())
	assert.NoError(t, err)

	assert.Len(t, objects, 1)
	serviceAccount, ok := objects[0].(*corev1.ServiceAccount)
	assert.True(t, ok)

	assert.Equal(t, common.NatsSyncClientConfigMapServiceAccountName, serviceAccount.Name)
}

// Below are all the nil objects
func TestNatsSyncK8sObjectsNotCreated(t *testing.T) {
	deployer := connector.NewNatsSyncClientDeployer()
	ctx := context.Background()

	m := DummyAstraConnector()

	objects, err := deployer.GetStatefulSetObjects(&m, ctx)
	assert.Nil(t, objects)
	assert.Nil(t, err)

	objects, err = deployer.GetClusterRoleObjects(&m, ctx)
	assert.Nil(t, objects)
	assert.Nil(t, err)

	objects, err = deployer.GetClusterRoleBindingObjects(&m, ctx)
	assert.Nil(t, objects)
	assert.Nil(t, err)
}
