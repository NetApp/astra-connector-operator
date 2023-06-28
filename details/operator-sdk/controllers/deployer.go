package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"reflect"
	"time"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/NetApp-Polaris/astra-connector-operator/deployer/model"
	"github.com/NetApp-Polaris/astra-connector-operator/details/k8s"
	installer "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

// getK8sResources of function type
type getK8sResources func(model.Deployer, *installer.AstraConnector, context.Context) ([]client.Object, error)

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

func (r *AstraConnectorController) deployResources(ctx context.Context, deployer model.Deployer, astraConnector *installer.AstraConnector, natsSyncClientStatus *installer.NatsSyncClientStatus) error {
	log := ctrllog.FromContext(ctx)
	k8sUtil := k8s.NewK8sUtil(r.Client, log)

	for _, funcList := range resources {

		resourceList, err := funcList.getResource(deployer, astraConnector, ctx)
		if resourceList == nil {
			continue
		}
		if err != nil {
			return errors.Wrapf(err, "Unable to get resource")
		}

		for _, kubeObject := range resourceList {

			key := client.ObjectKeyFromObject(kubeObject)
			statusMsg := fmt.Sprintf(funcList.createMessage, key.Namespace, key.Name)
			log.Info(statusMsg)
			natsSyncClientStatus.Status = statusMsg
			_ = r.updateAstraConnectorStatus(ctx, astraConnector, *natsSyncClientStatus)

			err = k8sUtil.CreateOrUpdateResource(ctx, kubeObject, astraConnector)
			if err != nil {
				return r.formatError(ctx, astraConnector, log, funcList.errorMessage, key.Namespace, key.Name, err, natsSyncClientStatus)
			} else {
				err = r.waitForResourceReady(ctx, kubeObject)
				if err != nil {
					return r.formatError(ctx, astraConnector, log, funcList.errorMessage, key.Namespace, key.Name, err, natsSyncClientStatus)
				}
				log.Info("Successfully deployed resources")
			}
		}

	}
	return nil
}

func (r *AstraConnectorController) deleteClusterScopedResources(ctx context.Context, deployer model.Deployer, astraConnector *installer.AstraConnector) {
	log := ctrllog.FromContext(ctx)
	k8sUtil := k8s.NewK8sUtil(r.Client, log)

	for _, funcList := range resources {
		if !funcList.clusterScope {
			// Skip non-cluster scoped resources
			continue
		}

		resourceList, _ := funcList.getResource(deployer, astraConnector, ctx)
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

func (r *AstraConnectorController) waitForResourceReady(ctx context.Context, kubeObject client.Object) error {
	log := ctrllog.FromContext(ctx)
	timeout := time.After(3 * time.Minute)
	ticker := time.NewTicker(3 * time.Second)

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timed out waiting for resource %s/%s to be ready", kubeObject.GetNamespace(), kubeObject.GetName())
		case <-ticker.C:
			err := r.Client.Get(ctx, client.ObjectKeyFromObject(kubeObject), kubeObject)
			if err != nil {
				log.Error(err, "Error getting resource", "namespace", kubeObject.GetNamespace(), "name", kubeObject.GetName())
				continue
			}

			isReady := false
			switch obj := kubeObject.(type) {
			case *appsv1.Deployment:
				isReady = obj.Status.ReadyReplicas == obj.Status.Replicas
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
	natsSyncClientStatus *installer.NatsSyncClientStatus) error {
	statusMsg := fmt.Sprintf(errorMessage, namespace, name)
	natsSyncClientStatus.Status = statusMsg
	_ = r.updateAstraConnectorStatus(ctx, astraConnector, *natsSyncClientStatus)
	log.Error(err, statusMsg)
	return errors.Wrapf(err, statusMsg)
}
