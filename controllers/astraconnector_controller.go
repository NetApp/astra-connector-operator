/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package controllers

import (
	"context"
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/NetApp/astraagent-operator/api/v1"
	"github.com/NetApp/astraagent-operator/common"
	"github.com/NetApp/astraagent-operator/register"
)

// AstraConnectorReconciler reconciles a AstraConnector object
type AstraConnectorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=netapp.astraconnector.com,resources=astraconnectors,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=netapp.astraconnector.com,resources=astraconnectors/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=netapp.astraconnector.com,resources=astraconnectors/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete

func (r *AstraConnectorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// Fetch the AstraConnector instance
	astraConnector := &v1.AstraConnector{}
	err := r.Get(ctx, req.NamespacedName, astraConnector)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("AstraConnector resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get AstraConnector")
		return ctrl.Result{}, err
	}

	if !astraConnector.Spec.Astra.AcceptEULA {
		log.Info("End User License Agreement set to false, will not proceed with the install")
		return ctrl.Result{}, nil
	}

	// name of our custom finalizer
	finalizerName := "astraconnector.com/finalizer"
	// examine DeletionTimestamp to determine if object is under deletion
	if astraConnector.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(astraConnector, finalizerName) {
			log.Info("Adding finalizer to AstraConnector instance", "finalizerName", finalizerName)
			controllerutil.AddFinalizer(astraConnector, finalizerName)
			if err := r.Update(ctx, astraConnector); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(astraConnector, finalizerName) {
			// our finalizer is present, so lets handle any external dependency
			log.Info("Unregistering the cluster with Astra upon CRD delete")
			err = register.RemoveConnectorIDFromAstra(astraConnector, ctx)
			if err != nil {
				log.Error(err, "Failed to unregister the cluster with Astra, ignoring...")
			} else {
				log.Info("Unregistered the cluster with Astra upon CRD delete")
			}

			log.Info("Unregistering natssync-client upon CRD delete")
			err = register.UnregisterClient(astraConnector)
			if err != nil {
				log.Error(err, "Failed to unregister natssync-client, ignoring...")
			} else {
				log.Info("Unregistered natssync-client upon CRD delete")
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(astraConnector, finalizerName)
			if err := r.Update(ctx, astraConnector); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	err = r.CreateServices(astraConnector, ctx)
	if err != nil {
		log.Error(err, "Error creating services")
		return ctrl.Result{}, err
	}

	err = r.CreateConfigMaps(astraConnector, ctx)
	if err != nil {
		log.Error(err, "Error creating configmaps")
		return ctrl.Result{}, err
	}

	err = r.CreateRoles(astraConnector, ctx)
	if err != nil {
		log.Error(err, "Error creating roles")
		return ctrl.Result{}, err
	}

	err = r.CreateRoleBindings(astraConnector, ctx)
	if err != nil {
		log.Error(err, "Error creating role bindings")
		return ctrl.Result{}, err
	}

	err = r.CreateServiceAccounts(astraConnector, ctx)
	if err != nil {
		log.Error(err, "Error creating service accounts")
		return ctrl.Result{}, err
	}

	err = r.CreateStatefulSets(astraConnector, ctx)
	if err != nil {
		log.Error(err, "Error creating stateful sets")
		return ctrl.Result{}, err
	}

	err = r.CreateDeployments(astraConnector, ctx)
	if err != nil {
		log.Error(err, "Error creating deployments")
		return ctrl.Result{}, err
	}

	registered := false
	log.Info("Checking for natssync-client configmap")
	foundCM := &corev1.ConfigMap{}
	astraConnectorID := ""
	err = r.Get(ctx, types.NamespacedName{Name: common.NatssyncClientConfigMapName, Namespace: astraConnector.Namespace}, foundCM)
	if len(foundCM.Data) != 0 {
		registered = true
		astraConnectorID, err = register.GetConnectorIDFromConfigMap(foundCM.Data)
		if err != nil {
			log.Error(err, "Failed to get the location ID from configmap")
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
		}
		if astraConnectorID == "" {
			log.Error(err, "Got an empty location ID from configmap")
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
		}
	}

	// RegisterClient
	natssyncClientStatus := v1.NatssyncClientStatus{}
	if !astraConnector.Spec.Astra.Unregister {
		if registered {
			log.Info("natssync-client already registered", "astraConnectorID", astraConnectorID)
		} else {
			log.Info("Registering natssync-client")
			astraConnectorID, err = register.RegisterClient(astraConnector)
			if err != nil {
				log.Error(err, "Failed to register natssync-client")
				return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
			}

			log.Info("natssync-client ConnectorID", "astraConnectorID", astraConnectorID)
		}
		natssyncClientStatus.Registered = "true"
		natssyncClientStatus.AstraConnectorID = astraConnectorID

		if astraConnector.Spec.Astra.Token == "" || astraConnector.Spec.Astra.AccountID == "" || astraConnector.Spec.Astra.ClusterName == "" {
			log.Info("Skipping cluster registration with Astra, incomplete Astra details provided Token/AccountID/ClusterName")
		} else {
			log.Info("Registering cluster with Astra")
			err = register.AddConnectorIDtoAstra(astraConnector, astraConnectorID, ctx)
			if err != nil {
				log.Error(err, "Failed to register astraConnectorID")
				return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
			}
			log.Info("Registered cluster with Astra")
		}
	} else {
		if registered {
			if astraConnector.Spec.Astra.Token == "" || astraConnector.Spec.Astra.AccountID == "" || astraConnector.Spec.Astra.ClusterName == "" {
				log.Info("Skipping cluster unregister with Astra, incomplete Astra details provided Token/AccountID/ClusterName")
			} else {
				log.Info("Unregistering the cluster with Astra")
				err = register.RemoveConnectorIDFromAstra(astraConnector, ctx)
				if err != nil {
					log.Error(err, "Failed to unregister the cluster with Astra")
					return ctrl.Result{Requeue: true}, err
				}
				log.Info("Unregistered the cluster with Astra")
			}

			log.Info("Unregistering natssync-client")
			err = register.UnregisterClient(astraConnector)
			if err != nil {
				log.Error(err, "Failed to unregister natssync-client")
				return ctrl.Result{Requeue: true}, err
			}
			log.Info("Unregistered natssync-client")
		} else {
			log.Info("Already unregistered with Astra")
		}
		natssyncClientStatus.Registered = "false"
		natssyncClientStatus.AstraConnectorID = ""
	}

	// Update the astraConnector status with the pod names
	// List the pods for this astraConnector's deployment
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(astraConnector.Namespace),
	}
	if err = r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "Namespace", astraConnector.Namespace)
		return ctrl.Result{}, err
	}
	podNames := getPodNames(podList.Items)

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, astraConnector.Status.Nodes) {
		log.Info("Updating the pod status")
		astraConnector.Status.Nodes = podNames
		err := r.Status().Update(ctx, astraConnector)
		if err != nil {
			log.Error(err, "Failed to update astraConnector status")
			return ctrl.Result{}, err
		}
	}

	if !reflect.DeepEqual(natssyncClientStatus, astraConnector.Status.NatssyncClient) {
		log.Info("Updating the natssync-client status")
		astraConnector.Status.NatssyncClient = natssyncClientStatus
		err := r.Status().Update(ctx, astraConnector)
		if err != nil {
			log.Error(err, "Failed to update natssync-client status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AstraConnectorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.AstraConnector{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
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
