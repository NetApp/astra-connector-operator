package neptune

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/model"
	"github.com/NetApp-Polaris/astra-connector-operator/common"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

type NeptuneClientDeployer struct{}

func NewNeptuneClientDeployer() model.Deployer {
	return &NeptuneClientDeployer{}
}

func (n NeptuneClientDeployer) GetDeploymentObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	var deps []client.Object

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NeptuneName,
			Namespace: m.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/component":  "manager",
				"app.kubernetes.io/created-by": "neptune",
				"app.kubernetes.io/instance":   "controller-manager",
				"app.kubernetes.io/managed-by": "kustomize",
				"app.kubernetes.io/name":       "deployment",
				"app.kubernetes.io/part-of":    "neptune",
				"control-plane":                "controller-manager",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"control-plane": "controller-manager",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"kubectl.kubernetes.io/default-container": "manager",
					},
					Labels: map[string]string{
						"control-plane": "controller-manager",
					},
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/arch",
												Operator: corev1.NodeSelectorOpIn,
												Values: []string{
													"amd64",
													"arm64",
													"ppc64le",
													"s390x",
												},
											},
											{
												Key:      "kubernetes.io/os",
												Operator: corev1.NodeSelectorOpIn,
												Values: []string{
													"linux",
												},
											},
										},
									},
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Args: []string{
								"--secure-listen-address=0.0.0.0:8443",
								"--upstream=http://127.0.0.1:8080/",
								"--logtostderr=true",
								"--v=0",
							},
							Image: "gcr.io/kubebuilder/kube-rbac-proxy:v0.13.1",
							Name:  "kube-rbac-proxy",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8443,
									Name:          "https",
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("5m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: pointer.Bool(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{
										"ALL",
									},
								},
							},
						},
						{
							Args: []string{
								"--health-probe-bind-address=:8081",
								"--metrics-bind-address=127.0.0.1:8080",
								"--leader-elect",
							},
							Command: []string{
								"/manager",
							},
							Image: "controller:latest",
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt(8081),
									},
								},
								InitialDelaySeconds: 15,
								PeriodSeconds:       20,
							},
							Name: "manager",
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/readyz",
										Port: intstr.FromInt(8081),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: pointer.Bool(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{
										"ALL",
									},
								},
							},
						},
					},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: pointer.Bool(true),
					},
					ServiceAccountName:            "neptune-controller-manager",
					TerminationGracePeriodSeconds: pointer.Int64(10),
				},
			},
		},
	}

	if m.Spec.ImageRegistry.Secret != "" {
		deployment.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
			{
				Name: m.Spec.ImageRegistry.Secret,
			},
		}
	}

	deps = append(deps, deployment)
	return deps, nil
}

func (n NeptuneClientDeployer) GetStatefulSetObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}

func (n NeptuneClientDeployer) GetServiceObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	var services []client.Object

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "neptune-controller-manager-metrics-service",
			Namespace: m.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/component":  "kube-rbac-proxy",
				"app.kubernetes.io/created-by": "neptune",
				"app.kubernetes.io/instance":   "controller-manager-metrics-service",
				"app.kubernetes.io/managed-by": "kustomize",
				"app.kubernetes.io/name":       "service",
				"app.kubernetes.io/part-of":    "neptune",
				"control-plane":                "controller-manager",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "https",
					Port:       common.NeptuneMetricServicePort,
					Protocol:   common.NeptuneMetricServiceProtocol,
					TargetPort: intstr.FromString("https"),
				},
			},
			Selector: map[string]string{
				"control-plane": "controller-manager",
			},
		},
	}

	services = append(services, service)
	return services, nil
}

func (n NeptuneClientDeployer) GetConfigMapObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}

func (n NeptuneClientDeployer) GetServiceAccountObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NeptuneName,
			Namespace: m.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/component":  "rbac",
				"app.kubernetes.io/created-by": "neptune",
				"app.kubernetes.io/instance":   "leader-election-role",
				"app.kubernetes.io/managed-by": "kustomize",
				"app.kubernetes.io/name":       "serviceaccount",
				"app.kubernetes.io/part-of":    "neptune",
			},
		},
	}
	return []client.Object{sa}, nil
}

func (n NeptuneClientDeployer) GetRoleObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	var rs []client.Object
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: m.Namespace,
			Name:      common.NeptuneLeaderElectionRoleName,
			Labels: map[string]string{
				"app.kubernetes.io/component":  "rbac",
				"app.kubernetes.io/created-by": "neptune",
				"app.kubernetes.io/instance":   "leader-election-role",
				"app.kubernetes.io/managed-by": "kustomize",
				"app.kubernetes.io/name":       "role",
				"app.kubernetes.io/part-of":    "neptune",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"create", "patch"},
			},
		},
	}

	rs = append(rs, role)
	return rs, nil
}

func (n NeptuneClientDeployer) GetClusterRoleObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	var clusterRoles []client.Object

	neptuneClusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: common.NeptuneClusterRoleName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"create", "get", "list", "patch", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumeclaims"},
				Verbs:     []string{"create", "get", "list", "patch", "watch"},
			},
			{
				APIGroups: []string{"batch"},
				Resources: []string{"jobs"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"batch"},
				Resources: []string{"jobs/finalizers"},
				Verbs:     []string{"create", "update"},
			},
			{
				APIGroups: []string{"batch"},
				Resources: []string{"jobs/status"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"applications"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"applications/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"applications/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"appvaults"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"appvaults/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"appvaults/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"autosupportbundles"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"autosupportbundles/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"autosupportbundles/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"backupinplacerestores"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"backupinplacerestores/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"backupinplacerestores/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"backuprestores"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"backuprestores/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"backuprestores/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"backups"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"backups/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"backups/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"disruptivebackups"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"disruptivebackups/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"disruptivebackups/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"exechooks"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"exechooks/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"exechooks/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"exechooksruns"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"exechooksruns/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"exechooksruns/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"policies"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"policies/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"policies/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"pvccopies"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"pvccopies/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"pvccopies/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"pvcerases"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"pvcerases/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"pvcerases/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"resourcebackups"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"resourcebackups/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"resourcebackups/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"resourcedeletes"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"resourcedeletes/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"resourcedeletes/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"resourcerestores"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"resourcerestores/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"resourcerestores/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"resticvolumebackups"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"resticvolumebackups/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"resticvolumebackups/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"resticvolumerestores"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"resticvolumerestores/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"resticvolumerestores/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"schedules"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"schedules/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"schedules/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"snapshotrestores"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"snapshotrestores/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"snapshotrestores/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"snapshots"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"snapshots/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"management.astra.netapp.io"},
				Resources: []string{"snapshots/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"snapshot.storage.k8s.io"},
				Resources: []string{"volumesnapshotcontents"},
				Verbs:     []string{"create", "get", "list", "patch", "watch"},
			},
			{
				APIGroups: []string{"snapshot.storage.k8s.io"},
				Resources: []string{"volumesnapshots"},
				Verbs:     []string{"create", "get", "list", "patch", "watch"},
			},
		},
	}

	neptuneMetricsReaderCR := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "neptune-metrics-reader",
			Labels: map[string]string{
				"app.kubernetes.io/component":  "kube-rbac-proxy",
				"app.kubernetes.io/created-by": "neptune",
				"app.kubernetes.io/instance":   "metrics-reader",
				"app.kubernetes.io/managed-by": "kustomize",
				"app.kubernetes.io/name":       "clusterrole",
				"app.kubernetes.io/part-of":    "neptune",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				NonResourceURLs: []string{"/metrics"},
				Verbs:           []string{"get"},
			},
		},
	}

	neptuneProxyCR := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "neptune-proxy-role",
			Labels: map[string]string{
				"app.kubernetes.io/component":  "kube-rbac-proxy",
				"app.kubernetes.io/created-by": "neptune",
				"app.kubernetes.io/instance":   "proxy-role",
				"app.kubernetes.io/managed-by": "kustomize",
				"app.kubernetes.io/name":       "clusterrole",
				"app.kubernetes.io/part-of":    "neptune",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"authentication.k8s.io"},
				Resources: []string{"tokenreviews"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups: []string{"authorization.k8s.io"},
				Resources: []string{"subjectaccessreviews"},
				Verbs:     []string{"create"},
			},
		},
	}

	clusterRoles = append(clusterRoles, neptuneClusterRole, neptuneMetricsReaderCR, neptuneProxyCR)

	return clusterRoles, nil
}

func (n NeptuneClientDeployer) GetRoleBindingObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	configMapRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: m.Namespace,
			Name:      common.NeptuneLeaderElectionRoleBindingName,
			Labels: map[string]string{
				"app.kubernetes.io/component":  "rbac",
				"app.kubernetes.io/created-by": "neptune",
				"app.kubernetes.io/instance":   "leader-election-rolebinding",
				"app.kubernetes.io/managed-by": "kustomize",
				"app.kubernetes.io/name":       "rolebinding",
				"app.kubernetes.io/part-of":    "neptune",
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: common.NeptuneName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     common.NeptuneLeaderElectionRoleName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	return []client.Object{configMapRoleBinding}, nil
}

func (n NeptuneClientDeployer) GetClusterRoleBindingObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	var crb []client.Object

	neptuneManagerRB := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "neptune-manager-rolebinding",
			Labels: map[string]string{
				"app.kubernetes.io/component":  "rbac",
				"app.kubernetes.io/created-by": "neptune",
				"app.kubernetes.io/instance":   "manager-rolebinding",
				"app.kubernetes.io/managed-by": "kustomize",
				"app.kubernetes.io/name":       "clusterrolebinding",
				"app.kubernetes.io/part-of":    "neptune",
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      common.NeptuneName,
				Namespace: m.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     common.NeptuneClusterRoleName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	neptuneProxyRB := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "neptune-proxy-rolebinding",
			Labels: map[string]string{
				"app.kubernetes.io/component":  "kube-rbac-proxy",
				"app.kubernetes.io/created-by": "neptune",
				"app.kubernetes.io/instance":   "proxy-rolebinding",
				"app.kubernetes.io/managed-by": "kustomize",
				"app.kubernetes.io/name":       "clusterrolebinding",
				"app.kubernetes.io/part-of":    "neptune",
				"control-plane":                "controller-manager",
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      common.NeptuneName,
				Namespace: m.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "neptune-proxy-role",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	crb = append(crb, neptuneManagerRB, neptuneProxyRB)
	return crb, nil
}
