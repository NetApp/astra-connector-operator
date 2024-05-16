/*
 * Copyright (c) 2024. NetApp, Inc. All Rights Reserved.
 */

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/NetApp-Polaris/astra-connector-operator/common"
	appsv1 "k8s.io/api/apps/v1"
	"net/http"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
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
	*kubernetes.Clientset
	Scheme        *runtime.Scheme
	DynamicClient dynamic.Interface
}

// +kubebuilder:rbac:groups=astra.netapp.io,resources=astraconnectors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=astra.netapp.io,resources=astraconnectors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=astra.netapp.io,resources=astraconnectors/finalizers,verbs=update
// +kubebuilder:rbac:groups=*,resources=*,verbs=*
// +kubebuilder:rbac:groups="";apiextensions.k8s.io;apps;autoscaling;batch;crd.projectcalico.org;extensions;networking.k8s.io;policy;rbac.authorization.k8s.io;security.openshift.io;snapshot.storage.k8s.io;storage.k8s.io;trident.netapp.io,resources=configmaps;cronjobs;customresourcedefinitions;daemonsets;deployments;horizontalpodautoscalers;ingresses;jobs;namespaces;networkpolicies;persistentvolumeclaims;poddisruptionbudgets;pods;podtemplates;podsecuritypolicies;replicasets;replicationcontrollers;replicationcontrollers/scale;rolebindings;roles;secrets;serviceaccounts;services;statefulsets;storageclasses;csidrivers;csinodes;securitycontextconstraints;tridentmirrorrelationships;tridentsnapshotinfos;tridentvolumes;volumesnapshots;volumesnapshotcontents;tridentversions;tridentbackends;tridentnodes,verbs=get;list;watch;delete;use;create;update;patch
// +kubebuilder:rbac:urls=/metrics,verbs=get;list;watch

func (r *AstraConnectorController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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
		log.Error(err, FailedAstraConnectorGet)
		_ = r.updateAstraConnectorStatus(ctx, astraConnector, v1.NatsSyncClientStatus{
			Status:     FailedAstraConnectorGet,
			Registered: "false",
		})
		// Do not requeue
		return ctrl.Result{}, err
	}

	natsSyncClientStatus := astraConnector.Status.NatsSyncClient
	natsSyncClientStatus.AstraClusterId = astraConnector.Status.NatsSyncClient.AstraClusterId

	if natsSyncClientStatus.Registered == "" {
		natsSyncClientStatus.Registered = "false"
	}

	// Validate AstraConnector CR for any errors
	err = r.validateAstraConnector(*astraConnector, log)
	if err != nil {
		// Error validating the connector object. Do not requeue and update the connector status.
		log.Error(err, FailedAstraConnectorValidation)
		natsSyncClientStatus.Status = fmt.Sprintf("%s; %s", FailedAstraConnectorValidation, err.Error())
		_ = r.updateAstraConnectorStatus(ctx, astraConnector, natsSyncClientStatus)
		// Do not requeue. This is a user input error
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
				return ctrl.Result{}, err
			}
			// spec change this will trigger a reconcile
			return ctrl.Result{}, nil
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(astraConnector, finalizerName) {
			// Update status message to indicate that CR delete is in progress
			natsSyncClientStatus.Status = DeleteInProgress
			_ = r.updateAstraConnectorStatus(ctx, astraConnector, natsSyncClientStatus)

			// delete any cluster scoped resources created by the operator
			r.deleteConnectorClusterScopedResources(ctx, astraConnector)

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(astraConnector, finalizerName)
			if err := r.Update(ctx, astraConnector); err != nil {
				natsSyncClientStatus.Status = FailedFinalizerRemove
				_ = r.updateAstraConnectorStatus(ctx, astraConnector, natsSyncClientStatus)
				// Do not requeue. Item is being deleted
				return ctrl.Result{}, err
			}

			// Update status message to indicate that CR delete is in finished
			natsSyncClientStatus.Status = DeletionComplete
			_ = r.updateAstraConnectorStatus(ctx, astraConnector, natsSyncClientStatus)
		}

		// Stop reconciliation as the item is being deleted
		// Do not requeue
		return ctrl.Result{}, nil
	}

	if !astraConnector.Spec.SkipPreCheck {
		k8sUtil := k8s.NewK8sUtil(r.Client, r.Clientset, log)
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
			errString = "Pre-check errors: " + errString
			natsSyncClientStatus.Status = errString
			_ = r.updateAstraConnectorStatus(ctx, astraConnector, natsSyncClientStatus)
			// Do not requeue. Item is being deleted
			return ctrl.Result{}, errors.New(errString)
		}
	}

	// deploy Neptune
	if conf.Config.FeatureFlags().DeployNeptune() {
		log.Info("Initiating Neptune deployment")
		neptuneResult, err := r.deployNeptune(ctx, astraConnector, &natsSyncClientStatus)
		if err != nil {
			// Note: Returning nil in error since we want to wait for the requeue to happen
			// non nil errors triggers the requeue right away
			log.Error(err, "Error deploying Neptune, requeueing after delay", "delay", conf.Config.ErrorTimeout())
			return neptuneResult, nil
		}
	}

	if conf.Config.FeatureFlags().DeployNatsConnector() {
		log.Info("Initiating Connector deployment")
		var connectorResults ctrl.Result
		var deployError error

		connectorResults, deployError = r.deployNatlessConnector(ctx, astraConnector, &natsSyncClientStatus)

		// Wait for the cluster to become managed (aka "registered")
		natsSyncClientStatus.Status = WaitForClusterManagedState
		_ = r.updateAstraConnectorStatus(ctx, astraConnector, natsSyncClientStatus)
		isManaged, err := waitForManagedCluster(astraConnector, r.Client, log)
		if !isManaged {
			log.Error(err, "timed out waiting for cluster to become managed, requeueing after delay", "delay", conf.Config.ErrorTimeout())
			natsSyncClientStatus.Status = ErrorClusterUnmanaged
			_ = r.updateAstraConnectorStatus(ctx, astraConnector, natsSyncClientStatus)
			// Do not wait 5min, wait 5sec before requeue instead since we have already been waiting in waitForManagedCluster
			return ctrl.Result{RequeueAfter: time.Second * conf.Config.ErrorTimeout()}, nil
		}
		log.Info("Cluster is managed")

		// ASUP Setup
		err = r.createASUPCR(ctx, astraConnector, astraConnector.Spec.Astra.ClusterId)
		if err != nil {
			log.Error(err, FailedASUPCreation)
			natsSyncClientStatus.Status = FailedASUPCreation
			_ = r.updateAstraConnectorStatus(ctx, astraConnector, natsSyncClientStatus)
			return ctrl.Result{RequeueAfter: time.Minute * conf.Config.ErrorTimeout()}, err
		}

		natsSyncClientStatus.Registered = "true"
		natsSyncClientStatus.AstraClusterId = astraConnector.Spec.Astra.ClusterId
		natsSyncClientStatus.Status = RegisteredWithAstra
		_ = r.updateAstraConnectorStatus(ctx, astraConnector, natsSyncClientStatus)

		if deployError != nil {
			// Note: Returning nil in error since we want to wait for the requeue to happen
			// non nil errors triggers the requeue right away
			log.Error(err, "Error deploying NatsConnector, requeueing after delay", "delay", conf.Config.ErrorTimeout())
			return connectorResults, nil
		}
	}

	if natsSyncClientStatus.AstraClusterId != "" {
		log.Info(fmt.Sprintf("Updating CR status, clusterID: '%s'", natsSyncClientStatus.AstraClusterId))
	}

	_ = r.updateAstraConnectorStatus(ctx, astraConnector, natsSyncClientStatus)
	err = r.waitForStatusUpdate(astraConnector, log)
	if err != nil {
		log.Error(err, "Failed to update status, ignoring since this will be fixed on a future reconcile.")
	}

	return ctrl.Result{}, nil
}

func (r *AstraConnectorController) updateAstraConnectorStatus(
	ctx context.Context,
	astraConnector *v1.AstraConnector,
	natsSyncClientStatus v1.NatsSyncClientStatus) error {

	// due to conflicts with network or changing object we need to retry on conflict
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		astraConnector.Status.NatsSyncClient = natsSyncClientStatus

		// Update the status
		err := r.Status().Update(ctx, astraConnector)
		if err != nil {
			return err
		}

		return nil
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *AstraConnectorController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.AstraConnector{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
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

// waitForStatusUpdate waits for the status of an AstraConnector to be updated in Kubernetes.
// Status().Update() function returns even though the update is still in progress on k8s.
// Polling to make sure this function exits only after status subresource update is reflected in k8s.
func (r *AstraConnectorController) waitForStatusUpdate(astraConnector *v1.AstraConnector, log logr.Logger) error {
	interval := 2 * time.Second
	timeout := 15 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Start polling process
	err := errors.Wrap(wait.PollUntilContextTimeout(ctx, interval, timeout, true,
		func(ctx context.Context) (bool, error) {
			current := &v1.AstraConnector{}
			err := r.Get(ctx, types.NamespacedName{Name: astraConnector.Name, Namespace: astraConnector.Namespace}, current)
			if err != nil {
				log.Error(err, "Failed to get the current status of the AstraConnector. Retrying...")
				return false, nil
			}

			astraConnectorStatusJson, err := json.Marshal(astraConnector.Status)
			if err != nil {
				log.Error(err, "Failed to marshal astraConnector.Status to JSON")
				return false, nil
			}

			currentStatusJson, err := json.Marshal(current.Status)
			if err != nil {
				log.Error(err, "Failed to marshal current.Status to JSON")
				return false, nil
			}

			// If the status has not been updated yet, log the current and expected statuses and continue polling.
			if string(astraConnectorStatusJson) != string(currentStatusJson) {
				log.Info("AstraConnector instance status subresource update is in progress... retrying",
					"Expected status", astraConnector.Status, "Actual status", current.Status)
				return false, nil
			}

			// Otherwise stop polling
			return true, nil
		}), fmt.Sprintf("AstraConnector status is not updated even after %s", timeout))

	if err == nil {
		log.Info("AstraConnector status reflected in k8s")
	}
	return err
}

func (r *AstraConnectorController) needsReconcile(ctx context.Context, connector v1.AstraConnector, log logr.Logger) bool {
	observedSpec := connector.DeepCopy()
	err := r.Get(ctx, types.NamespacedName{Name: connector.Name, Namespace: connector.Namespace}, observedSpec) //if err != nil {
	if err != nil {
		log.Error(err, "Get connector CR failed, needs reconcile")
		return true
	}

	// Ensure that the cluster has registered successfully
	if connector.Status.NatsSyncClient.Registered != "true" {
		log.Info("Registered != true, needs reconcile")
		return true
	}
	// Ensure that the CR spec has not changed between reconciles
	if !reflect.DeepEqual(observedSpec.Spec, connector.Spec) {
		log.Info("Actual state does not match desired state", "registered", connector.Status.NatsSyncClient.Registered, "desiredSpec", connector.Spec, "observedSpec", observedSpec.Spec)
		return true
	}

	astraConnectDeployment := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: common.AstraConnectName, Namespace: connector.Namespace}, astraConnectDeployment)
	if err != nil {
		log.Info("Number of AstraConnector replicas does not match", "Expected", connector.Spec.AstraConnect.Replicas, "Actual", *astraConnectDeployment.Spec.Replicas)
		return true
	}
	if *astraConnectDeployment.Spec.Replicas != connector.Spec.AstraConnect.Replicas {
		return true
	}

	neptuneDeployment := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: common.AstraConnectName, Namespace: connector.Namespace}, neptuneDeployment)
	if err != nil {
		log.Info("Get neptuneDeployment failed, , needs reconcile")
		return true
	}
	if *neptuneDeployment.Spec.Replicas != common.NeptuneReplicas {
		log.Info("Number of Neptune replicas does not match", "Expected", connector.Spec.AstraConnect.Replicas, "Actual", *astraConnectDeployment.Spec.Replicas)
		return true
	}

	return false
}

func waitForManagedCluster(astraConnector *v1.AstraConnector, client client.Client, log logr.Logger) (bool, error) {
	registerUtil := register.NewClusterRegisterUtil(astraConnector, &http.Client{}, client, nil, log, context.Background())
	// SetHttpClient should be in the New func above but would require a larger refactor
	// Setup TLS if enabled, setup hostAliasIP if used
	err := registerUtil.SetHttpClient(astraConnector.Spec.Astra.SkipTLSValidation, register.GetAstraHostURL(astraConnector))
	if err != nil {
		return false, fmt.Errorf("failed to setup HTTP client: %w", err)
	}

	maxRetries := 5
	waitTime := 3 * time.Second
	var isManaged bool
	for i := 1; i <= maxRetries; i++ {
		isManaged, _, err = registerUtil.IsClusterManaged()
		if isManaged {
			break
		}
		if err != nil {
			log.Error(err, "encountered error while checking for cluster management")
		}
		log.Info("cluster not yet managed", "retiresLeft", maxRetries-i)
		time.Sleep(waitTime)
	}
	return isManaged, err
}
