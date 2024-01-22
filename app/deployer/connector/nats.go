/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package connector

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/model"

	"github.com/NetApp-Polaris/astra-connector-operator/common"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

type NatsDeployer struct{}

func NewNatsDeployer() model.Deployer {
	return &NatsDeployer{}
}

// GetStatefulSetObjects returns a NATS Statefulset object
func (n *NatsDeployer) GetStatefulSetObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	log := ctrllog.FromContext(ctx)
	ls := labelsForNats(common.NatsName, m.Spec.Labels)

	var replicas int32
	if m.Spec.Nats.Replicas > 2 {
		replicas = m.Spec.Nats.Replicas
	} else {
		log.Info("Defaulting the Nats replica size", "size", common.NatsDefaultReplicas)
		replicas = common.NatsDefaultReplicas
	}

	var natsImage string
	var imageRegistry string
	var containerImage string
	if m.Spec.ImageRegistry.Name != "" {
		imageRegistry = m.Spec.ImageRegistry.Name
	} else {
		imageRegistry = common.DefaultImageRegistry
	}

	if m.Spec.Nats.Image != "" {
		containerImage = m.Spec.Nats.Image
	} else {
		containerImage = common.NatsDefaultImage
	}

	natsImage = fmt.Sprintf("%s/%s", imageRegistry, containerImage)
	log.Info("Using nats image", "image", natsImage)

	// High UID to satisfy OCP requirements
	userUID := int64(1000740000)
	readOnlyRootFilesystem := true
	runAsNonRoot := true
	dep := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NatsName,
			Namespace: m.Namespace,
			Annotations: map[string]string{
				"container.seccomp.security.alpha.kubernetes.io/pod": "runtime/default",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: common.NatsClusterServiceName,
			Replicas:    &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: common.NatsServiceAccountName,
					Containers: []corev1.Container{{
						Image: natsImage,
						Name:  common.NatsName,
						Ports: []corev1.ContainerPort{
							{
								Name:          "client",
								ContainerPort: common.NatsClientPort,
							},
							{
								Name:          "cluster",
								ContainerPort: common.NatsClusterPort,
							},
							{
								Name:          "monitor",
								ContainerPort: common.NatsMonitorPort,
							},
							{
								Name:          "metrics",
								ContainerPort: common.NatsMetricsPort,
							},
						},
						Command: []string{"nats-server", "--config", "/etc/nats-config/nats.conf"},
						Env: []corev1.EnvVar{
							{
								Name:  "CLUSTER_ADVERTISE",
								Value: fmt.Sprintf("%s.nats.%s.svc", common.NatsName, m.Namespace),
							},
							{
								Name:  "POD_NAME",
								Value: common.NatsName,
							}, {
								Name:  "POD_NAMESPACE",
								Value: m.Namespace,
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      common.NatsVolumeName,
								MountPath: "/etc/nats-config",
							},
							{
								Name:      "pid",
								MountPath: "/var/run/nats",
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
					Volumes: []corev1.Volume{
						{
							Name: common.NatsVolumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: common.NatsConfigMapName,
									},
								},
							},
						},
						{
							Name: "pid",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									Medium: "",
								},
							},
						},
					},
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

// GetConfigMapObjects returns a ConfigMap object for nats
func (n *NatsDeployer) GetConfigMapObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	log := ctrllog.FromContext(ctx)

	routes := make([]string, 0)
	var index int32
	index = 0
	var replicas int32

	// Setting the replicas to 1, things dont work with multiple replicas on GKE
	// Uncomment once issue is fixed.
	//if m.Spec.Nats.Replicas > 2 {
	//	replicas = m.Spec.Nats.Replicas
	//} else {
	//	log.Info("Defaulting the Nats replica size", "size", common.NatsDefaultReplicas)
	//	replicas = common.NatsDefaultReplicas
	//}

	log.Info("Defaulting the Nats replica size", "size", common.NatsDefaultReplicas)
	replicas = common.NatsDefaultReplicas

	for index < replicas {
		rt := fmt.Sprintf("\n    nats://nats-%d.nats-cluster:%d", index, common.NatsClusterPort)
		routes = append(routes, rt)
		index += 1
	}
	routes[len(routes)-1] += "\n    "
	routeConfig := strings.Join(routes, "")

	natsConf := "pid_file: \"/var/run/nats/nats.pid\"\nhttp: %d\nmax_payload: %d\n\ncluster {\n  port: %d\n  routes [%s]\n\n  cluster_advertise: $CLUSTER_ADVERTISE\n  connect_retries: 30\n}\n"
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: m.Namespace,
			Name:      common.NatsConfigMapName,
		},
		Data: map[string]string{
			"nats.conf": fmt.Sprintf(natsConf, common.NatsMonitorPort, common.NatsMaxPayload, common.NatsClusterPort, routeConfig),
		},
	}
	return []client.Object{configMap}, nil
}

// GetServiceAccountObjects returns a ServiceAccount object for nats
func (n *NatsDeployer) GetServiceAccountObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NatsServiceAccountName,
			Namespace: m.Namespace,
			Labels:    labelsForNats(common.NatsName, m.Spec.Labels),
		},
	}
	return []client.Object{sa}, nil
}

// GetServiceObjects returns a Service object for nats
func (n *NatsDeployer) GetServiceObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	ls := labelsForNats(common.NatsName, m.Spec.Labels)
	var services []client.Object

	natsService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NatsName,
			Namespace: m.Namespace,
			Labels:    ls,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name: common.NatsName,
					Port: common.NatsClientPort,
				},
			},
			Selector: ls,
		},
	}
	natsClusterService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NatsClusterServiceName,
			Namespace: m.Namespace,
			Labels:    ls,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "",
			Ports: []corev1.ServicePort{
				{
					Name: "client",
					Port: common.NatsClientPort,
				},
				{
					Name: "cluster",
					Port: common.NatsClusterPort,
				},
				{
					Name: "monitor",
					Port: common.NatsMonitorPort,
				},
				{
					Name: "metrics",
					Port: common.NatsMetricsPort,
				},
				{
					Name: "gateways",
					Port: common.NatsGatewaysPort,
				},
			},
			Selector: ls,
		},
	}
	services = append(services, natsService, natsClusterService)
	return services, nil
}

// labelsForNats returns the labels for selecting the nats resources
func labelsForNats(name string, mLabels map[string]string) map[string]string {
	labels := map[string]string{"app": name}
	maps.Copy(labels, mLabels)
	return labels
}

func (n *NatsDeployer) GetDeploymentObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}

func (n *NatsDeployer) GetRoleObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}

func (n *NatsDeployer) GetRoleBindingObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}

func (n *NatsDeployer) GetClusterRoleObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}

func (n *NatsDeployer) GetClusterRoleBindingObjects(m *v1.AstraConnector, ctx context.Context) ([]client.Object, error) {
	return nil, nil
}

// GetNatsURL returns the nats URL
func GetNatsURL(m *v1.AstraConnector) string {
	natsURL := fmt.Sprintf("nats://%s.%s:%d", common.NatsName, m.Namespace, common.NatsClientPort)
	return natsURL
}
