package controllers

import (
	"context"
	"fmt"

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
}

// ResourcesToDeploy This is a list and order matters since things will be created in the order specified
var resourcesToDeploy = []createResourceParams{
	{createMessage: CreateConfigMap, errorMessage: ErrorCreateConfigMaps, getResource: model.Deployer.GetConfigMapObjects},
	{createMessage: CreateRole, errorMessage: ErrorCreateRoles, getResource: model.Deployer.GetRoleObjects},
	{createMessage: CreateClusterRole, errorMessage: ErrorCreateClusterRoles, getResource: model.Deployer.GetClusterRoleObjects},
	{createMessage: CreateRoleBinding, errorMessage: ErrorCreateRoleBindings, getResource: model.Deployer.GetRoleBindingObjects},
	{createMessage: CreateClusterRoleBinding, errorMessage: ErrorCreateClusterRoleBindings, getResource: model.Deployer.GetClusterRoleBindingObjects},
	{createMessage: CreateServiceAccount, errorMessage: ErrorCreateServiceAccounts, getResource: model.Deployer.GetServiceAccountObjects},
	{createMessage: CreateStatefulSet, errorMessage: ErrorCreateStatefulSets, getResource: model.Deployer.GetStatefulSetObjects},
	{createMessage: CreateService, errorMessage: ErrorCreateService, getResource: model.Deployer.GetServiceObjects},
	{createMessage: CreateDeployment, errorMessage: ErrorCreateDeployments, getResource: model.Deployer.GetDeploymentObjects},
}

func (r *AstraConnectorController) deployResources(ctx context.Context, deployer model.Deployer, astraConnector *installer.AstraConnector, natsSyncClientStatus *installer.NatsSyncClientStatus) error {
	log := ctrllog.FromContext(ctx)
	k8sUtil := k8s.NewK8sUtil(r.Client, log)

	for _, funcList := range resourcesToDeploy {

		resourceList, _ := funcList.getResource(deployer, astraConnector, ctx)
		if resourceList == nil {
			continue
		}

		for _, kubeObject := range resourceList {

			key := client.ObjectKeyFromObject(kubeObject)
			statusMsg := fmt.Sprintf(funcList.createMessage, key.Namespace, key.Name)
			log.Info(statusMsg)
			natsSyncClientStatus.Status = statusMsg
			_ = r.updateAstraConnectorStatus(ctx, astraConnector, *natsSyncClientStatus)

			err := k8sUtil.CreateOrUpdateResource(ctx, kubeObject, astraConnector)
			if err != nil {
				statusMsg = fmt.Sprintf(funcList.errorMessage, key.Namespace, key.Name)
				log.Info(statusMsg)
				natsSyncClientStatus.Status = statusMsg
				_ = r.updateAstraConnectorStatus(ctx, astraConnector, *natsSyncClientStatus)
				log.Error(err, statusMsg)
				return errors.Wrapf(err, statusMsg)
			} else {
				log.Info("Successfully deployed resources")
			}
		}

	}
	return nil
}
