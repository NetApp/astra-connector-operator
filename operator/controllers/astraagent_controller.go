/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"context"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime"

	cachev1 "github.com/NetApp/astraagent-operator/api/v1"
)

// AstraAgentReconciler reconciles a AstraAgent object
type AstraAgentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=cache.astraagent.com,resources=astraagents,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cache.astraagent.com,resources=astraagents/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cache.astraagent.com,resources=astraagents/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

func (r *AstraAgentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// Fetch the AstraAgent instance
	astraAgent := &cachev1.AstraAgent{}
	err := r.Get(ctx, req.NamespacedName, astraAgent)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("AstraAgent resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get AstraAgent")
		return ctrl.Result{}, err
	}

	// Check if the deployment already exists, if not create a new one
	foundDep := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: astraAgent.Name, Namespace: astraAgent.Namespace}, foundDep)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep := r.deploymentForAstraAgent(astraAgent)
		log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Create(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	// Ensure the deployment size is the same as the spec
	size := astraAgent.Spec.Size
	if *foundDep.Spec.Replicas != size {
		foundDep.Spec.Replicas = &size
		err = r.Update(ctx, foundDep)
		if err != nil {
			log.Error(err, "Failed to update Deployment", "Deployment.Namespace", foundDep.Namespace, "Deployment.Name", foundDep.Name)
			return ctrl.Result{}, err
		}
		// Ask to requeue after 1 minute in order to give enough time for the
		// pods be created on the cluster side and the operand be able
		// to do the next update step accurately.
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// Check if the service already exists, if not create a new one
	foundSer := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: astraAgent.Name, Namespace: astraAgent.Namespace}, foundSer)
	if err != nil && errors.IsNotFound(err) {
		// Define a new service
		serv := r.serviceForAstraAgent(astraAgent)
		log.Info("Creating a new Service", "Service.Namespace", serv.Namespace, "Service.Name", serv.Name)
		err = r.Create(ctx, serv)
		if err != nil {
			log.Error(err, "Failed to create new Service", "Service.Namespace", serv.Namespace, "Service.Name", serv.Name)
			return ctrl.Result{}, err
		}
		// Service created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	// Update the astraAgent status with the pod names
	// List the pods for this astraAgent's deployment
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(astraAgent.Namespace),
		client.MatchingLabels(labelsForAstraAgent(astraAgent.Name)),
	}
	if err = r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "astraAgent.Namespace", astraAgent.Namespace, "astraAgent.Name", astraAgent.Name)
		return ctrl.Result{}, err
	}
	podNames := getPodNames(podList.Items)

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, astraAgent.Status.Nodes) {
		astraAgent.Status.Nodes = podNames
		err := r.Status().Update(ctx, astraAgent)
		if err != nil {
			log.Error(err, "Failed to update astraAgent status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AstraAgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1.AstraAgent{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

// deploymentForAstraAgent returns a astraAgent Deployment object
func (r *AstraAgentReconciler) deploymentForAstraAgent(m *cachev1.AstraAgent) *appsv1.Deployment {
	ls := labelsForAstraAgent(m.Name)
	replicas := m.Spec.Size

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
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
						Image: "theotw/natssync-client:latest",
						Name:  "astra-agent",
					}},
				},
			},
		},
	}
	// Set astraAgent instance as the owner and controller
	ctrl.SetControllerReference(m, dep, r.Scheme)
	return dep
}

// serviceForAstraAgent returns a astraAgent Deployment object
func (r *AstraAgentReconciler) serviceForAstraAgent(m *cachev1.AstraAgent) *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "astra-agent",
			Namespace: "astra-agent",
			Labels: map[string]string{
				"app": "astra-agent",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{
					Port:     8080,
					NodePort: 31000,
					Protocol: "TCP",
				},
			},
			Selector: map[string]string{
				"app": "astra-agent",
			},
		},
	}
	// Set astraAgent instance as the owner and controller
	ctrl.SetControllerReference(m, service, r.Scheme)
	return service
}

// labelsForAstraAgent returns the labels for selecting the resources
// belonging to the given astraAgent CR name.
func labelsForAstraAgent(name string) map[string]string {
	return map[string]string{"app": "astraAgent", "astraAgent_cr": name}
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}
