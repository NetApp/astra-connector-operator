/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package httpproxy_client

import (
	"context"
	"fmt"

	v1 "github.com/NetApp/astraagent-operator/api/v1"
	"github.com/NetApp/astraagent-operator/common"
	"github.com/NetApp/astraagent-operator/nats"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

type Deployer struct{}

func NewHttpproxyClientDeployer() *Deployer {
	return &Deployer{}
}

// GetDeploymentObject returns an HttpProxyClient Deployment object
func (d *Deployer) GetDeploymentObject(m *v1.AstraAgent, ctx context.Context) (*appsv1.Deployment, error) {
	log := ctrllog.FromContext(ctx)
	ls := labelsForProxyClient(common.HttpProxyClientName)
	replicas := int32(common.HttpProxyClientsize)

	var imageRegistry string
	var containerImage string
	var httpProxyClientImage string
	if m.Spec.ImageRegistry.Name != "" {
		imageRegistry = m.Spec.EchoClient.Image
	} else {
		imageRegistry = common.DefaultImageRegistry
	}

	if m.Spec.HttpProxyClient.Image != "" {
		containerImage = m.Spec.HttpProxyClient.Image
	} else {
		containerImage = common.HttpProxyClientDefaultImage
	}

	httpProxyClientImage = fmt.Sprintf("%s/%s", imageRegistry, containerImage)
	log.Info("Using HttpProxyClient image", "image", httpProxyClientImage)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.HttpProxyClientName,
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
						Image: httpProxyClientImage,
						Name:  common.HttpProxyClientName,
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

// labelsForProxyClient returns the labels for selecting the HttpProxyClient
func labelsForProxyClient(name string) map[string]string {
	return map[string]string{"app": name}
}

func (d *Deployer) GetStatefulsetObject(m *v1.AstraAgent, ctx context.Context) (*appsv1.StatefulSet, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d *Deployer) GetServiceObject(m *v1.AstraAgent, serviceName string) (*corev1.Service, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d *Deployer) GetConfigMapObject(m *v1.AstraAgent) (*corev1.ConfigMap, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d *Deployer) GetServiceAccountObject(m *v1.AstraAgent) (*corev1.ServiceAccount, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d *Deployer) GetRoleObject(m *v1.AstraAgent) (*rbacv1.Role, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d *Deployer) GetRoleBindingObject(m *v1.AstraAgent) (*rbacv1.RoleBinding, error) {
	return nil, fmt.Errorf("not implemented")
}
