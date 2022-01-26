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
	rbacv1 "k8s.io/api/rbac/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"context"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
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
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete

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

	deployments := map[string]string{
		astraAgent.Spec.NatssyncClient.Name:  "DeploymentForNatssyncClient",
		astraAgent.Spec.HttpProxyClient.Name: "DeploymentForProxyClient",
		astraAgent.Spec.EchoClient.Name:      "DeploymentForEchoClient",
	}
	statefulSets := map[string]string{
		astraAgent.Spec.Nats.Name: "StatefulsetForNats",
	}
	services := map[string]string{
		astraAgent.Spec.NatssyncClient.Name:     "ServiceForNatssyncClient",
		astraAgent.Spec.Nats.Name:               "ServiceForNats",
		astraAgent.Spec.Nats.ClusterServiceName: "ClusterServiceForNats",
	}

	// Check if the deployment already exists, if not create a new one
	replicaSize := map[string]int32{
		astraAgent.Spec.Nats.Name:           astraAgent.Spec.Nats.Size,
		astraAgent.Spec.NatssyncClient.Name: astraAgent.Spec.NatssyncClient.Size,
	}

	for statefulSet, funcName := range statefulSets {
		foundSet := &appsv1.StatefulSet{}
		log.Info("Finding StatefulSet", "StatefulSet.Namespace", astraAgent.Spec.Namespace, "StatefulSet.Name", statefulSet)
		err = r.Get(ctx, types.NamespacedName{Name: statefulSet, Namespace: astraAgent.Spec.Namespace}, foundSet)
		if err != nil && errors.IsNotFound(err) {
			// Define a new statefulset
			// Use reflection to call the method
			in := make([]reflect.Value, 1)
			in[0] = reflect.ValueOf(astraAgent)
			method := reflect.ValueOf(r).MethodByName(funcName)
			val := method.Call(in)
			set := val[0].Interface().(*appsv1.StatefulSet)

			log.Info("Creating a new StatefulSet", "StatefulSet.Namespace", set.Namespace, "StatefulSet.Name", set.Name)
			err = r.Create(ctx, set)
			if err != nil {
				log.Error(err, "Failed to create new StatefulSet", "StatefulSet.Namespace", set.Namespace, "StatefulSet.Name", set.Name)
				return ctrl.Result{}, err
			}
			// StatefulSet created successfully - return and requeue
			return ctrl.Result{Requeue: true}, nil
		} else if err != nil {
			log.Error(err, "Failed to get nats StatefulSet")
			return ctrl.Result{}, err
		}

		// Ensure the nats statefulset size is the same as the spec
		natsSize := replicaSize[astraAgent.Spec.Nats.Name]
		if *foundSet.Spec.Replicas != natsSize {
			foundSet.Spec.Replicas = &natsSize
			err = r.Update(ctx, foundSet)
			if err != nil {
				log.Error(err, "Failed to update StatefulSet", "StatefulSet.Namespace", foundSet.Namespace, "StatefulSet.Name", foundSet.Name)
				return ctrl.Result{}, err
			}
			// Ask to requeue after 1 minute in order to give enough time for the
			// pods be created on the cluster side and the operand be able
			// to do the next update step accurately.
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}
	}

	for deployment, funcName := range deployments {
		foundDep := &appsv1.Deployment{}
		log.Info("Finding Deployment", "Deployment.Namespace", astraAgent.Spec.Namespace, "Deployment.Name", deployment)
		err = r.Get(ctx, types.NamespacedName{Name: deployment, Namespace: astraAgent.Spec.Namespace}, foundDep)
		if err != nil && errors.IsNotFound(err) {
			// Define a new deployment
			// Use reflection to call the method
			in := make([]reflect.Value, 1)
			in[0] = reflect.ValueOf(astraAgent)
			method := reflect.ValueOf(r).MethodByName(funcName)
			val := method.Call(in)
			dep := val[0].Interface().(*appsv1.Deployment)
			errCall := val[1].Interface()
			if errCall != nil {
				log.Error(errCall.(error), "Failed to get Deployment object")
				return ctrl.Result{}, errCall.(error)
			}

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
		size := replicaSize[astraAgent.Spec.NatssyncClient.Name]
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
	}

	// Check if the services already exists, if not create a new one
	for service, funcName := range services {
		foundSer := &corev1.Service{}
		log.Info("Finding Service", "Service.Namespace", astraAgent.Spec.Namespace, "Service.Name", service)
		err = r.Get(ctx, types.NamespacedName{Name: service, Namespace: astraAgent.Spec.Namespace}, foundSer)
		if err != nil && errors.IsNotFound(err) {
			// Define a new service
			// Use reflection to call the method
			in := make([]reflect.Value, 1)
			in[0] = reflect.ValueOf(astraAgent)
			method := reflect.ValueOf(r).MethodByName(funcName)
			val := method.Call(in)
			serv := val[0].Interface().(*corev1.Service)

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
	}

	// Create configmap
	foundCM := &corev1.ConfigMap{}
	log.Info("Finding ConfigMap", "ConfigMap.Namespace", astraAgent.Spec.Namespace, "ConfigMap.Name", astraAgent.Spec.ConfigMap.Name)
	err = r.Get(ctx, types.NamespacedName{Name: astraAgent.Spec.ConfigMap.Name, Namespace: astraAgent.Spec.Namespace}, foundCM)
	if err != nil && errors.IsNotFound(err) {
		// Define a new configmap
		configMP := r.ConfigMap(astraAgent)
		log.Info("Creating a new ConfigMap", "ConfigMap.Namespace", configMP.Namespace, "ConfigMap.Name", configMP.Name)
		err = r.Create(ctx, configMP)
		if err != nil {
			log.Error(err, "Failed to create new ConfigMap", "ConfigMap.Namespace", configMP.Namespace, "ConfigMap.Name", configMP.Name)
			return ctrl.Result{}, err
		}
		// ConfigMap created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get ConfigMap")
		return ctrl.Result{}, err
	}

	// Create configmap role
	foundRole := &rbacv1.Role{}
	log.Info("Finding ConfigMap Role", "Role.Namespace", astraAgent.Spec.Namespace, "Role.Name", astraAgent.Spec.ConfigMap.RoleName)
	err = r.Get(ctx, types.NamespacedName{Name: astraAgent.Spec.ConfigMap.RoleName, Namespace: astraAgent.Spec.Namespace}, foundRole)
	if err != nil && errors.IsNotFound(err) {
		// Define a new Role
		configMPRole := r.ConfigMapRole(astraAgent)
		log.Info("Creating a new Role", "Role.Namespace", configMPRole.Namespace, "Role.Name", configMPRole.Name)
		err = r.Create(ctx, configMPRole)
		if err != nil {
			log.Error(err, "Failed to create new Role", "Role.Namespace", configMPRole.Namespace, "Role.Name", configMPRole.Name)
			return ctrl.Result{}, err
		}
		// Role created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Role")
		return ctrl.Result{}, err
	}

	// Create configmap rolebinding
	foundRoleB := &rbacv1.RoleBinding{}
	log.Info("Finding ConfigMap RoleBinding", "RoleBinding.Namespace", astraAgent.Spec.Namespace, "RoleBinding.Name", astraAgent.Spec.ConfigMap.RoleBindingName)
	err = r.Get(ctx, types.NamespacedName{Name: astraAgent.Spec.ConfigMap.RoleBindingName, Namespace: astraAgent.Spec.Namespace}, foundRoleB)
	if err != nil && errors.IsNotFound(err) {
		// Define a new RoleBinding
		roleB := r.ConfigMapRoleBinding(astraAgent)
		log.Info("Creating a new RoleBinding", "RoleBinding.Namespace", roleB.Namespace, "RoleBinding.Name", roleB.Name)
		err = r.Create(ctx, roleB)
		if err != nil {
			log.Error(err, "Failed to create new RoleBinding", "RoleBinding.Namespace", roleB.Namespace, "RoleBinding.Name", roleB.Name)
			return ctrl.Result{}, err
		}
		// RoleBinding created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get RoleBinding")
		return ctrl.Result{}, err
	}

	// Create configmap service account
	foundSA := &corev1.ServiceAccount{}
	log.Info("Finding ServiceAccount", "ServiceAccount.Namespace", astraAgent.Spec.Namespace, "ServiceAccount.Name", astraAgent.Spec.ConfigMap.ServiceAccountName)
	err = r.Get(ctx, types.NamespacedName{Name: astraAgent.Spec.ConfigMap.ServiceAccountName, Namespace: astraAgent.Spec.Namespace}, foundSA)
	if err != nil && errors.IsNotFound(err) {
		// Define a new ServiceAccount
		configMPRoleB := r.ServiceAccountForConfigMap(astraAgent)
		log.Info("Creating a new ServiceAccount", "ServiceAccount.Namespace", configMPRoleB.Namespace, "ServiceAccount.Name", configMPRoleB.Name)
		err = r.Create(ctx, configMPRoleB)
		if err != nil {
			log.Error(err, "Failed to create new ServiceAccount", "ServiceAccount.Namespace", configMPRoleB.Namespace, "ServiceAccount.Name", configMPRoleB.Name)
			return ctrl.Result{}, err
		}
		// ServiceAccount created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get ServiceAccount")
		return ctrl.Result{}, err
	}

	// RegisterClient
	natssyncClientStatus, err := r.getNatssyncClientStatus(astraAgent, ctx)
	if err != nil {
		log.Error(err, "Failed to get natssync-client status")
		return ctrl.Result{}, err
	}

	if astraAgent.Spec.Astra.Register && natssyncClientStatus.Registered == "False" {
		log.Info("Registering natssync-client")
		locationID, err := r.RegisterClient(astraAgent)
		if err != nil {
			log.Error(err, "Failed to register natssync-client")
			return ctrl.Result{}, err
		}
		natssyncClientStatus.Registered = "True"
		natssyncClientStatus.LocationID = locationID
		log.Info("natssync-client locationID", "locationID", locationID)
	}

	// Update the astraAgent status with the pod names
	// List the pods for this astraAgent's deployment
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(astraAgent.Spec.Namespace),
	}
	if err = r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "astraAgent.Spec.Namespace", astraAgent.Spec.Namespace)
		return ctrl.Result{}, err
	}
	podNames := getPodNames(podList.Items)

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, astraAgent.Status.Nodes) {
		log.Info("Updating the pod status")
		astraAgent.Status.Nodes = podNames
		err := r.Status().Update(ctx, astraAgent)
		if err != nil {
			log.Error(err, "Failed to update astraAgent status")
			return ctrl.Result{}, err
		}
	}

	if !reflect.DeepEqual(natssyncClientStatus, astraAgent.Status.NatssyncClient) {
		log.Info("Updating the natssync-client status")
		astraAgent.Status.NatssyncClient = natssyncClientStatus
		err := r.Status().Update(ctx, astraAgent)
		if err != nil {
			log.Error(err, "Failed to update natssync-client status")
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
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}
