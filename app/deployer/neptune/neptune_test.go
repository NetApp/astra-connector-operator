package neptune_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/neptune"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
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
		},
	}

	// Create a new context
	ctx := context.Background()

	return n, &m, ctx
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
