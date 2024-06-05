package controllers

import (
	"context"
	"fmt"
	"github.com/NetApp-Polaris/astra-connector-operator/app/conf"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/model"
	"github.com/NetApp-Polaris/astra-connector-operator/details/k8s"
	installer "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

// getK8sResources of function type
type getK8sResources func(model.Deployer, *installer.AstraConnector, context.Context) ([]client.Object, controllerutil.MutateFn, error)

type createResourceParams struct {
	getResource   getK8sResources
	createMessage string
	errorMessage  string
	clusterScope  bool
}

const ()

// ResourcesToDeploy This is a list and order matters since things will be created in the order specified
var resources = []createResourceParams{
	{createMessage: CreateConfigMap, errorMessage: ErrorCreateConfigMaps, getResource: model.Deployer.GetConfigMapObjects, clusterScope: false},
	{createMessage: CreateRole, errorMessage: ErrorCreateRoles, getResource: model.Deployer.GetRoleObjects, clusterScope: false},
	{createMessage: CreateClusterRole, errorMessage: ErrorCreateClusterRoles, getResource: model.Deployer.GetClusterRoleObjects, clusterScope: true},
	{createMessage: CreateRoleBinding, errorMessage: ErrorCreateRoleBindings, getResource: model.Deployer.GetRoleBindingObjects, clusterScope: false},
	{createMessage: CreateClusterRoleBinding, errorMessage: ErrorCreateClusterRoleBindings, getResource: model.Deployer.GetClusterRoleBindingObjects, clusterScope: true},
	{createMessage: CreateServiceAccount, errorMessage: ErrorCreateServiceAccounts, getResource: model.Deployer.GetServiceAccountObjects, clusterScope: false},
	{createMessage: CreateStatefulSet, errorMessage: ErrorCreateStatefulSets, getResource: model.Deployer.GetStatefulSetObjects, clusterScope: false},
	{createMessage: CreateService, errorMessage: ErrorCreateService, getResource: model.Deployer.GetServiceObjects, clusterScope: false},
	{createMessage: CreateDeployment, errorMessage: ErrorCreateDeployments, getResource: model.Deployer.GetDeploymentObjects, clusterScope: false},
}

func (r *AstraConnectorController) deployResources(ctx context.Context, deployer model.Deployer, astraConnector *installer.AstraConnector, astraConnectorStatus *installer.AstraConnectorStatus) error {
	log := ctrllog.FromContext(ctx)
	k8sUtil := k8s.NewK8sUtil(r.Client, r.Clientset, log)

	for _, funcList := range resources {

		resourceList, mutateFunc, err := funcList.getResource(deployer, astraConnector, ctx)
		if err != nil {
			return errors.Wrapf(err, "Unable to get resource")
		}
		if resourceList == nil {
			continue
		}

		for _, kubeObject := range resourceList {
			key := client.ObjectKeyFromObject(kubeObject)
			statusMsg := fmt.Sprintf(funcList.createMessage, key.Namespace, key.Name)
			log.Info(statusMsg)
			astraConnectorStatus.Status = statusMsg
			_ = r.updateAstraConnectorStatus(ctx, astraConnector, *astraConnectorStatus)

			result, err := k8sUtil.CreateOrUpdateResource(ctx, kubeObject, astraConnector, mutateFunc)
			if err != nil {
				return r.formatError(ctx, astraConnector, log, funcList.errorMessage, key.Namespace, key.Name, err, astraConnectorStatus)
			} else {
				waitCtx, cancel := context.WithCancel(ctx)
				defer cancel()
				err = r.waitForResourceReady(waitCtx, kubeObject, astraConnector)
				if err != nil {
					return r.formatError(ctx, astraConnector, log, funcList.errorMessage, key.Namespace, key.Name, err, astraConnectorStatus)
				}
				log.Info(fmt.Sprintf("Successfully %s resources", result))
			}
		}

	}
	return nil
}

func (r *AstraConnectorController) deleteClusterScopedResources(ctx context.Context, deployer model.Deployer, astraConnector *installer.AstraConnector) {
	log := ctrllog.FromContext(ctx)
	k8sUtil := k8s.NewK8sUtil(r.Client, r.Clientset, log)

	for _, funcList := range resources {
		if !funcList.clusterScope {
			// Skip non-cluster scoped resources
			continue
		}

		resourceList, _, _ := funcList.getResource(deployer, astraConnector, ctx)
		if resourceList == nil {
			continue
		}

		for _, kubeObject := range resourceList {
			key := client.ObjectKeyFromObject(kubeObject)
			objectKind := reflect.TypeOf(kubeObject).String()

			log.WithValues("name", key.Name, "kind", objectKind).Info("Deleting resource")
			err := k8sUtil.DeleteResource(ctx, kubeObject)
			if err != nil {
				log.WithValues("name", key.Name, "kind", objectKind).Error(err, "error deleting resource")
				return
			}
			log.WithValues("name", key.Name, "kind", objectKind).Info("Deleted resource")
		}
	}
}

func (r *AstraConnectorController) waitForResourceReady(ctx context.Context, kubeObject client.Object, astraConnector *installer.AstraConnector) error {
	log := ctrllog.FromContext(ctx)
	timeout := time.After(conf.Config.WaitDurationForResource()) // default is 2 mins
	ticker := time.NewTicker(3 * time.Second)
	originalSpec := astraConnector.Spec

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("reconcile cancelled")
		case <-timeout:
			return fmt.Errorf("timed out waiting for resource %s/%s to be ready", kubeObject.GetNamespace(), kubeObject.GetName())
		case <-ticker.C:
			// First lets make sure controller isn't modified, this check allow us
			// to respond faster when there is a cr spec change or deletion
			if r.isControllerModified(ctx, astraConnector, originalSpec) {
				return fmt.Errorf("controller updated, requeue to handle changes")
			}

			err := r.Client.Get(ctx, client.ObjectKeyFromObject(kubeObject), kubeObject)
			if err != nil {
				log.Error(err, "Error getting resource", "namespace", kubeObject.GetNamespace(), "name", kubeObject.GetName())
				continue
			}

			isReady := false
			switch obj := kubeObject.(type) {
			case *appsv1.Deployment:
				isReady = obj.Status.ReadyReplicas == obj.Status.Replicas
			case *appsv1.StatefulSet:
				isReady = obj.Status.ReadyReplicas == obj.Status.Replicas && obj.Status.CurrentReplicas == obj.Status.Replicas
			default:
				isReady = true
			}

			if isReady {
				log.Info("Resource is ready", "namespace", kubeObject.GetNamespace(), "name", kubeObject.GetName())
				return nil
			}
		}
	}
}

func (r *AstraConnectorController) formatError(ctx context.Context, astraConnector *installer.AstraConnector,
	log logr.Logger, errorMessage, namespace, name string, err error,
	astraConnectorStatus *installer.AstraConnectorStatus) error {
	statusMsg := fmt.Sprintf(errorMessage, namespace, name)
	astraConnectorStatus.Status = statusMsg
	_ = r.updateAstraConnectorStatus(ctx, astraConnector, *astraConnectorStatus)
	log.Error(err, statusMsg)
	return errors.Wrapf(err, statusMsg)
}

func (r *AstraConnectorController) isControllerModified(ctx context.Context,
	astraConnector *installer.AstraConnector, originalSpec installer.AstraConnectorSpec) bool {
	log := ctrllog.FromContext(ctx)
	// Fetch the AstraConnector instance
	controllerKey := client.ObjectKeyFromObject(astraConnector)
	updatedAstraConnector := &installer.AstraConnector{}
	err := r.Get(ctx, controllerKey, updatedAstraConnector)
	if err != nil {
		log.Info("AstraConnector resource not found. Ignoring since object must be deleted")
		return true
	}

	if updatedAstraConnector.GetDeletionTimestamp() != nil {
		log.Info("AstraConnector marked for deletion, reconciler requeue")
		return true
	}

	if !reflect.DeepEqual(updatedAstraConnector.Spec, originalSpec) {
		log.Info("AstraConnector spec change, reconciler requeue")
		return true
	}
	return false
}
