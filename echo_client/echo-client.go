/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package echo_client

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"

	v1 "github.com/NetApp/astra-connector-operator/api/v1"
	"github.com/NetApp/astra-connector-operator/common"
	"github.com/NetApp/astra-connector-operator/nats"
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
func (d *Deployer) GetDeploymentObject(m *v1.AstraConnector, ctx context.Context) (*appsv1.Deployment, error) {
	log := ctrllog.FromContext(ctx)
	ls := labelsForEchoClient(common.EchoClientName)
	var replicas int32
	if m.Spec.EchoClient.Size > 0 {
		replicas = m.Spec.EchoClient.Size
	} else {
		log.Info("Defaulting the EchoClient replica size", "size", common.EchoClientDefaultSize)
		replicas = common.EchoClientDefaultSize
	}

	var imageRegistry string
	var echoClientImage string
	var containerImage string
	if m.Spec.ImageRegistry.Name != "" {
		imageRegistry = m.Spec.ImageRegistry.Name
	} else {
		imageRegistry = common.DefaultImageRegistry
	}

	if m.Spec.EchoClient.Image != "" {
		containerImage = m.Spec.EchoClient.Image
	} else {
		containerImage = common.EchoClientDefaultImage
	}

	echoClientImage = fmt.Sprintf("%s/%s", imageRegistry, containerImage)
	log.Info("Using EchoClient image", "image", echoClientImage)
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

	if m.Spec.ImageRegistry.Secret != "" {
		dep.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
			{
				Name: m.Spec.ImageRegistry.Secret,
			},
		}
	}
	return dep, nil
}

// labelsForEchoClient returns the labels for selecting the EchoClient
func labelsForEchoClient(name string) map[string]string {
	return map[string]string{"app": name}
}

func (d *Deployer) GetStatefulsetObject(m *v1.AstraConnector, ctx context.Context) (*appsv1.StatefulSet, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d *Deployer) GetServiceObject(m *v1.AstraConnector, serviceName string) (*corev1.Service, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d *Deployer) GetConfigMapObject(m *v1.AstraConnector) (*corev1.ConfigMap, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d *Deployer) GetServiceAccountObject(m *v1.AstraConnector) (*corev1.ServiceAccount, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d *Deployer) GetRoleObject(m *v1.AstraConnector) (*rbacv1.Role, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d *Deployer) GetRoleBindingObject(m *v1.AstraConnector) (*rbacv1.RoleBinding, error) {
	return nil, fmt.Errorf("not implemented")
}
