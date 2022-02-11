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

// DeploymentForEchoClient returns an EchoClient Deployment object
func (r *AstraAgentReconciler) DeploymentForEchoClient(m *cachev1.AstraAgent, ctx context.Context) (*appsv1.Deployment, error) {
	log := ctrllog.FromContext(ctx)
	ls := labelsForEchoClient(EchoClientName)
	var replicas int32
	if m.Spec.EchoClient.Size > 0 {
		replicas = m.Spec.EchoClient.Size
	} else {
		log.Info("Defaulting the Nats replica size", "size", EchoClientDefaultSize)
		replicas = EchoClientDefaultSize
	}

	var echoClientImage string
	if m.Spec.EchoClient.Image != "" {
		echoClientImage = m.Spec.EchoClient.Image
	} else {
		log.Info("Defaulting the EchoClient image", "image", EchoClientDefaultImage)
		echoClientImage = EchoClientDefaultImage
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      EchoClientName,
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
						Image: echoClientImage,
						Name:  EchoClientName,
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

// labelsForEchoClient returns the labels for selecting the EchoClient
// belonging to the given astraAgent CR name.
func labelsForEchoClient(name string) map[string]string {
	return map[string]string{"app": name}
}
