/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package connector

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/NetApp-Polaris/astra-connector-operator/common"
	"github.com/NetApp-Polaris/astra-connector-operator/deployer/model"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
	"github.com/NetApp-Polaris/astra-connector-operator/register"
)

type NatsSyncClientDeployer struct{}

func NewNatsSyncClientDeployer() model.Deployer {
	return &NatsSyncClientDeployer{}
}

// GetDeploymentObjects returns a Natssync-client Deployment object
func (d *NatsSyncClientDeployer) GetDeploymentObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
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

	if m.Spec.ConnectorSpec.NatssyncClient.Image != "" {
		containerImage = m.Spec.ConnectorSpec.NatssyncClient.Image
	} else {
		containerImage = common.NatssyncClientDefaultImage
	}

	natssyncClientImage = fmt.Sprintf("%s/%s", imageRegistry, containerImage)
	log.Info("Using NatssyncClient image", "image", natssyncClientImage)
	natssyncCloudBridgeURL := register.GetAstraHostURL(m)
	keyStoreURLSplit := strings.Split(common.NatssyncClientKeystoreUrl, "://")
	if len(keyStoreURLSplit) < 2 {
		return nil, errors.New("invalid keyStoreURLSplit provided, format - configmap:///configmap-data")
	}

	var replicas int32
	if m.Spec.ConnectorSpec.NatssyncClient.Size > 1 {
		replicas = m.Spec.ConnectorSpec.NatssyncClient.Size
	} else {
		log.Info("Defaulting the NatssyncClient replica size", "size", common.NatssyncClientSize)
		replicas = common.NatssyncClientSize
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
								Value: GetNatsURL(m),
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
								Value: strconv.FormatBool(m.Spec.ConnectorSpec.Astra.SkipTLSValidation),
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

	if m.Spec.ConnectorSpec.NatssyncClient.HostAlias {
		hostNamesSplit := strings.Split(natssyncCloudBridgeURL, "://")
		if len(hostNamesSplit) < 2 {
			return nil, errors.New("invalid hostname provided, hostname format - https://hostname")
		}
		dep.Spec.Template.Spec.HostAliases = []corev1.HostAlias{
			{
				IP:        m.Spec.ConnectorSpec.NatssyncClient.HostAliasIP,
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
	return []client.Object{dep}, nil
}

// GetServiceObjects returns a Natssync-client Service object
func (d *NatsSyncClientDeployer) GetServiceObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NatssyncClientName,
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
	return []client.Object{service}, nil
}

// LabelsForNatssyncClient returns the labels for selecting the NatssyncClient
func LabelsForNatssyncClient(name string) map[string]string {
	return map[string]string{"app": name}
}

// GetConfigMapObjects returns a ConfigMap object for NatssyncClient
func (d *NatsSyncClientDeployer) GetConfigMapObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: m.Namespace,
			Name:      common.NatssyncClientConfigMapName,
		},
	}
	return []client.Object{configMap}, nil
}

// GetRoleObjects returns a ConfigMapRole object for NatssyncClient
func (d *NatsSyncClientDeployer) GetRoleObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
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
	return []client.Object{configMapRole}, nil
}

// GetRoleBindingObjects returns a Natssync-Client ConfigMapRoleBinding object
func (d *NatsSyncClientDeployer) GetRoleBindingObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
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
	return []client.Object{configMapRoleBinding}, nil
}

// GetServiceAccountObjects returns a ServiceAccount object for NatssyncClient
func (d *NatsSyncClientDeployer) GetServiceAccountObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NatssyncClientConfigMapServiceAccountName,
			Namespace: m.Namespace,
		},
	}
	return []client.Object{sa}, nil
}

func (d *NatsSyncClientDeployer) GetStatefulSetObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}

func (d *NatsSyncClientDeployer) GetClusterRoleObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}

func (d *NatsSyncClientDeployer) GetClusterRoleBindingObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}
