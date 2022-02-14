package echo_client

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"

	cachev1 "github.com/NetApp/astraagent-operator/api/v1"
	"github.com/NetApp/astraagent-operator/common"
	"github.com/NetApp/astraagent-operator/nats"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

type Deployer struct{}

func NewEchoClientDeployer() *Deployer {
	return &Deployer{}
}

// GetDeploymentObject returns an EchoClient Deployment object
func (d *Deployer) GetDeploymentObject(m *cachev1.AstraAgent, ctx context.Context) (*appsv1.Deployment, error) {
	log := ctrllog.FromContext(ctx)
	ls := labelsForEchoClient(common.EchoClientName)
	var replicas int32
	if m.Spec.EchoClient.Size > 0 {
		replicas = m.Spec.EchoClient.Size
	} else {
		log.Info("Defaulting the Nats replica size", "size", common.EchoClientDefaultSize)
		replicas = common.EchoClientDefaultSize
	}

	var echoClientImage string
	if m.Spec.EchoClient.Image != "" {
		echoClientImage = m.Spec.EchoClient.Image
	} else {
		log.Info("Defaulting the EchoClient image", "image", common.EchoClientDefaultImage)
		echoClientImage = common.EchoClientDefaultImage
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.EchoClientName,
			Namespace: m.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: echoClientImage,
						Name:  common.EchoClientName,
						Env: []corev1.EnvVar{
							{
								Name:  "NATS_SERVER_URL",
								Value: nats.GetNatsURL(m),
							},
						},
					}},
				},
			},
		},
	}
	return dep, nil
}

// labelsForEchoClient returns the labels for selecting the EchoClient
// belonging to the given astraAgent CR name.
func labelsForEchoClient(name string) map[string]string {
	return map[string]string{"app": name}
}

func (d Deployer) GetStatefulsetObject(m *cachev1.AstraAgent, ctx context.Context) (*appsv1.StatefulSet, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d Deployer) GetServiceObject(m *cachev1.AstraAgent) (*corev1.Service, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d Deployer) GetClusterServiceObject(m *cachev1.AstraAgent) (*corev1.Service, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d Deployer) GetConfigMapObject(m *cachev1.AstraAgent) (*corev1.ConfigMap, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d Deployer) GetServiceAccountObject(m *cachev1.AstraAgent) (*corev1.ServiceAccount, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d Deployer) GetRoleObject(m *cachev1.AstraAgent) (*rbacv1.Role, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d Deployer) GetRoleBindingObject(m *cachev1.AstraAgent) (*rbacv1.RoleBinding, error) {
	return nil, fmt.Errorf("not implemented")
}
