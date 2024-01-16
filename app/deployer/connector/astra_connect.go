/*
 * Copyright (c) 2023. NetApp, Inc. All Rights Reserved.
 */

package connector

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/api/resource"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/model"
	"github.com/NetApp-Polaris/astra-connector-operator/common"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

type AstraConnectDeployer struct{}

func NewAstraConnectorDeployer() model.Deployer {
	return &AstraConnectDeployer{}
}

// GetDeploymentObjects returns a Astra Connect Deployment object
func (d *AstraConnectDeployer) GetDeploymentObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	log := ctrllog.FromContext(ctx)
	ls := LabelsForAstraConnectClient(common.AstraConnectName)

	var imageRegistry string
	var containerImage string
	var natssyncClientImage string
	if m.Spec.ImageRegistry.Name != "" {
		imageRegistry = m.Spec.ImageRegistry.Name
	} else {
		imageRegistry = common.DefaultImageRegistry
	}

	if m.Spec.AstraConnect.Image != "" {
		containerImage = m.Spec.AstraConnect.Image
	} else {
		containerImage = common.AstraConnectDefaultImage
	}

	natssyncClientImage = fmt.Sprintf("%s/%s", imageRegistry, containerImage)
	log.Info("Using AstraConnector image", "image", natssyncClientImage)

	// TODO what is appropriate default size
	var replicas int32
	if m.Spec.AstraConnect.Replicas > 1 {
		replicas = m.Spec.AstraConnect.Replicas
	} else {
		log.Info("Defaulting the Astra Connect replica size", "size", common.AstraConnectDefaultReplicas)
		replicas = common.AstraConnectDefaultReplicas
	}

	ref := &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: common.AstraConnectName}, Key: "nats_url"}

	// High UID to satisfy OCP requirements
	userUID := int64(1000740000)
	readOnlyRootFilesystem := true
	runAsNonRoot := true
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.AstraConnectName,
			Namespace: m.Namespace,
			Annotations: map[string]string{
				"container.seccomp.security.alpha.kubernetes.io/pod": "runtime/default",
			},
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
						Name:  common.AstraConnectName,
						Env: []corev1.EnvVar{
							{
								Name:      "NATS_SERVER_URL",
								ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: ref},
							},
							{
								Name:  "LOG_LEVEL",
								Value: "trace",
							},
							{
								Name: "POD_NAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										APIVersion: "v1",
										FieldPath:  "metadata.name",
									},
								},
							},
							{
								Name: "NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										APIVersion: "v1",
										FieldPath:  "metadata.namespace",
									},
								},
							},
						},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("0.1"),
							},
						},
						SecurityContext: &corev1.SecurityContext{
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{"ALL"},
							},
							ReadOnlyRootFilesystem: &readOnlyRootFilesystem,
							RunAsNonRoot:           &runAsNonRoot,
							RunAsUser:              &userUID,
						},
					}},
					ServiceAccountName: common.AstraConnectName,
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
	return []client.Object{dep}, nil
}

// GetServiceObjects returns an Astra-Connect Service object
func (d *AstraConnectDeployer) GetServiceObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}

// LabelsForAstraConnectClient returns the labels for selecting the AstraConnectClient
func LabelsForAstraConnectClient(name string) map[string]string {
	return map[string]string{"type": name, "role": name}
}

// GetConfigMapObjects returns a ConfigMap object for Astra Connect
func (d *AstraConnectDeployer) GetConfigMapObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: m.Namespace,
			Name:      common.AstraConnectName,
		},
		Data: map[string]string{
			"nats_url":            GetNatsURL(m),
			"skip_tls_validation": strconv.FormatBool(m.Spec.Astra.SkipTLSValidation),
		},
	}
	return []client.Object{configMap}, nil
}

// GetServiceAccountObjects returns a ServiceAccount object for Astra Connect
func (d *AstraConnectDeployer) GetServiceAccountObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.AstraConnectName,
			Namespace: m.Namespace,
		},
	}
	return []client.Object{sa}, nil
}

func (d *AstraConnectDeployer) GetClusterRoleObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: common.AstraConnectName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces", "persistentvolumes", "nodes", "pods", "services"},
				Verbs:     []string{"watch", "list", "get"},
			},
			{
				APIGroups: []string{"storage.k8s.io"},
				Resources: []string{"storageclasses"},
				Verbs:     []string{"update", "watch", "list", "get"},
			},
			{
				APIGroups: []string{"storage.k8s.io"},
				Resources: []string{"csidrivers"},
				Verbs:     []string{"watch", "list", "get"},
			},
			{
				APIGroups: []string{"snapshot.storage.k8s.io"},
				Resources: []string{"volumesnapshotclasses"},
				Verbs:     []string{"watch", "list", "get"},
			},
			{
				APIGroups: []string{"trident.netapp.io"},
				Resources: []string{"tridentversions", "tridentorchestrators"},
				Verbs:     []string{"watch", "list", "get"},
			},
		},
	}
	return []client.Object{clusterRole}, nil
}

func (d *AstraConnectDeployer) GetClusterRoleBindingObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: common.AstraConnectName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      common.AstraConnectName,
				Namespace: m.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     common.AstraConnectName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	return []client.Object{clusterRoleBinding}, nil
}

// GetRoleObjects returns a ConfigMapRole object for Astra Connect
func (d *AstraConnectDeployer) GetRoleObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.AstraConnectName,
			Namespace: m.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"create", "update"},
			},
			{
				APIGroups: []string{"astra.netapp.io"},
				Resources: []string{"applications", "appmirrorrelationships", "appmirrorupdates", "appvaults", "autosupportbundles", "backups", "backupinplacerestores", "backuprestores", "exechooks", "exechooksruns", "pvccopies", "pvcerases", "resourcebackups", "resourcedeletes", "resourcerestores", "resourcesummaryuploads", "resticvolumebackups", "resticvolumerestores", "schedules", "snapshotinplacerestores", "snapshotrestores", "snapshots", "astraconnectors"},
				Verbs:     []string{"create", "update", "delete", "watch", "list", "get"},
			},
		},
	}
	return []client.Object{role}, nil
}

// GetRoleBindingObjects returns a ConfigMapRoleBinding object
func (d *AstraConnectDeployer) GetRoleBindingObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.AstraConnectName,
			Namespace: m.Namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      common.AstraConnectName,
				Namespace: m.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     common.AstraConnectName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	return []client.Object{roleBinding}, nil
}

// NIL RESOURCES BELOW
func (d *AstraConnectDeployer) GetStatefulSetObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}
