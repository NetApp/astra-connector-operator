/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package connector

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/NetApp-Polaris/astra-connector-operator/app/conf"
	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/model"
	"github.com/NetApp-Polaris/astra-connector-operator/app/register"
	"github.com/NetApp-Polaris/astra-connector-operator/common"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

type NatsSyncClientDeployer struct{}

func NewNatsSyncClientDeployer() model.Deployer {
	return &NatsSyncClientDeployer{}
}

// GetDeploymentObjects returns a NatsSyncClient Deployment object
func (d *NatsSyncClientDeployer) GetDeploymentObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error) {
	log := ctrllog.FromContext(ctx)
	ls := LabelsForNatsSyncClient(common.NatsSyncClientName, m.Spec.Labels)

	var imageRegistry string
	var containerImage string
	var natsSyncClientImage string
	if m.Spec.ImageRegistry.Name != "" {
		imageRegistry = m.Spec.ImageRegistry.Name
	} else {
		imageRegistry = common.DefaultImageRegistry
	}

	if m.Spec.NatsSyncClient.Image != "" {
		containerImage = m.Spec.NatsSyncClient.Image
	} else {
		containerImage = common.NatsSyncClientDefaultImage
	}

	natsSyncClientImage = fmt.Sprintf("%s/%s", imageRegistry, containerImage)
	log.Info("Using NatsSyncClient image", "image", natsSyncClientImage)
	natsSyncCloudBridgeURL := register.GetAstraHostURL(m)
	keyStoreURLSplit := strings.Split(common.NatsSyncClientKeystoreUrl, "://")
	if len(keyStoreURLSplit) < 2 {
		return nil, nil, errors.New("invalid keyStoreURLSplit provided, format - configmap:///configmap-data")
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NatsSyncClientName,
			Namespace: m.Namespace,
			Annotations: map[string]string{
				"container.seccomp.security.alpha.kubernetes.io/pod": "runtime/default",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &m.Spec.NatsSyncClient.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: natsSyncClientImage,
						Name:  common.NatsSyncClientName,
						Env: []corev1.EnvVar{
							{
								Name:  "NATS_SERVER_URL",
								Value: GetNatsURL(m),
							},
							{
								Name:  "CLOUD_BRIDGE_URL",
								Value: natsSyncCloudBridgeURL,
							},
							{
								Name:  "CONFIGMAP_NAME",
								Value: common.NatsSyncClientConfigMapName,
							},
							{
								Name:  "POD_NAMESPACE",
								Value: m.Namespace,
							},
							{
								Name:  "KEYSTORE_URL",
								Value: common.NatsSyncClientKeystoreUrl,
							},
							{
								Name:  "SKIP_TLS_VALIDATION",
								Value: strconv.FormatBool(m.Spec.Astra.SkipTLSValidation),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      common.NatsSyncClientConfigMapVolumeName,
								MountPath: keyStoreURLSplit[1],
							},
						},
						SecurityContext: conf.GetSecurityContext(),
					}},
					ServiceAccountName: common.NatsSyncClientConfigMapServiceAccountName,
					Volumes: []corev1.Volume{
						{
							Name: common.NatsSyncClientConfigMapVolumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: common.NatsSyncClientConfigMapName,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if m.Spec.NatsSyncClient.HostAliasIP != "" {
		hostNamesSplit := strings.Split(natsSyncCloudBridgeURL, "://")
		if len(hostNamesSplit) < 2 {
			return nil, nil, errors.New("invalid hostname provided, hostname format - https://hostname")
		}
		dep.Spec.Template.Spec.HostAliases = []corev1.HostAlias{
			{
				IP:        m.Spec.NatsSyncClient.HostAliasIP,
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
	return []client.Object{dep}, model.NonMutateFn, nil
}

// GetServiceObjects returns a NatsSyncClient Service object
func (d *NatsSyncClientDeployer) GetServiceObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NatsSyncClientName,
			Namespace: m.Namespace,
			Labels: map[string]string{
				"app": common.NatsSyncClientName,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Port:     common.NatsSyncClientPort,
					Protocol: common.NatsSyncClientProtocol,
				},
			},
			Selector: map[string]string{
				"app": common.NatsSyncClientName,
			},
		},
	}
	return []client.Object{service}, model.NonMutateFn, nil
}

// LabelsForNatsSyncClient returns the labels for selecting the NatsSyncClient
func LabelsForNatsSyncClient(name string, mLabels map[string]string) map[string]string {
	labels := map[string]string{"app": name}
	maps.Copy(labels, mLabels)
	return labels
}

// GetConfigMapObjects returns a ConfigMap object for NatsSyncClient
func (d *NatsSyncClientDeployer) GetConfigMapObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: m.Namespace,
			Name:      common.NatsSyncClientConfigMapName,
		},
	}
	return []client.Object{configMap}, model.NonMutateFn, nil
}

// GetRoleObjects returns a ConfigMapRole object for NatsSyncClient
func (d *NatsSyncClientDeployer) GetRoleObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error) {
	configMapRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: m.Namespace,
			Name:      common.NatsSyncClientConfigMapRoleName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"get", "list", "patch"},
			},
			{
				APIGroups: []string{"security.openshift.io"},
				Resources: []string{"securitycontextconstraints"},
				Verbs:     []string{"use"},
			},
		},
	}
	return []client.Object{configMapRole}, model.NonMutateFn, nil
}

// GetRoleBindingObjects returns a NatsSyncClient ConfigMapRoleBinding object
func (d *NatsSyncClientDeployer) GetRoleBindingObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error) {
	configMapRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: m.Namespace,
			Name:      common.NatsSyncClientConfigMapRoleBindingName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: common.NatsSyncClientConfigMapServiceAccountName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     common.NatsSyncClientConfigMapRoleName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	return []client.Object{configMapRoleBinding}, model.NonMutateFn, nil
}

// GetServiceAccountObjects returns a ServiceAccount object for NatsSyncClient
func (d *NatsSyncClientDeployer) GetServiceAccountObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NatsSyncClientConfigMapServiceAccountName,
			Namespace: m.Namespace,
		},
	}
	return []client.Object{sa}, model.NonMutateFn, nil
}

func (d *NatsSyncClientDeployer) GetStatefulSetObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error) {
	return nil, model.NonMutateFn, nil
}

func (d *NatsSyncClientDeployer) GetClusterRoleObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error) {
	return nil, model.NonMutateFn, nil
}

func (d *NatsSyncClientDeployer) GetClusterRoleBindingObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error) {
	return nil, model.NonMutateFn, nil
}
