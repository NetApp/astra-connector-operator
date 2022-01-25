package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	cachev1 "github.com/NetApp/astraagent-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// DeploymentForNatssyncClient returns a astraAgent Deployment object
func (r *AstraAgentReconciler) DeploymentForNatssyncClient(m *cachev1.AstraAgent) *appsv1.Deployment {
	ls := labelsForNatssyncClient(m.Spec.NatssyncClient.Name)
	replicas := m.Spec.NatssyncClient.Size

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Spec.NatssyncClient.Name,
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
						Image: m.Spec.NatssyncClient.Image,
						Name:  m.Spec.NatssyncClient.Name,
						Env: []corev1.EnvVar{
							{
								Name:  "NATS_SERVER_URL",
								Value: m.Spec.Nats.ServerURL,
							},
							{
								Name:  "CLOUD_BRIDGE_URL",
								Value: m.Spec.NatssyncClient.CloudBridgeURL,
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

// ServiceForNatssyncClient returns a astraAgent Deployment object
func (r *AstraAgentReconciler) ServiceForNatssyncClient(m *cachev1.AstraAgent) *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Spec.NatssyncClient.Name,
			Namespace: m.Spec.Namespace,
			Labels: map[string]string{
				"app": m.Spec.NatssyncClient.Name,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{
					Port:     m.Spec.NatssyncClient.Port,
					NodePort: m.Spec.NatssyncClient.NodePort,
					Protocol: m.Spec.NatssyncClient.Protocol,
				},
			},
			Selector: map[string]string{
				"app": m.Spec.NatssyncClient.Name,
			},
		},
	}
	// Set astraAgent instance as the owner and controller
	ctrl.SetControllerReference(m, service, r.Scheme)
	return service
}

// labelsForNatssyncClient returns the labels for selecting the resources
// belonging to the given astraAgent CR name.
func labelsForNatssyncClient(name string) map[string]string {
	return map[string]string{"app": name}
}

func (r *AstraAgentReconciler) getNatssyncClientStatus(m *cachev1.AstraAgent, ctx context.Context) (cachev1.NatssyncClientStatus, error) {
	pods := &corev1.PodList{}
	lb := labelsForNatssyncClient(m.Spec.NatssyncClient.Name)
	listOpts := []client.ListOption{
		client.MatchingLabels(lb),
	}
	log := ctrllog.FromContext(ctx)

	if err := r.List(ctx, pods, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "Namespace", m.Spec.Namespace)
		return cachev1.NatssyncClientStatus{}, err
	}

	natssyncClientStatus := cachev1.NatssyncClientStatus{
		PodIP:      "",
		Registered: "Unknown",
	}

	if len(pods.Items) < 1 {
		return cachev1.NatssyncClientStatus{}, errors.New("natssync-client pods not found")
	}
	nsClientPod := pods.Items[0]
	// If a pod is terminating, then we can't access the corresponding vault node's status.
	// so we break from here and return an error.
	if nsClientPod.Status.Phase != v1.PodRunning || nsClientPod.DeletionTimestamp != nil {
		return cachev1.NatssyncClientStatus{}, errors.New("natssync-client pod is terminating")
	}

	natssyncClientStatus.State = string(nsClientPod.Status.Phase)
	natssyncClientStatus.PodIP = nsClientPod.Status.PodIP
	natsSyncClientURL := fmt.Sprintf("http://%s.%s:%d/bridge-client/1/register", m.Spec.NatssyncClient.Name, m.Spec.Namespace, m.Spec.NatssyncClient.Port)
	resp, err := http.Get(natsSyncClientURL)
	if err != nil {
		log.Error(err, "Failed to get the registration state")
		return cachev1.NatssyncClientStatus{}, err
	}

	type registrationResponse struct {
		locationID string
	}
	var registrationResp registrationResponse
	all, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(all, &registrationResp)
	if err != nil {
		log.Error(err, "Failed to unmarshal the registration response")
		return cachev1.NatssyncClientStatus{}, err
	}

	if registrationResp.locationID == "" {
		natssyncClientStatus.Registered = "False"
	} else {
		natssyncClientStatus.Registered = "True"
	}

	return natssyncClientStatus, nil
}
