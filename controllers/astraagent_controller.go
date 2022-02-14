/*
Copyright 2022 NetApp, Inc. All Rights Reserved.

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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime"

	cachev1 "github.com/NetApp/astraagent-operator/api/v1"
	"github.com/NetApp/astraagent-operator/common"
	"github.com/NetApp/astraagent-operator/natssync_client"
	"github.com/NetApp/astraagent-operator/register"
)

// AstraAgentReconciler reconciles a AstraAgent object
type AstraAgentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=netapp.astraagent.com,resources=astraagents,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=netapp.astraagent.com,resources=astraagents/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=netapp.astraagent.com,resources=astraagents/finalizers,verbs=update
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
		if k8serrors.IsNotFound(err) {
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

	if !astraAgent.Spec.Astra.AcceptEULA {
		log.Info("End User License Agreement set to false, will not proceed with the install")
		return ctrl.Result{}, nil
	}

	// name of our custom finalizer
	finalizerName := "astraagent.com/finalizer"
	// examine DeletionTimestamp to determine if object is under deletion
	if astraAgent.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(astraAgent, finalizerName) {
			log.Info("Adding finalizer to AstraAgent instance", "finalizerName", finalizerName)
			controllerutil.AddFinalizer(astraAgent, finalizerName)
			if err := r.Update(ctx, astraAgent); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(astraAgent, finalizerName) {
			// our finalizer is present, so lets handle any external dependency
			log.Info("Unregistering the cluster with Astra upon CRD delete")
			err = register.RemoveLocationIDFromCloudExtension(astraAgent, ctx)
			if err != nil {
				log.Error(err, "Failed to unregister the cluster with Astra, ignoring...")
			} else {
				log.Info("Unregistered the cluster with Astra upon CRD delete")
			}

			log.Info("Unregistering natssync-client upon CRD delete")
			err = register.UnregisterClient(astraAgent)
			if err != nil {
				log.Error(err, "Failed to unregister natssync-client, ignoring...")
			} else {
				log.Info("Unregistered natssync-client upon CRD delete")
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(astraAgent, finalizerName)
			if err := r.Update(ctx, astraAgent); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	err = r.CreateServices(astraAgent, ctx)
	if err != nil {
		log.Error(err, "Error creating services")
		return ctrl.Result{}, err
	}

	err = r.CreateConfigMaps(astraAgent, ctx)
	if err != nil {
		log.Error(err, "Error creating configmaps")
		return ctrl.Result{}, err
	}

	err = r.CreateRoles(astraAgent, ctx)
	if err != nil {
		log.Error(err, "Error creating roles")
		return ctrl.Result{}, err
	}

	err = r.CreateRoleBindings(astraAgent, ctx)
	if err != nil {
		log.Error(err, "Error creating role bindings")
		return ctrl.Result{}, err
	}

	err = r.CreateServiceAccounts(astraAgent, ctx)
	if err != nil {
		log.Error(err, "Error creating service accounts")
		return ctrl.Result{}, err
	}

	err = r.CreateStatefulSets(astraAgent, ctx)
	if err != nil {
		log.Error(err, "Error creating stateful sets")
		return ctrl.Result{}, err
	}

	err = r.CreateDeployments(astraAgent, ctx)
	if err != nil {
		log.Error(err, "Error creating deployments")
		return ctrl.Result{}, err
	}

	registered := false
	log.Info("Checking for natssync-client configmap")
	foundCM := &corev1.ConfigMap{}
	locationID := ""
	err = r.Get(ctx, types.NamespacedName{Name: common.NatssyncClientConfigMapName, Namespace: astraAgent.Namespace}, foundCM)
	if len(foundCM.Data) != 0 {
		registered = true
		locationID, err = register.GetLocationIDFromConfigMap(foundCM.Data)
		if err != nil {
			log.Error(err, "Failed to get the location ID from configmap")
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
		}
		if locationID == "" {
			log.Error(err, "Got an empty location ID from configmap")
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
		}
	}

	// RegisterClient
	if !astraAgent.Spec.Astra.Unregister {
		if registered {
			log.Info("natssync-client already registered", "locationID", locationID)
		} else {
			log.Info("Registering natssync-client")
			locationID, err = register.RegisterClient(astraAgent)
			if err != nil {
				log.Error(err, "Failed to register natssync-client")
				return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
			}
			log.Info("natssync-client locationID", "locationID", locationID)
		}

		log.Info("Registering locationID with Astra")
		err = register.AddLocationIDtoCloudExtension(astraAgent, locationID, ctx)
		if err != nil {
			log.Error(err, "Failed to register locationID with Astra")
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
		}
		log.Info("Registered locationID with Astra")
	} else {
		if registered {
			log.Info("Unregistering the cluster with Astra")
			err = register.RemoveLocationIDFromCloudExtension(astraAgent, ctx)
			if err != nil {
				log.Error(err, "Failed to unregister the cluster with Astra")
				return ctrl.Result{Requeue: true}, err
			}
			log.Info("Unregistered the cluster with Astra")

			log.Info("Unregistering natssync-client")
			err = register.UnregisterClient(astraAgent)
			if err != nil {
				log.Error(err, "Failed to unregister natssync-client")
				return ctrl.Result{Requeue: true}, err
			}
			log.Info("Unregistered natssync-client")
		} else {
			log.Info("Already unregistered with Astra")
		}
	}

	// Update the astraAgent status with the pod names
	// List the pods for this astraAgent's deployment
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(astraAgent.Namespace),
	}
	if err = r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "Namespace", astraAgent.Namespace)
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
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	natssyncClientStatus, err := r.getNatssyncClientStatus(astraAgent, ctx)
	if err != nil {
		log.Error(err, "Failed to get natssync-client status")
		return ctrl.Result{}, err
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

// contains checks if a string is present in a string array
func contains(strArr []string, input string) bool {
	for _, s := range strArr {
		if s == input {
			return true
		}
	}
	return false
}

// getNatssyncClientStatus returns NatssyncClientStatus object
func (r *AstraAgentReconciler) getNatssyncClientStatus(m *cachev1.AstraAgent, ctx context.Context) (cachev1.NatssyncClientStatus, error) {
	pods := &corev1.PodList{}
	lb := natssync_client.LabelsForNatssyncClient(common.NatssyncClientName)
	listOpts := []client.ListOption{
		client.MatchingLabels(lb),
	}
	log := ctrllog.FromContext(ctx)

	if err := r.List(ctx, pods, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "Namespace", m.Namespace)
		return cachev1.NatssyncClientStatus{}, err
	}

	natssyncClientStatus := cachev1.NatssyncClientStatus{}

	if len(pods.Items) < 1 {
		return cachev1.NatssyncClientStatus{}, errors.New("natssync-client pods not found")
	}
	nsClientPod := pods.Items[0]
	// If a pod is terminating, then we can't access the corresponding vault node's status.
	// so we break from here and return an error.
	if nsClientPod.Status.Phase != corev1.PodRunning || nsClientPod.DeletionTimestamp != nil {
		errNew := errors.New("natssync-client not in the desired state")
		log.Error(errNew, "natssync-client pod", "Phase", nsClientPod.Status.Phase, "DeletionTimestamp", nsClientPod.DeletionTimestamp)
		return cachev1.NatssyncClientStatus{}, errNew
	}

	natssyncClientStatus.State = string(nsClientPod.Status.Phase)
	natssyncClientLocationID, err := r.getNatssyncClientRegistrationStatus(register.GetNatssyncClientRegistrationURL(m))
	if err != nil {
		log.Error(err, "Failed to get the registration status")
		return cachev1.NatssyncClientStatus{}, err
	}
	natssyncClientVersion, err := r.getNatssyncClientVersion(r.getNatssyncClientAboutURL(m))
	if err != nil {
		log.Error(err, "Failed to get the natssync-client version")
		return cachev1.NatssyncClientStatus{}, err
	}
	natssyncClientStatus.Registered = strconv.FormatBool(natssyncClientLocationID != "")
	natssyncClientStatus.LocationID = natssyncClientLocationID
	natssyncClientStatus.Version = natssyncClientVersion
	return natssyncClientStatus, nil
}

// getNatssyncClientAboutURL returns NatssyncClient About URL
func (r *AstraAgentReconciler) getNatssyncClientAboutURL(m *cachev1.AstraAgent) string {
	natsSyncClientURL := fmt.Sprintf("http://%s.%s:%d/bridge-client/1", common.NatssyncClientName, m.Namespace, common.NatssyncClientPort)
	natsSyncClientAboutURL := fmt.Sprintf("%s/about", natsSyncClientURL)
	return natsSyncClientAboutURL
}

// getNatssyncClientRegistrationStatus returns the locationID string
func (r *AstraAgentReconciler) getNatssyncClientRegistrationStatus(natsSyncClientRegisterURL string) (string, error) {
	resp, err := http.Get(natsSyncClientRegisterURL)
	if err != nil {
		return "", err
	}

	type registrationResponse struct {
		LocationID string `json:"locationID"`
	}
	var registrationResp registrationResponse
	all, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(all, &registrationResp)
	if err != nil {
		return "", err
	}

	return registrationResp.LocationID, nil
}

// getNatssyncClientVersion returns the NatssyncClient Version
func (r *AstraAgentReconciler) getNatssyncClientVersion(natsSyncClientAboutURL string) (string, error) {
	resp, err := http.Get(natsSyncClientAboutURL)
	if err != nil {
		return "", err
	}

	type aboutResponse struct {
		AppVersion string `json:"appVersion,omitempty"`
	}
	var aboutResp aboutResponse
	all, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(all, &aboutResp)
	if err != nil {
		return "", err
	}
	return aboutResp.AppVersion, nil
}
