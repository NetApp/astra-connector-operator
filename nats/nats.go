package nats

import (
	"context"
	"fmt"
	"github.com/NetApp/astraagent-operator/common"

	rbacv1 "k8s.io/api/rbac/v1"

	cachev1 "github.com/NetApp/astraagent-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

type Deployer struct{}

func NewNatsDeployer() *Deployer {
	return &Deployer{}
}

// GetStatefulsetObject returns a NATS Statefulset object
func (n *Deployer) GetStatefulsetObject(m *cachev1.AstraAgent, ctx context.Context) (*appsv1.StatefulSet, error) {
	log := ctrllog.FromContext(ctx)
	ls := labelsForNats(common.NatsName)

	var replicas int32
	if m.Spec.Nats.Size > 2 {
		replicas = m.Spec.Nats.Size
	} else {
		log.Info("Defaulting the Nats replica size", "size", common.NatsDefaultSize)
		replicas = common.NatsDefaultSize
	}

	var natsImage string
	if m.Spec.Nats.Image != "" {
		natsImage = m.Spec.Nats.Image
	} else {
		log.Info("Defaulting the Nats image", "image", common.NatsDefaultImage)
		natsImage = common.NatsDefaultImage
	}

	dep := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NatsName,
			Namespace: m.Namespace,
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
	return dep, nil
}

// GetNatsClusterServiceObject returns a cluster Service object for Nats
func (n *Deployer) GetNatsClusterServiceObject(m *cachev1.AstraAgent) (*corev1.Service, error) {
	ls := labelsForNats(common.NatsName)
	service := &corev1.Service{
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
	return service, nil
}

// GetConfigMapObject returns a ConfigMap object for nats
func (n *Deployer) GetConfigMapObject(m *cachev1.AstraAgent) (*corev1.ConfigMap, error) {
	natsConf := "pid_file: \"/var/run/nats/nats.pid\"\nhttp: %d\n\ncluster {\n  port: %d\n  routes [\n    nats://nats-0.nats-cluster:%d\n    nats://nats-1.nats-cluster:%d\n    nats://nats-2.nats-cluster:%d\n  ]\n\n  cluster_advertise: $CLUSTER_ADVERTISE\n  connect_retries: 30\n}\n"
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: m.Namespace,
			Name:      common.NatsConfigMapName,
		},
		Data: map[string]string{
			"nats.conf": fmt.Sprintf(natsConf, common.NatsMonitorPort, common.NatsClusterPort, common.NatsClusterPort, common.NatsClusterPort, common.NatsClusterPort),
		},
	}
	return configMap, nil
}

// GetServiceAccountObject returns a ServiceAccount object for nats
func (n *Deployer) GetServiceAccountObject(m *cachev1.AstraAgent) (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NatsServiceAccountName,
			Namespace: m.Namespace,
			Labels:    labelsForNats(common.NatsName),
		},
	}
	return sa, nil
}

// GetServiceObject returns a Service object for nats
func (n *Deployer) GetServiceObject(m *cachev1.AstraAgent, serviceName string) (*corev1.Service, error) {
	if serviceName == common.NatsName {
		return n.GetNatsServiceObject(m)
	} else if serviceName == common.NatsClusterServiceName {
		return n.GetNatsClusterServiceObject(m)
	}
	return nil, fmt.Errorf("unknown serviceName: %s", serviceName)
}

// GetNatsServiceObject returns a Service object for nats
func (n *Deployer) GetNatsServiceObject(m *cachev1.AstraAgent) (*corev1.Service, error) {
	ls := labelsForNats(common.NatsName)
	service := &corev1.Service{
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
	return service, nil
}

// labelsForNats returns the labels for selecting the nats resources
func labelsForNats(name string) map[string]string {
	return map[string]string{"app": name}
}

func (n *Deployer) GetDeploymentObject(m *cachev1.AstraAgent, ctx context.Context) (*appsv1.Deployment, error) {
	return nil, fmt.Errorf("not implemented")
}

func (n *Deployer) GetRoleObject(m *cachev1.AstraAgent) (*rbacv1.Role, error) {
	return nil, fmt.Errorf("not implemented")
}

func (n *Deployer) GetRoleBindingObject(m *cachev1.AstraAgent) (*rbacv1.RoleBinding, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetNatsURL returns the nats URL
func GetNatsURL(m *cachev1.AstraAgent) string {
	natsURL := fmt.Sprintf("nats://%s.%s:%d", common.NatsName, m.Namespace, common.NatsClientPort)
	return natsURL
}
