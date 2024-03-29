package connector_test

import (
	"context"
	"testing"

	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/connector"
	"github.com/NetApp-Polaris/astra-connector-operator/common"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

			Nats: v1.Nats{
				Replicas: 3,
				Image:    "test-image",
			},

			ImageRegistry: v1.ImageRegistry{
				Name:   "test-registry",
				Secret: "test-secret",
			},
		},
	}

	objects, _, err := deployer.GetStatefulSetObjects(m, ctx)
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

func TestNatsGetStatefulSetObjectsUseDefaults(t *testing.T) {
	deployer := connector.NewNatsDeployer()
	ctx := context.Background()

	m := &v1.AstraConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-connector",
			Namespace: "test-namespace",
		},
		Spec: v1.AstraConnectorSpec{

			Nats: v1.Nats{
				Replicas: -2,
			},
		},
	}

	objects, _, err := deployer.GetStatefulSetObjects(m, ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(objects))

	statefulSet, ok := objects[0].(*appsv1.StatefulSet)
	assert.True(t, ok)

	assert.Equal(t, common.NatsName, statefulSet.Name)
	assert.Equal(t, m.Namespace, statefulSet.Namespace)
	assert.Equal(t, int32(1), *statefulSet.Spec.Replicas)
	assert.Equal(t, "netappdownloads.jfrog.io/docker-astra-control-staging/arch30/neptune/nats:2.10.1-alpine3.18", statefulSet.Spec.Template.Spec.Containers[0].Image)
	assert.Nil(t, statefulSet.Spec.Template.Spec.ImagePullSecrets)
}

func TestNatsGetConfigMapObjects(t *testing.T) {
	deployer := connector.NewNatsDeployer()
	ctx := context.Background()

	m := DummyAstraConnector()

	objects, _, err := deployer.GetConfigMapObjects(&m, ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(objects))

	configMap, ok := objects[0].(*corev1.ConfigMap)
	assert.True(t, ok)

	// Todo Add assertions for the expected values in the ConfigMap object
	assert.Equal(t, common.NatsConfigMapName, configMap.Name)
	assert.Equal(t, m.Namespace, configMap.Namespace)
	data := map[string]string{"nats.conf": "pid_file: \"/var/run/nats/nats.pid\"\nhttp: 8222\nmax_payload: 8388608\n\ncluster {\n  port: 6222\n  routes [\n    nats://nats-0.nats-cluster:6222\n    ]\n\n  cluster_advertise: $CLUSTER_ADVERTISE\n  connect_retries: 30\n}\n"}
	assert.Equal(t, data, configMap.Data)
}

func TestNatsGetServiceAccountObjects(t *testing.T) {
	deployer := connector.NewNatsDeployer()
	ctx := context.Background()

	m := DummyAstraConnector()

	objects, _, err := deployer.GetServiceAccountObjects(&m, ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(objects))

	serviceAccount, ok := objects[0].(*corev1.ServiceAccount)
	assert.True(t, ok)

	assert.Equal(t, common.NatsServiceAccountName, serviceAccount.Name)
	assert.Equal(t, m.Namespace, serviceAccount.Namespace)
	assert.Equal(t, map[string]string{"Label1": "Value1", "app": "nats"}, serviceAccount.Labels)
}

func TestNatsGetServiceObjects(t *testing.T) {
	deployer := connector.NewNatsDeployer()
	ctx := context.Background()

	m := DummyAstraConnector()

	objects, _, err := deployer.GetServiceObjects(&m, ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(objects))

	// test the first service which is natsService
	service, ok := objects[0].(*corev1.Service)
	assert.True(t, ok)

	assert.Equal(t, common.NatsName, service.Name)
	assert.Equal(t, m.Namespace, service.Namespace)
	assert.Equal(t, map[string]string{"Label1": "Value1", "app": "nats"}, service.Labels)
	assert.Equal(t, corev1.ServiceTypeClusterIP, service.Spec.Type)

	// now test the second service nats-cluster
	service, ok = objects[1].(*corev1.Service)
	assert.True(t, ok)

	assert.Equal(t, common.NatsClusterServiceName, service.Name)
	assert.Equal(t, m.Namespace, service.Namespace)
	assert.Equal(t, map[string]string{"Label1": "Value1", "app": "nats"}, service.Labels)

}

// Below are all the nil objects

func TestNatsK8sObjectsNotCreated(t *testing.T) {
	deployer := connector.NewNatsDeployer()
	ctx := context.Background()

	m := DummyAstraConnector()

	objects, fn, err := deployer.GetDeploymentObjects(&m, ctx)
	assert.Nil(t, objects)
	assert.NotNil(t, fn)
	assert.Nil(t, err)

	objects, fn, err = deployer.GetRoleObjects(&m, ctx)
	assert.NotNil(t, objects)
	assert.NotNil(t, fn)
	assert.Nil(t, err)

	objects, fn, err = deployer.GetRoleBindingObjects(&m, ctx)
	assert.NotNil(t, objects)
	assert.NotNil(t, fn)
	assert.Nil(t, err)

	objects, fn, err = deployer.GetClusterRoleObjects(&m, ctx)
	assert.Nil(t, objects)
	assert.NotNil(t, fn)
	assert.Nil(t, err)

	objects, fn, err = deployer.GetClusterRoleBindingObjects(&m, ctx)
	assert.Nil(t, objects)
	assert.NotNil(t, fn)
	assert.Nil(t, err)
}
