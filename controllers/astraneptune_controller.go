/*
 * Copyright (c) 2024. NetApp, Inc. All Rights Reserved.
 */

package controllers

import (
	"context"
	"fmt"
	"github.com/NetApp-Polaris/astra-connector-operator/api/v1"
	"github.com/NetApp-Polaris/astra-connector-operator/k8s"
	"github.com/NetApp-Polaris/astra-connector-operator/k8s/precheck"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/builder"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/NetApp-Polaris/astra-connector-operator/app/conf"
)

// AstraNeptuneController reconciles a AstraConnector object
type AstraNeptuneController struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=astra.netapp.io,resources=astraneptunes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=astra.netapp.io,resources=astraneptunes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=astra.netapp.io,resources=astraneptunes/finalizers,verbs=update
// +kubebuilder:rbac:groups=*,resources=*,verbs=*
// +kubebuilder:rbac:groups="";apiextensions.k8s.io;apps;autoscaling;batch;crd.projectcalico.org;extensions;networking.k8s.io;policy;rbac.authorization.k8s.io;security.openshift.io;snapshot.storage.k8s.io;storage.k8s.io;trident.netapp.io,resources=configmaps;cronjobs;customresourcedefinitions;daemonsets;deployments;horizontalpodautoscalers;ingresses;jobs;namespaces;networkpolicies;persistentvolumeclaims;poddisruptionbudgets;pods;podtemplates;podsecuritypolicies;replicasets;replicationcontrollers;replicationcontrollers/scale;rolebindings;roles;secrets;serviceaccounts;services;statefulsets;storageclasses;csidrivers;csinodes;securitycontextconstraints;tridentmirrorrelationships;tridentsnapshotinfos;tridentvolumes;volumesnapshots;volumesnapshotcontents;tridentversions;tridentbackends;tridentnodes,verbs=get;list;watch;delete;use;create;update;patch
// +kubebuilder:rbac:urls=/metrics,verbs=get;list;watch

func (r *AstraNeptuneController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// Fetch the AstraConnector instance
	astraNeptune := &v1.AstraNeptune{}
	err := r.Get(ctx, req.NamespacedName, astraNeptune)
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
		_ = r.updateAstraNeptuneStatus(ctx, astraNeptune, FailedAstraConnectorGet)
		// Do not requeue
		return ctrl.Result{}, err
	}
	// todo write valdidator for this - Oscar
	//// Validate AstraConnector CR for any errors
	//err = r.validateAstraNeptune(*astraNeptune, log)
	//if err != nil {
	//	// Error validating the connector object. Do not requeue and update the connector status.
	//	log.Error(err, FailedAstraConnectorValidation)
	//	_ = r.updateAstraConnectorStatus(ctx, astraNeptune, FailedAstraConnectorValidation)
	//	// Do not requeue. This is a user input error
	//	return ctrl.Result{}, err
	//}

	// name of our custom finalizer
	finalizerName := "netapp.io/finalizer"
	// examine DeletionTimestamp to determine if object is under deletion
	if astraNeptune.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(astraNeptune, finalizerName) {
			log.Info("Adding finalizer to AstraConnector instance", "finalizerName", finalizerName)
			controllerutil.AddFinalizer(astraNeptune, finalizerName)
			if err := r.Update(ctx, astraNeptune); err != nil {
				_ = r.updateAstraNeptuneStatus(ctx, astraNeptune, FailedFinalizerAdd)
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(astraNeptune, finalizerName) {
			// Update status message to indicate that CR delete is in progress
			//natsSyncClientStatus.Status = DeleteInProgress
			//_ = r.updateAstraConnectorStatus    (ctx, astraConnector, natsSyncClientStatus)

			// delete any cluster scoped resources created by the operator
			r.deleteNeptuneClusterScopedResources(ctx, astraNeptune)

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(astraNeptune, finalizerName)
			if err := r.Update(ctx, astraNeptune); err != nil {
				_ = r.updateAstraNeptuneStatus(ctx, astraNeptune, FailedFinalizerRemove)
				// Do not requeue. Item is being deleted
				return ctrl.Result{}, err
			}

			// Update status message to indicate that CR delete is in finished
			_ = r.updateAstraNeptuneStatus(ctx, astraNeptune, DeletionComplete)
		}

		// Stop reconciliation as the item is being deleted
		// Do not requeue
		return ctrl.Result{}, nil
	}

	if !astraNeptune.Spec.SkipPreCheck {
		k8sUtil := k8s.NewK8sUtil(r.Client, log)
		preCheckClient := precheck.NewPrecheckClient(log, k8sUtil)
		errList := preCheckClient.Run()

		if errList != nil {
			errString := ""
			for i, err := range errList {
				if i > 0 {
					errString = errString + ", "
				}

				log.Error(err, "Pre-check Error")
				errString = errString + err.Error()
			}
			_ = r.updateAstraNeptuneStatus(ctx, astraNeptune, errString)
			// Do not requeue. Item is being deleted
			return ctrl.Result{}, errors.New(errString)
		}
	}

	// deploy Neptune
	if conf.Config.FeatureFlags().DeployNeptune() {
		log.Info("Initiating Neptune deployment")
		neptuneResult, err := r.deployNeptune(ctx, astraNeptune)
		if err != nil {
			// Note: Returning nil in error since we want to wait for the requeue to happen
			// non nil errors triggers the requeue right away
			log.Error(err, "Error deploying Neptune, requeueing after delay", "delay", conf.Config.ErrorTimeout())
			return neptuneResult, nil
		}
	}

	return ctrl.Result{}, nil

}

func (r *AstraNeptuneController) updateAstraNeptuneStatus(
	ctx context.Context,
	astraNeptune *v1.AstraNeptune,
	status string) error {
	// Update the astraConnector status with the pod names
	// List the pods for this astraConnector's deployment
	//log := ctrllog.FromContext(ctx)

	astraNeptune.Status.Status = status

	// Update the status
	err := r.Status().Update(ctx, astraNeptune)
	if err != nil {
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AstraNeptuneController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.AstraNeptune{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		WithEventFilter(predicate.GenerationChangedPredicate{}). // Avoid reconcile for status updates
		Complete(r)
}
func (r *AstraNeptuneController) createASUPCR(ctx context.Context, astraNeptune *v1.AstraNeptune, astraClusterID string) error {
	log := ctrllog.FromContext(ctx)
	k8sUtil := k8s.NewK8sUtil(r.Client, log)

	cr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "astra.netapp.io/v1",
			"kind":       "AutoSupportBundleSchedule",
			"metadata": map[string]interface{}{
				"name":      "asupbundleschedule-" + astraClusterID,
				"namespace": astraNeptune.Namespace,
			},
			"spec": map[string]interface{}{
				"enabled": astraNeptune.Spec.AutoSupport.Enrolled,
			},
		},
	}
	// Define the MutateFn function
	mutateFn := func() error {
		cr.Object["spec"].(map[string]interface{})["enabled"] = astraNeptune.Spec.AutoSupport.Enrolled
		return nil
	}
	result, err := k8sUtil.CreateOrUpdateResource(ctx, cr, astraNeptune, mutateFn)
	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Successfully %s AutoSupportBundleSchedule", result))
	return nil
}

//func (r *AstraNeptuneController) validateAstraConnector(connector v1.AstraConnector, logger logr.Logger) error {
//	var validateErrors field.ErrorList
//
//	logger.V(3).Info("Validating Create AstraConnector")
//	validateErrors = connector.ValidateCreateAstraConnector()
//
//	var fieldErrors []string
//	for _, v := range validateErrors {
//		if v == nil {
//			continue
//		}
//		fieldErrors = append(fieldErrors, fmt.Sprintf("'%s' %s", v.Field, v.Detail))
//	}
//
//	if len(fieldErrors) == 0 {
//		return nil
//	}
//
//	return errors.New(fmt.Sprintf("Errors while validating AstraConnector CR: %s", strings.Join(fieldErrors, "; ")))
//}
