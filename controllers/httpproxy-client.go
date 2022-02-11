package controllers

import (
	"context"

	cachev1 "github.com/NetApp/astraagent-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// DeploymentForProxyClient returns an HttpProxyClient Deployment object
func (r *AstraAgentReconciler) DeploymentForProxyClient(m *cachev1.AstraAgent, ctx context.Context) (*appsv1.Deployment, error) {
	log := ctrllog.FromContext(ctx)
	ls := labelsForProxyClient(HttpProxyClientName)
	replicas := int32(HttpProxyClientsize)

	var httpProxyClientImage string
	if m.Spec.HttpProxyClient.Image != "" {
		httpProxyClientImage = m.Spec.HttpProxyClient.Image
	} else {
		log.Info("Defaulting the HttpProxyClient image", "image", HttpProxyClientDefaultImage)
		httpProxyClientImage = HttpProxyClientDefaultImage
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HttpProxyClientName,
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
						Image: httpProxyClientImage,
						Name:  HttpProxyClientName,
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
	err := ctrl.SetControllerReference(m, dep, r.Scheme)
	if err != nil {
		return nil, err
	}
	return dep, nil
}

// labelsForProxyClient returns the labels for selecting the HttpProxyClient
// belonging to the given astraAgent CR name.
func labelsForProxyClient(name string) map[string]string {
	return map[string]string{"app": name}
}
