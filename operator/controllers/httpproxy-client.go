package controllers

import (
	cachev1 "github.com/NetApp/astraagent-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// DeploymentForProxyClient returns a astraAgent Deployment object
func (r *AstraAgentReconciler) DeploymentForProxyClient(m *cachev1.AstraAgent) *appsv1.Deployment {
	ls := labelsForProxyClient(m.Spec.HttpProxyClient.Name)
	replicas := m.Spec.HttpProxyClient.Size

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Spec.HttpProxyClient.Name,
			Namespace: m.Spec.Namespace,
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
						Image: m.Spec.HttpProxyClient.Image,
						Name:  m.Spec.HttpProxyClient.Name,
						Env: []corev1.EnvVar{
							{
								Name:  "NATS_SERVER_URL",
								Value: m.Spec.Nats.ServerURL,
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

// labelsForProxyClient returns the labels for selecting the resources
// belonging to the given astraAgent CR name.
func labelsForProxyClient(name string) map[string]string {
	return map[string]string{"app": name}
}
