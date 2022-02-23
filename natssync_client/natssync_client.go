/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package natssync_client

import (
	"context"
	"errors"
	"fmt"
	"github.com/NetApp/astraagent-operator/register"

	"strconv"
	"strings"

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

func NewNatssyncClientDeployer() *Deployer {
	return &Deployer{}
}

// GetDeploymentObject returns a Natssync-client Deployment object
func (d *Deployer) GetDeploymentObject(m *v1.AstraAgent, ctx context.Context) (*appsv1.Deployment, error) {
	log := ctrllog.FromContext(ctx)
	ls := LabelsForNatssyncClient(common.NatssyncClientName)

	var imageRegistry string
	var containerImage string
	var natssyncClientImage string
	if m.Spec.ImageRegistry.Name != "" {
		imageRegistry = m.Spec.ImageRegistry.Name
	} else {
		imageRegistry = common.DefaultImageRegistry
	}

	if m.Spec.NatssyncClient.Image != "" {
		containerImage = m.Spec.NatssyncClient.Image
	} else {
		containerImage = common.NatssyncClientDefaultImage
	}

	natssyncClientImage = fmt.Sprintf("%s/%s", imageRegistry, containerImage)
	log.Info("Using NatssyncClient image", "image", natssyncClientImage)
	natssyncCloudBridgeURL := register.GetAstraHostURL(m, ctx)
	replicas := int32(common.NatssyncClientSize)
	keyStoreURLSplit := strings.Split(common.NatssyncClientKeystoreUrl, "://")
	if len(keyStoreURLSplit) < 2 {
		return nil, errors.New("invalid keyStoreURLSplit provided, format - configmap:///configmap-data")
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NatssyncClientName,
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
						Image: natssyncClientImage,
						Name:  common.NatssyncClientName,
						Env: []corev1.EnvVar{
							{
								Name:  "NATS_SERVER_URL",
								Value: nats.GetNatsURL(m),
							},
							{
								Name:  "CLOUD_BRIDGE_URL",
								Value: natssyncCloudBridgeURL,
							},
							{
								Name:  "CONFIGMAP_NAME",
								Value: common.NatssyncClientConfigMapName,
							},
							{
								Name:  "POD_NAMESPACE",
								Value: m.Namespace,
							},
							{
								Name:  "KEYSTORE_URL",
								Value: common.NatssyncClientKeystoreUrl,
							},
							{
								Name:  "SKIP_TLS_VALIDATION",
								Value: strconv.FormatBool(m.Spec.NatssyncClient.SkipTLSValidation),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      common.NatssyncClientConfigMapVolumeName,
								MountPath: keyStoreURLSplit[1],
							},
						},
					}},
					ServiceAccountName: common.NatssyncClientConfigMapServiceAccountName,
					Volumes: []corev1.Volume{
						{
							Name: common.NatssyncClientConfigMapVolumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: common.NatssyncClientConfigMapName,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if m.Spec.NatssyncClient.HostAlias {
		hostNamesSplit := strings.Split(natssyncCloudBridgeURL, "://")
		if len(hostNamesSplit) < 2 {
			return nil, errors.New("invalid hostname provided, hostname format - https://hostname")
		}
		dep.Spec.Template.Spec.HostAliases = []corev1.HostAlias{
			{
				IP:        m.Spec.NatssyncClient.HostAliasIP,
				Hostnames: []string{hostNamesSplit[1]},
			},
		}
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

// GetServiceObject returns a Natssync-client Service object
func (d *Deployer) GetServiceObject(m *v1.AstraAgent, serviceName string) (*corev1.Service, error) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: m.Namespace,
			Labels: map[string]string{
				"app": common.NatssyncClientName,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Port:     common.NatssyncClientPort,
					Protocol: common.NatssyncClientProtocol,
				},
			},
			Selector: map[string]string{
				"app": common.NatssyncClientName,
			},
		},
	}
	return service, nil
}

// LabelsForNatssyncClient returns the labels for selecting the NatssyncClient
func LabelsForNatssyncClient(name string) map[string]string {
	return map[string]string{"app": name}
}

// GetConfigMapObject returns a ConfigMap object for NatssyncClient
func (d *Deployer) GetConfigMapObject(m *v1.AstraAgent) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: m.Namespace,
			Name:      common.NatssyncClientConfigMapName,
		},
	}
	return configMap, nil
}

// GetRoleObject returns a ConfigMapRole object for NatssyncClient
func (d *Deployer) GetRoleObject(m *v1.AstraAgent) (*rbacv1.Role, error) {
	configMapRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: m.Namespace,
			Name:      common.NatssyncClientConfigMapRoleName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"get", "list", "patch"},
			},
		},
	}
	return configMapRole, nil
}

// GetRoleBindingObject returns a Natssync-Client ConfigMapRoleBinding object
func (d *Deployer) GetRoleBindingObject(m *v1.AstraAgent) (*rbacv1.RoleBinding, error) {
	configMapRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: m.Namespace,
			Name:      common.NatssyncClientConfigMapRoleBindingName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: common.NatssyncClientConfigMapServiceAccountName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     common.NatssyncClientConfigMapRoleName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	return configMapRoleBinding, nil
}

// GetServiceAccountObject returns a ServiceAccount object for NatssyncClient
func (d *Deployer) GetServiceAccountObject(m *v1.AstraAgent) (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NatssyncClientConfigMapServiceAccountName,
			Namespace: m.Namespace,
		},
	}
	return sa, nil
}

func (d *Deployer) GetStatefulsetObject(m *v1.AstraAgent, ctx context.Context) (*appsv1.StatefulSet, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d *Deployer) GetClusterServiceObject(m *v1.AstraAgent) (*corev1.Service, error) {
	return nil, fmt.Errorf("not implemented")
}
