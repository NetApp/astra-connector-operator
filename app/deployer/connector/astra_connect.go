/*
 * Copyright (c) 2023. NetApp, Inc. All Rights Reserved.
 */

package connector

import (
	"context"
	"fmt"
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
	ls := LabelsForAstraConnectClient(common.AstraConnectName, ad)

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

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.AstraConnectName,
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
func LabelsForAstraConnectClient(name string, additionalLabels map[string]string) map[string]string {
	labels := map[string]string{"type": name, "role": name}
	for k, v := range additionalLabels {
		labels[k] = v
	}
	return labels
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
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{
					"",
					"apiextensions.k8s.io",
					"apps",
					"autoscaling",
					"batch",
					"crd.projectcalico.org",
					"extensions",
					"networking.k8s.io",
					"policy",
					"rbac.authorization.k8s.io",
					"security.openshift.io",
					"snapshot.storage.k8s.io",
					"storage.k8s.io",
					"trident.netapp.io",
				},
				Resources: []string{
					"configmaps",
					"cronjobs",
					"customresourcedefinitions",
					"daemonsets",
					"deployments",
					"horizontalpodautoscalers",
					"ingresses",
					"jobs",
					"namespaces",
					"networkpolicies",
					"persistentvolumeclaims",
					"poddisruptionbudgets",
					"pods",
					"podtemplates",
					"podsecuritypolicies",
					"replicasets",
					"replicationcontrollers",
					"replicationcontrollers/scale",
					"rolebindings",
					"roles",
					"secrets",
					"serviceaccounts",
					"services",
					"statefulsets",
					"storageclasses",
					"csidrivers",
					"csinodes",
					"securitycontextconstraints",
					"tridentmirrorrelationships",
					"tridentsnapshotinfos",
					"tridentvolumes",
					"volumesnapshots",
					"volumesnapshotcontents",
					"tridentversions",
					"tridentbackends",
					"tridentnodes",
				},
				Verbs: []string{"get", "list", "watch", "delete", "use", "create", "update", "patch"},
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

// NIL RESOURCES BELOW

// GetRoleObjects returns a ConfigMapRole object for Astra Connect
func (d *AstraConnectDeployer) GetRoleObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}

// GetRoleBindingObjects returns a ConfigMapRoleBinding object
func (d *AstraConnectDeployer) GetRoleBindingObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}

func (d *AstraConnectDeployer) GetStatefulSetObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}
