package controllers

import (
	"fmt"
	cachev1 "github.com/NetApp/astraagent-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// StatefulsetForNats returns a astraAgent Deployment object
func (r *AstraAgentReconciler) StatefulsetForNats(m *cachev1.AstraAgent) *appsv1.StatefulSet {
	ls := labelsForNats(m.Spec.Nats.Name)
	replicas := m.Spec.Nats.Size

	dep := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Spec.Nats.Name,
			Namespace: m.Spec.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
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
						Image: m.Spec.Nats.Image,
						Name:  m.Spec.Nats.Name,
						Ports: []corev1.ContainerPort{
							{
								Name:          "client",
								ContainerPort: m.Spec.Nats.ClientPort,
							},
							{
								Name:          "cluster",
								ContainerPort: m.Spec.Nats.ClusterPort,
							},
							{
								Name:          "monitor",
								ContainerPort: m.Spec.Nats.MonitorPort,
							},
							{
								Name:          "metrics",
								ContainerPort: m.Spec.Nats.MetricsPort,
							},
						},
					}},
				},
			},
		},
	}
	// Set astraAgent instance as the owner and controller
	ctrl.SetControllerReference(m, dep, r.Scheme)
	return dep
}

// ClusterServiceForNats returns a astraAgent Deployment object
func (r *AstraAgentReconciler) ClusterServiceForNats(m *cachev1.AstraAgent) *corev1.Service {
	ls := labelsForNats(m.Spec.Nats.Name)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Spec.Nats.ClusterServiceName,
			Namespace: m.Spec.Namespace,
			Labels:    ls,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "",
			Ports: []corev1.ServicePort{
				{
					Name: "client",
					Port: m.Spec.Nats.ClientPort,
				},
				{
					Name: "cluster",
					Port: m.Spec.Nats.ClusterPort,
				},
				{
					Name: "monitor",
					Port: m.Spec.Nats.MonitorPort,
				},
				{
					Name: "metrics",
					Port: m.Spec.Nats.MetricsPort,
				},
				{
					Name: "gateways",
					Port: m.Spec.Nats.GatewaysPort,
				},
			},
			Selector: ls,
		},
	}
	// Set astraAgent instance as the owner and controller
	ctrl.SetControllerReference(m, service, r.Scheme)
	return service
}

// ServiceForNats returns a astraAgent Deployment object
func (r *AstraAgentReconciler) ServiceForNats(m *cachev1.AstraAgent) *corev1.Service {
	ls := labelsForNats(m.Spec.Nats.Name)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Spec.Nats.Name,
			Namespace: m.Spec.Namespace,
			Labels:    ls,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name: m.Spec.Nats.Name,
					Port: m.Spec.Nats.ClientPort,
				},
			},
			Selector: ls,
		},
	}
	// Set astraAgent instance as the owner and controller
	ctrl.SetControllerReference(m, service, r.Scheme)
	return service
}

// labelsForNatssyncClient returns the labels for selecting the resources
// belonging to the given astraAgent CR name.
func labelsForNats(name string) map[string]string {
	return map[string]string{"app": name}
}

// GetNatsURL returns a astraAgent Deployment object
func (r *AstraAgentReconciler) GetNatsURL(m *cachev1.AstraAgent) string {
	natsURL := fmt.Sprintf("nats://%s.%s:%d", m.Spec.Nats.Name, m.Spec.Namespace, m.Spec.Nats.ClientPort)
	return natsURL
}
