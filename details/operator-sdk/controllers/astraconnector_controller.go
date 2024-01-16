/*
 * Copyright (c) 2024. NetApp, Inc. All Rights Reserved.
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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/dynamic"
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
	Scheme        *runtime.Scheme
	DynamicClient dynamic.Interface
}

//+kubebuilder:rbac:groups=astra.netapp.io,resources=astraconnectors,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=astra.netapp.io,resources=astraconnectors/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=astra.netapp.io,resources=astraconnectors/finalizers,verbs=update
//+kubebuilder:rbac:groups=*,resources=*,verbs=*
//+kubebuilder:rbac:groups="";apiextensions.k8s.io;apps;autoscaling;batch;crd.projectcalico.org;extensions;networking.k8s.io;policy;rbac.authorization.k8s.io;security.openshift.io;snapshot.storage.k8s.io;storage.k8s.io;trident.netapp.io,resources=configmaps;cronjobs;customresourcedefinitions;daemonsets;deployments;horizontalpodautoscalers;ingresses;jobs;namespaces;networkpolicies;persistentvolumeclaims;poddisruptionbudgets;pods;podtemplates;podsecuritypolicies;replicasets;replicationcontrollers;replicationcontrollers/scale;rolebindings;roles;secrets;serviceaccounts;services;statefulsets;storageclasses;csidrivers;csinodes;securitycontextconstraints;tridentmirrorrelationships;tridentsnapshotinfos;tridentvolumes;volumesnapshots;volumesnapshotcontents;tridentversions;tridentbackends;tridentnodes,verbs=get;list;watch;delete;use;create;update;patch
// +kubebuilder:rbac:urls=/metrics,verbs=get;list;watch

func (r *AstraConnectorController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	natsSyncClientStatus := v1.NatsSyncClientStatus{
		Registered: "false",
	}
	// Fetch the AstraConnector instance
	astraConnector := &v1.AstraConnector{}
	err := r.Get(ctx, req.NamespacedName, astraConnector)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("AstraConnector resource not found. Ignoring since object must be deleted")
			return ctrl.Result{Requeue: false}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, FailedAstraConnectorGet)
		natsSyncClientStatus.Status = FailedAstraConnectorGet
		_ = r.updateAstraConnectorStatus(ctx, astraConnector, natsSyncClientStatus)
		// Do not requeue
		return ctrl.Result{Requeue: false}, err
	}
	natsSyncClientStatus.AstraClusterId = astraConnector.Status.NatsSyncClient.AstraClusterId

	// Validate AstraConnector CR for any errors
	err = r.validateAstraConnector(*astraConnector, log)
	if err != nil {
		// Do not requeue. This is a user input error
		return ctrl.Result{Requeue: false}, err
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

			// delete any Neptune resources from the namespace AstraConnector is installed
			r.deleteNeptuneResources(ctx, astraConnector.GetNamespace())

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(astraConnector, finalizerName)
			if err := r.Update(ctx, astraConnector); err != nil {
				natsSyncClientStatus.Status = FailedFinalizerRemove
				_ = r.updateAstraConnectorStatus(ctx, astraConnector, natsSyncClientStatus)
				// Do not requeue. Item is being deleted
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		// Do not requeue
		return ctrl.Result{Requeue: false}, nil
	}

	if r.needsReconcile(*astraConnector, log) {
		log.Info("Actual state does not match desired state", "registered", astraConnector.Status.NatsSyncClient.Registered, "desiredSpec", astraConnector.Spec)
		if !astraConnector.Spec.SkipPreCheck {
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
			connectorResults, err := r.deployConnector(ctx, astraConnector, &natsSyncClientStatus)
			if err != nil {
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
		log.Info("assignging to status", "spec", astraConnector.Spec)
		astraConnector.Status.ObservedSpec = astraConnector.Spec
		err = r.Status().Update(ctx, astraConnector)
		if err != nil {
			log.Error(err, "Failed to update AstraConnector status")
			return ctrl.Result{RequeueAfter: time.Minute * conf.Config.ErrorTimeout()}, err
		}
		return ctrl.Result{Requeue: false}, nil

	} else {
		log.Info("Actual state matches desired state", "registered", astraConnector.Status.NatsSyncClient.Registered, "desiredSpec", astraConnector.Spec)
		return ctrl.Result{Requeue: false}, nil
	}
}

func (r *AstraConnectorController) deleteNeptuneResources(ctx context.Context, namespace string) {
	log := ctrllog.FromContext(ctx)
	log.Info("deleting neptune resources")

	// Get a list of all installed CRDs
	crdList := &apiextensionsv1.CustomResourceDefinitionList{}
	if err := r.Client.List(ctx, crdList); err != nil {
		log.Error(err, "Unable to list CRDs")
		return
	}

	for _, crd := range crdList.Items {
		// Only look for the Neptune ones
		if strings.Contains(crd.Name, "astra.netapp.io") && !strings.Contains(crd.Name, "astraconnectors.astra.netapp.io") {
			gv := strings.Split(crd.Spec.Group, "/")
			gvr := schema.GroupVersionResource{
				Group:    gv[0],
				Version:  crd.Spec.Versions[0].Name,
				Resource: strings.ToLower(crd.Spec.Names.Plural),
			}

			log.Info("Deleting resources", "GVR", gvr)

			// Delete all resources of this CRD kind in the namespace AstraConnector is installed
			if err := r.DynamicClient.Resource(gvr).Namespace(namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{}); err != nil {
				log.Error(err, "Unable to delete resources", "Resource", gvr.Resource)
				return
			}
		}
	}
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

func (r *AstraConnectorController) needsReconcile(connector v1.AstraConnector, log logr.Logger) bool {
	// Ensure that the cluster has registered successfully
	if connector.Status.NatsSyncClient.Registered != "true" {
		return true
	}
	// Ensure that the CR spec has not changed between reconciles
	if connector.Status.ObservedSpec != connector.Spec {
		return true
	}
	return false
}
