package neptune_test

import (
	"context"
	"github.com/NetApp-Polaris/astra-connector-operator/api/v1"
	"github.com/NetApp-Polaris/astra-connector-operator/common"
	corev1 "k8s.io/api/core/v1"
	"testing"

	"github.com/stretchr/testify/assert"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/neptune"
)

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
			AutoSupport: v1.AutoSupport{
				Enrolled: true,
				URL:      "https://my-asup"},
			Labels: map[string]string{"Label1": "Value1"},
		},
	}

	// Create a new context
	ctx := context.Background()

	return n, &m, ctx
}

func TestGetDeploymentObjectsV2(t *testing.T) {
	n, m, ctx := createNeptuneDeployerV2()

	// Call the GetDeploymentObjects method
	deploymentObjects, fn, err := n.GetDeploymentObjects(m, ctx)

	// Check if there is no error
	assert.NoError(t, err)
	assert.NotNil(t, fn)

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

func TestUnimplementedObjectsV2(t *testing.T) {
	n, m, ctx := createNeptuneDeployerV2()

	// Call the GetDeploymentObjects method
	ret, fn, err := n.GetStatefulSetObjects(m, ctx)
	assert.Nil(t, ret)
	assert.NotNil(t, fn)
	assert.NoError(t, err)

	ret, fn, err = n.GetConfigMapObjects(m, ctx)
	assert.Nil(t, ret)
	assert.NotNil(t, fn)
	assert.NoError(t, err)

	ret, fn, err = n.GetServiceAccountObjects(m, ctx)
	assert.NotNil(t, ret)
	assert.NotNil(t, fn)
	assert.NoError(t, err)

	ret, fn, err = n.GetClusterRoleObjects(m, ctx)
	assert.Nil(t, ret)
	assert.NotNil(t, fn)
	assert.NoError(t, err)

	ret, fn, err = n.GetRoleBindingObjects(m, ctx)
	assert.Nil(t, ret)
	assert.NotNil(t, fn)
	assert.NoError(t, err)

	ret, fn, err = n.GetRoleObjects(m, ctx)
	assert.Nil(t, ret)
	assert.NotNil(t, fn)
	assert.NoError(t, err)

	ret, fn, err = n.GetClusterRoleBindingObjects(m, ctx)
	assert.Nil(t, ret)
	assert.NotNil(t, fn)
	assert.NoError(t, err)
}

func TestGetServiceObjects(t *testing.T) {
	n, m, ctx := createNeptuneDeployerV2()

	// Call the GetServiceObjects method
	serviceObjects, fn, err := n.GetServiceObjects(m, ctx)
	assert.NotNil(t, fn)

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
