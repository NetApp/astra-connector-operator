package connector_test

import (
	"context"
	"github.com/NetApp-Polaris/astra-connector-operator/common"
	"github.com/NetApp-Polaris/astra-connector-operator/deployer/connector"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestNatsGetStatefulSetObjects(t *testing.T) {
	deployer := connector.NewNatsDeployer()
	ctx := context.Background()

	m := &v1.AstraConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-connector",
			Namespace: "test-namespace",
		},
		Spec: v1.AstraConnectorSpec{
			ConnectorSpec: v1.ConnectorSpec{
				Nats: v1.Nats{
					Size:  3,
					Image: "test-image",
				},
			},
			ImageRegistry: v1.ImageRegistry{
				Name:   "test-registry",
				Secret: "test-secret",
			},
		},
	}

	objects, err := deployer.GetStatefulSetObjects(m, ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(objects))

	statefulSet, ok := objects[0].(*appsv1.StatefulSet)
	assert.True(t, ok)

	assert.Equal(t, common.NatsName, statefulSet.Name)
	assert.Equal(t, m.Namespace, statefulSet.Namespace)
	assert.Equal(t, int32(3), *statefulSet.Spec.Replicas)
	assert.Equal(t, "test-registry/test-image", statefulSet.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, []corev1.LocalObjectReference{{Name: "test-secret"}}, statefulSet.Spec.Template.Spec.ImagePullSecrets)
}

// Below are all the nil objects

func TestNatsK8sObjectsNotCreated(t *testing.T) {
	deployer := connector.NewNatsDeployer()
	ctx := context.Background()

	m := DummyAstraConnector()

	objects, err := deployer.GetDeploymentObjects(&m, ctx)
	assert.Nil(t, objects)
	assert.Nil(t, err)

	objects, err = deployer.GetRoleObjects(&m, ctx)
	assert.Nil(t, objects)
	assert.Nil(t, err)

	objects, err = deployer.GetRoleBindingObjects(&m, ctx)
	assert.Nil(t, objects)
	assert.Nil(t, err)

	objects, err = deployer.GetClusterRoleObjects(&m, ctx)
	assert.Nil(t, objects)
	assert.Nil(t, err)

	objects, err = deployer.GetClusterRoleBindingObjects(&m, ctx)
	assert.Nil(t, objects)
	assert.Nil(t, err)
}
