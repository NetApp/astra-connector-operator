/*
 * Copyright (c) 2024. NetApp, Inc. All Rights Reserved.
 */

package neptune

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/NetApp-Polaris/astra-connector-operator/app/conf"
	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/model"
	"github.com/NetApp-Polaris/astra-connector-operator/common"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

type NeptuneClientDeployerV2 struct{}

func NewNeptuneClientDeployerV2() model.Deployer {
	return &NeptuneClientDeployerV2{}
}

func (n NeptuneClientDeployerV2) GetDeploymentObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error) {
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
		containerImage = common.NeptuneImageTag
	}

	neptuneImage = fmt.Sprintf("%s/controller:%s", imageRegistry, containerImage)
	rbacProxyImage := fmt.Sprintf("%s/kube-rbac-proxy:v0.14.1", imageRegistry)
	log.Info("Using Neptune image", "image", neptuneImage)

	deploymentLabels := map[string]string{
		"app.kubernetes.io/component":  "manager",
		"app.kubernetes.io/created-by": "neptune",
		"app.kubernetes.io/instance":   "controller-manager",
		"app.kubernetes.io/managed-by": "kustomize",
		"app.kubernetes.io/name":       "deployment",
		"app.kubernetes.io/part-of":    "neptune",
		"control-plane":                "controller-manager",
	}
	// add any labels user wants to use or override
	maps.Copy(deploymentLabels, m.Spec.Labels)

	podLabels := map[string]string{
		"control-plane": "controller-manager",
		"app":           "controller.neptune.netapp.io",
	}
	maps.Copy(podLabels, m.Spec.Labels)
	neptuneReplicas := int32(common.NeptuneReplicas)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NeptuneName,
			Namespace: m.Namespace,
			Labels:    deploymentLabels,
			Annotations: map[string]string{
				"container.seccomp.security.alpha.kubernetes.io/pod": "runtime/default",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Replicas: &neptuneReplicas,
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
					Labels: podLabels,
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
							Image: rbacProxyImage,
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
									corev1.ResourceCPU:    resource.MustParse("1000m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: pointer.Bool(false),
							},
						},
						{
							Args: []string{
								"--health-probe-bind-address=:8081",
								"--metrics-bind-address=127.0.0.1:8080",
							},
							Command: []string{
								"/manager",
							},
							Image: neptuneImage,
							Env: getNeptuneEnvVars(
								imageRegistry,
								containerImage,
								m.Spec.Neptune.JobImagePullPolicy,
								m.Spec.ImageRegistry.Secret,
								m.Spec.AutoSupport.URL,
								m.Spec.Labels,
							),
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
									corev1.ResourceCPU:    resource.MustParse("1000m"),
									corev1.ResourceMemory: resource.MustParse("1280Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("640Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: pointer.Bool(false),
								ReadOnlyRootFilesystem:   pointer.Bool(true),
							},
						},
					},
					SecurityContext:               conf.GetPodSecurityContext(),
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

	mutateFunc := func() error {
		// Get the containers
		containers := deployment.Spec.Template.Spec.Containers
		newContainers := make([]corev1.Container, 0, 2)

		for i, container := range containers {
			if container.Name == "manager" {
				for j, envVar := range container.Env {
					if envVar.Name == "NEPTUNE_AUTOSUPPORT_URL" {
						containers[i].Env[j].Value = m.Spec.AutoSupport.URL
					}
				}
				container.Image = neptuneImage
			}
			newContainers = append(newContainers, container)
		}

		// Update the containers in the deployment
		deployment.Spec.Template.Spec.Containers = newContainers
		return nil
	}

	return deps, mutateFunc, nil
}

func getNeptuneEnvVars(imageRegistry, containerImage, jobImagePullPolicy, pullSecret, asupUrl string, mLabels map[string]string) []corev1.EnvVar {
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
	} else {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "NEPTUNE_TAG",
			Value: containerImage,
		})
	}

	if pullSecret != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "NEPTUNE_SECRET",
			Value: pullSecret,
		})
	}

	if jobImagePullPolicy != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "JOB_IMAGE_PULL_POLICY",
			Value: jobImagePullPolicy,
		})
	}

	if asupUrl != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "NEPTUNE_AUTOSUPPORT_URL",
			Value: asupUrl,
		})
	}

	if mLabels != nil {
		jsonData, err := json.Marshal(mLabels)
		if err != nil {
			log.Fatalf("JSON marshaling (LABELS) failed: %s", err)
		} else {
			jsonString := string(jsonData)
			envVars = append(envVars, corev1.EnvVar{
				Name:  "NEPTUNE_LABELS",
				Value: jsonString,
			})
		}
	}

	return envVars
}

func (n NeptuneClientDeployerV2) GetStatefulSetObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error) {
	return nil, model.NonMutateFn, nil
}

func (n NeptuneClientDeployerV2) GetServiceObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error) {
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

	return services, model.NonMutateFn, nil
}

func (n NeptuneClientDeployerV2) GetConfigMapObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error) {
	return nil, model.NonMutateFn, nil
}

func (n NeptuneClientDeployerV2) GetServiceAccountObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NeptuneName,
			Namespace: m.Namespace,
		},
	}
	return []client.Object{sa}, model.NonMutateFn, nil
}

func (n NeptuneClientDeployerV2) GetRoleObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error) {
	return nil, model.NonMutateFn, nil
}

func (n NeptuneClientDeployerV2) GetClusterRoleObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error) {
	return nil, model.NonMutateFn, nil
}

func (n NeptuneClientDeployerV2) GetRoleBindingObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error) {
	return nil, model.NonMutateFn, nil
}

func (n NeptuneClientDeployerV2) GetClusterRoleBindingObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, controllerutil.MutateFn, error) {
	return nil, model.NonMutateFn, nil
}
