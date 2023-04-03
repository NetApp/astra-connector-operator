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
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/NetApp/astra-connector-operator/api/v1"
	"github.com/NetApp/astra-connector-operator/common"
	"github.com/NetApp/astra-connector-operator/register"
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
	natssyncClientStatus := v1.NatssyncClientStatus{
		Registered: "false",
	}
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
		log.Error(err, FailedAstraConnectorGet)
		natssyncClientStatus.Status = FailedAstraConnectorGet
		r.updateAstraConnectorStatus(ctx, astraConnector, natssyncClientStatus)
		return ctrl.Result{}, err
	}

	if !astraConnector.Spec.Astra.AcceptEULA {
		log.Info(EULANA + ", will not proceed with the install")
		natssyncClientStatus.Status = EULANA
		r.updateAstraConnectorStatus(ctx, astraConnector, natssyncClientStatus)
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
				natssyncClientStatus.Status = FailedFinalizerAdd
				r.updateAstraConnectorStatus(ctx, astraConnector, natssyncClientStatus)
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(astraConnector, finalizerName) {
			// our finalizer is present, so lets handle any external dependency
			log.Info("Unregistering the cluster with Astra upon CRD delete")
			if astraConnector.Spec.Astra.ClusterName != "" {
				err = register.RemoveConnectorIDFromAstra(astraConnector, ctx)
				if err != nil {
					log.Error(err, "Failed to unregister the cluster with Astra, ignoring...")
				} else {
					log.Info("Unregistered the cluster with Astra upon CRD delete")
				}
			}

			log.Info("Unregistering natssync-client upon CRD delete")
			err = register.UnregisterClient(astraConnector)
			if err != nil {
				log.Error(err, FailedUnRegisterNSClient+", ignoring...")
			} else {
				natssyncClientStatus.Status = "Unregistered"
				log.Info("Unregistered natssync-client upon CRD delete")
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(astraConnector, finalizerName)
			if err := r.Update(ctx, astraConnector); err != nil {
				natssyncClientStatus.Status = FailedFinalizerRemove
				r.updateAstraConnectorStatus(ctx, astraConnector, natssyncClientStatus)
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	err = r.CreateServices(astraConnector, natssyncClientStatus, ctx)
	if err != nil {
		log.Error(err, ErrorCreateService)
		natssyncClientStatus.Status = ErrorCreateService
		r.updateAstraConnectorStatus(ctx, astraConnector, natssyncClientStatus)
		return ctrl.Result{}, err
	}

	err = r.CreateConfigMaps(astraConnector, natssyncClientStatus, ctx)
	if err != nil {
		log.Error(err, ErrorCreateConfigMaps)
		natssyncClientStatus.Status = ErrorCreateConfigMaps
		r.updateAstraConnectorStatus(ctx, astraConnector, natssyncClientStatus)
		return ctrl.Result{}, err
	}

	err = r.CreateRoles(astraConnector, natssyncClientStatus, ctx)
	if err != nil {
		log.Error(err, ErrorCreateRoles)
		natssyncClientStatus.Status = ErrorCreateRoles
		r.updateAstraConnectorStatus(ctx, astraConnector, natssyncClientStatus)
		return ctrl.Result{}, err
	}

	err = r.CreateRoleBindings(astraConnector, natssyncClientStatus, ctx)
	if err != nil {
		log.Error(err, ErrorCreateRoleBindings)
		natssyncClientStatus.Status = ErrorCreateRoleBindings
		r.updateAstraConnectorStatus(ctx, astraConnector, natssyncClientStatus)
		return ctrl.Result{}, err
	}

	err = r.CreateServiceAccounts(astraConnector, natssyncClientStatus, ctx)
	if err != nil {
		log.Error(err, ErrorCreateServiceAccounts)
		natssyncClientStatus.Status = ErrorCreateServiceAccounts
		r.updateAstraConnectorStatus(ctx, astraConnector, natssyncClientStatus)
		return ctrl.Result{}, err
	}

	err = r.CreateStatefulSets(astraConnector, natssyncClientStatus, ctx)
	if err != nil {
		log.Error(err, ErrorCreateStatefulSets)
		natssyncClientStatus.Status = ErrorCreateStatefulSets
		r.updateAstraConnectorStatus(ctx, astraConnector, natssyncClientStatus)
		return ctrl.Result{}, err
	}

	err = r.CreateDeployments(astraConnector, natssyncClientStatus, ctx)
	if err != nil {
		log.Error(err, ErrorCreateDeployments)
		natssyncClientStatus.Status = ErrorCreateDeployments
		r.updateAstraConnectorStatus(ctx, astraConnector, natssyncClientStatus)
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
			log.Error(err, FailedLocationIDGet)
			natssyncClientStatus.Status = FailedLocationIDGet
			r.updateAstraConnectorStatus(ctx, astraConnector, natssyncClientStatus)
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
		}
		if astraConnectorID == "" {
			log.Error(err, EmptyLocationIDGet)
			natssyncClientStatus.Status = EmptyLocationIDGet
			r.updateAstraConnectorStatus(ctx, astraConnector, natssyncClientStatus)
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
		}
	}

	// RegisterClient
	if !astraConnector.Spec.Astra.Unregister {
		if registered {
			log.Info("natssync-client already registered", "astraConnectorID", astraConnectorID)
		} else {
			log.Info("Registering natssync-client")
			astraConnectorID, err = register.RegisterClient(astraConnector)
			if err != nil {
				log.Error(err, FailedRegisterNSClient)
				natssyncClientStatus.Status = FailedRegisterNSClient
				r.updateAstraConnectorStatus(ctx, astraConnector, natssyncClientStatus)
				return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
			}

			log.Info("natssync-client ConnectorID", "astraConnectorID", astraConnectorID)
		}
		natssyncClientStatus.Registered = "true"
		natssyncClientStatus.AstraConnectorID = astraConnectorID
		natssyncClientStatus.Status = RegisterNSClient

		if astraConnector.Spec.Astra.Token == "" || astraConnector.Spec.Astra.AccountID == "" || astraConnector.Spec.Astra.ClusterName == "" {
			log.Info("Skipping cluster registration with Astra, incomplete Astra details provided Token/AccountID/ClusterName")
		} else {
			log.Info("Registering cluster with Astra")
			err = register.AddConnectorIDtoAstra(astraConnector, astraConnectorID, ctx)
			if err != nil {
				log.Error(err, FailedConnectorIDAdd)
				natssyncClientStatus.Status = FailedConnectorIDAdd
				r.updateAstraConnectorStatus(ctx, astraConnector, natssyncClientStatus)
				return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
			}
			log.Info("Registered cluster with Astra")
		}
		natssyncClientStatus.Status = "Registered with Astra"
	} else {
		if registered {
			if astraConnector.Spec.Astra.Token == "" || astraConnector.Spec.Astra.AccountID == "" {
				log.Info("Skipping cluster unregister with Astra, incomplete Astra details provided Token/AccountID")
			} else {
				if astraConnector.Spec.Astra.ClusterName != "" {
					log.Info("Unregistering the cluster with Astra")
					err = register.RemoveConnectorIDFromAstra(astraConnector, ctx)
					if err != nil {
						log.Error(err, FailedConnectorIDRemove)
						natssyncClientStatus.Status = FailedConnectorIDRemove
						r.updateAstraConnectorStatus(ctx, astraConnector, natssyncClientStatus)
						return ctrl.Result{Requeue: true}, err
					}
					natssyncClientStatus.Status = UnregisterFromAstra
					log.Info(UnregisterFromAstra)
				} else {
					log.Info("Skipping unregistering the Astra cluster, no cluster name available")
				}
			}

			log.Info("Unregistering natssync-client")
			err = register.UnregisterClient(astraConnector)
			if err != nil {
				log.Error(err, FailedUnRegisterNSClient)
				natssyncClientStatus.Status = FailedUnRegisterNSClient
				r.updateAstraConnectorStatus(ctx, astraConnector, natssyncClientStatus)
				return ctrl.Result{Requeue: true}, err
			}
			log.Info(UnregisterNSClient)
		} else {
			log.Info("Already unregistered with Astra")
		}
		natssyncClientStatus.Registered = "false"
		natssyncClientStatus.AstraConnectorID = ""
		natssyncClientStatus.Status = UnregisterNSClient
	}

	_ = r.updateAstraConnectorStatus(ctx, astraConnector, natssyncClientStatus)
	return ctrl.Result{}, nil
}

func (r *AstraConnectorReconciler) updateAstraConnectorStatus(ctx context.Context, astraConnector *v1.AstraConnector, natssyncClientStatus v1.NatssyncClientStatus) error {
	// Update the astraConnector status with the pod names
	// List the pods for this astraConnector's deployment
	log := ctrllog.FromContext(ctx)
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(astraConnector.Namespace),
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "Namespace", astraConnector.Namespace)
		return err
	}
	podNames := getPodNames(podList.Items)

	err := r.Get(ctx, types.NamespacedName{Name: astraConnector.Name, Namespace: astraConnector.Namespace}, astraConnector)
	if err != nil {
		return err
	}

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, astraConnector.Status.Nodes) {
		astraConnector.Status.Nodes = podNames
	}

	if !reflect.DeepEqual(natssyncClientStatus, astraConnector.Status.NatssyncClient) {
		log.Info("Updating the natssync-client status")
		astraConnector.Status.NatssyncClient = natssyncClientStatus
		err := r.Status().Update(ctx, astraConnector)
		if err != nil {
			log.Error(err, "Failed to update natssync-client status")
			return err
		}
	}
	return nil
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
		WithEventFilter(predicate.GenerationChangedPredicate{}). // Avoid reconcile for status updates
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
