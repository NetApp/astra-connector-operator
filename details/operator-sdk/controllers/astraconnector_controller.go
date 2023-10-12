/*
 * Copyright (c) 2023. NetApp, Inc. All Rights Reserved.
 */

package controllers

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/NetApp-Polaris/astra-connector-operator/app/conf"
	"github.com/NetApp-Polaris/astra-connector-operator/app/register"
	"github.com/NetApp-Polaris/astra-connector-operator/details/k8s"
	"github.com/NetApp-Polaris/astra-connector-operator/details/k8s/precheck"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

// AstraConnectorController reconciles a AstraConnector object
type AstraConnectorController struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=astra.netapp.io,resources=astraconnectors,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=astra.netapp.io,resources=astraconnectors/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=astra.netapp.io,resources=astraconnectors/finalizers,verbs=update
//+kubebuilder:rbac:groups=*,resources=*,verbs=*
//+kubebuilder:rbac:groups="";apiextensions.k8s.io;apps;autoscaling;batch;crd.projectcalico.org;extensions;networking.k8s.io;policy;rbac.authorization.k8s.io;security.openshift.io;snapshot.storage.k8s.io;storage.k8s.io;trident.netapp.io,resources=configmaps;cronjobs;customresourcedefinitions;daemonsets;deployments;horizontalpodautoscalers;ingresses;jobs;namespaces;networkpolicies;persistentvolumeclaims;poddisruptionbudgets;pods;podtemplates;podsecuritypolicies;replicasets;replicationcontrollers;replicationcontrollers/scale;rolebindings;roles;secrets;serviceaccounts;services;statefulsets;storageclasses;csidrivers;csinodes;securitycontextconstraints;tridentmirrorrelationships;tridentsnapshotinfos;tridentvolumes;volumesnapshots;volumesnapshotcontents;tridentversions;tridentbackends;tridentnodes,verbs=get;list;watch;delete;use;create;update;patch
// +kubebuilder:rbac:urls=/metrics,verbs=get;list;watch

func (r *AstraConnectorController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// Fetch the AstraConnector instance
	astraConnector := &v1.AstraConnector{}
	natsSyncClientStatus := v1.NatsSyncClientStatus{
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
		natsSyncClientStatus.Status = FailedAstraConnectorGet
		_ = r.updateAstraConnectorStatus(ctx, astraConnector, natsSyncClientStatus)
		// Do not timeout for requeue. This is a user input error
		return ctrl.Result{}, err
	}

	// Validate AstraConnector CR for any errors
	err = r.validateAstraConnector(*astraConnector, log)
	if err != nil {
		// Do not timeout for requeue. This is a user input error
		return ctrl.Result{}, err
	}

	// name of our custom finalizer
	finalizerName := "netapp.io/finalizer"
	// examine DeletionTimestamp to determine if object is under deletion
	if astraConnector.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(astraConnector, finalizerName) {
			log.Info("Adding finalizer to AstraConnector instance", "finalizerName", finalizerName)
			controllerutil.AddFinalizer(astraConnector, finalizerName)
			if err := r.Update(ctx, astraConnector); err != nil {
				natsSyncClientStatus.Status = FailedFinalizerAdd
				_ = r.updateAstraConnectorStatus(ctx, astraConnector, natsSyncClientStatus)
				return ctrl.Result{RequeueAfter: time.Minute * conf.Config.ErrorTimeout()}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(astraConnector, finalizerName) {
			registerUtil := register.NewClusterRegisterUtil(astraConnector, &http.Client{}, r.Client, log, context.Background())
			log.Info("Unregistering natsSyncClient upon CRD delete")
			err = registerUtil.UnRegisterNatsSyncClient()
			if err != nil {
				log.Error(err, FailedUnRegisterNSClient+", ignoring...")
			} else {
				natsSyncClientStatus.Status = "Unregistered"
				log.Info("Unregistered natsSyncClient upon CRD delete")
			}

			// delete any cluster scoped resources created by the operator
			r.deleteConnectorClusterScopedResources(ctx, astraConnector)

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(astraConnector, finalizerName)
			if err := r.Update(ctx, astraConnector); err != nil {
				natsSyncClientStatus.Status = FailedFinalizerRemove
				_ = r.updateAstraConnectorStatus(ctx, astraConnector, natsSyncClientStatus)
				// Do not timeout requeue. Item is being deleted
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		// Do not timeout requeue
		return ctrl.Result{}, nil
	}

	k8sUtil := k8s.NewK8sUtil(r.Client, log)
	preCheckClient := precheck.NewPrecheckClient(log, k8sUtil)
	preCheckClient.Run()

	// deploy Neptune
	if conf.Config.FeatureFlags().DeployNeptune() {
		log.Info("Initiating Neptune deployment")
		neptuneResult, err := r.deployNeptune(ctx, astraConnector, &natsSyncClientStatus)
		if err != nil {
			return neptuneResult, err
		}
	}

	if conf.Config.FeatureFlags().DeployNatsConnector() {
		// deploy Connector
		connectorResults, err := r.deployConnector(ctx, astraConnector, &natsSyncClientStatus)
		if err != nil {
			log.Error(err, "Error deploying resources")
			// Note: Returning nil in error since we want to wait a minute for the requeue to happen
			// non nil errors triggers the requeue right away
			return connectorResults, nil
		}
	}

	_ = r.updateAstraConnectorStatus(ctx, astraConnector, natsSyncClientStatus)
	return ctrl.Result{}, nil
}

func (r *AstraConnectorController) updateAstraConnectorStatus(ctx context.Context, astraConnector *v1.AstraConnector, natsSyncClientStatus v1.NatsSyncClientStatus) error {
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

	// due to conflicts with network or changing object we need to retry on conflict
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := r.Get(ctx, types.NamespacedName{Name: astraConnector.Name, Namespace: astraConnector.Namespace}, astraConnector)
		if err != nil {
			return err
		}

		// Update status.Nodes if needed
		if !reflect.DeepEqual(podNames, astraConnector.Status.Nodes) {
			astraConnector.Status.Nodes = podNames
		}

		// FIXME Status should never be nil
		if astraConnector.Status.Nodes == nil {
			astraConnector.Status.Nodes = []string{""}
		}

		if !reflect.DeepEqual(natsSyncClientStatus, astraConnector.Status.NatsSyncClient) {
			log.Info("Updating the natsSyncClient status")
			astraConnector.Status.NatsSyncClient = natsSyncClientStatus
			err := r.Status().Update(ctx, astraConnector)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

// SetupWithManager sets up the controller with the Manager.
func (r *AstraConnectorController) SetupWithManager(mgr ctrl.Manager) error {
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

func (r *AstraConnectorController) validateAstraConnector(connector v1.AstraConnector, logger logr.Logger) error {
	var validateErrors field.ErrorList

	logger.V(3).Info("Validating Create AstraConnector")
	validateErrors = connector.ValidateCreateAstraConnector()

	var fieldErrors []string
	for _, v := range validateErrors {
		if v == nil {
			continue
		}
		fieldErrors = append(fieldErrors, fmt.Sprintf("'%s' %s", v.Field, v.Detail))
	}

	if len(fieldErrors) == 0 {
		return nil
	}

	return errors.New(fmt.Sprintf("Errors while validating AstraConnector CR: %s", strings.Join(fieldErrors, "; ")))
}
