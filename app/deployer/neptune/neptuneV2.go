/*
 * Copyright (c) 2023. NetApp, Inc. All Rights Reserved.
 */

package neptune

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/model"
	"github.com/NetApp-Polaris/astra-connector-operator/common"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

type NeptuneClientDeployerV2 struct{}

func NewNeptuneClientDeployerV2() model.Deployer {
	return &NeptuneClientDeployerV2{}
}

func (n NeptuneClientDeployerV2) GetDeploymentObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	var deps []client.Object
	log := ctrllog.FromContext(ctx)

	var imageRegistry string
	var containerImage string
	var neptuneImage string
	if m.Spec.ImageRegistry.Name != "" {
		imageRegistry = m.Spec.ImageRegistry.Name
	} else {
		imageRegistry = common.DefaultImageRegistry
	}

	if m.Spec.Neptune.Image != "" {
		containerImage = m.Spec.Neptune.Image
	} else {
		// Reading env variable for project root. This is to ensure that we can read this file in both test
		//	and production environments. This variable will be set in test, and will be ignored for the app
		//  running in docker.
		rootDir := os.Getenv("PROJECT_ROOT")
		if rootDir == "" {
			rootDir = "."
		}
		filePath := filepath.Join(rootDir, "common/neptune_manager_tag.txt")
		imageBytes, err := os.ReadFile(filePath)
		if err != nil {
			return nil, errors.Wrap(err, "error reading neptune manager tag")
		}

		containerImage = string(imageBytes)
		containerImage = strings.TrimSpace(containerImage)
	}

	neptuneImage = fmt.Sprintf("%s/controller:%s", imageRegistry, containerImage)
	log.Info("Using Neptune image", "image", neptuneImage)

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
							Image: neptuneImage,
							Env:   getNeptuneEnvVars(imageRegistry, containerImage, m.Spec.ImageRegistry.Secret),
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

func getNeptuneEnvVars(imageRegistry, containerImage, pullSecret string) []corev1.EnvVar {
	var envVars []corev1.EnvVar

	//DefaultImageRegistry := "netappdownloads.jfrog.io/docker-astra-control-staging/arch30/neptune"
	// would look like this after split
	// NEPTUNE_REGISTRY = netappdownloads.jfrog.io
	// NEPTUNE_REPOSITORY = docker-astra-control-staging/arch30/neptune
	splitImageReg := strings.SplitN(imageRegistry, "/", 2)
	splitImageName := strings.SplitN(containerImage, ":", 2)

	if len(splitImageReg) < 2 {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "NEPTUNE_REGISTRY",
			Value: imageRegistry,
		})
	} else {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "NEPTUNE_REGISTRY",
			Value: splitImageReg[0],
		})

		envVars = append(envVars, corev1.EnvVar{
			Name:  "NEPTUNE_REPOSITORY",
			Value: splitImageReg[1],
		})
	}

	if len(splitImageName) == 2 {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "NEPTUNE_TAG",
			Value: splitImageName[1],
		})
	}

	if pullSecret != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "NEPTUNE_SECRET",
			Value: pullSecret,
		})
	}

	return envVars
}

func (n NeptuneClientDeployerV2) GetStatefulSetObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}

func (n NeptuneClientDeployerV2) GetServiceObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}

func (n NeptuneClientDeployerV2) GetConfigMapObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}

func (n NeptuneClientDeployerV2) GetServiceAccountObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}

func (n NeptuneClientDeployerV2) GetRoleObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}

func (n NeptuneClientDeployerV2) GetClusterRoleObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}

func (n NeptuneClientDeployerV2) GetRoleBindingObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}

func (n NeptuneClientDeployerV2) GetClusterRoleBindingObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}
