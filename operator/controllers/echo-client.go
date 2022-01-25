package controllers

import (
	cachev1 "github.com/NetApp/astraagent-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// DeploymentForEchoClient returns a astraAgent Deployment object
func (r *AstraAgentReconciler) DeploymentForEchoClient(m *cachev1.AstraAgent) *appsv1.Deployment {
	ls := labelsForEchoClient(m.Spec.EchoClient.Name)
	replicas := m.Spec.EchoClient.Size

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Spec.EchoClient.Name,
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
						Image: m.Spec.EchoClient.Image,
						Name:  m.Spec.EchoClient.Name,
						Env: []corev1.EnvVar{
							{
								Name:  "NATS_SERVER_URL",
								Value: r.GetNatsURL(m),
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
func labelsForEchoClient(name string) map[string]string {
	return map[string]string{"app": name}
}
