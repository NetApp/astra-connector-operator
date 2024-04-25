package controllers

import (
	"context"
	"github.com/NetApp-Polaris/astra-connector-operator/api/v1"
	"github.com/NetApp-Polaris/astra-connector-operator/k8s"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/NetApp-Polaris/astra-connector-operator/app/conf"
	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/connector"
)

func (r *AstraConnectorController) deployNatlessConnector(ctx context.Context,
	astraConnector *v1.AstraConnector) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// let's deploy Astra Connector without Nats
	connectorDeployable := connector.NewAstraConnectorNatlessDeployer(*astraConnector)
	deployerCtrl := NewK8sDeployer(k8s.K8sUtil{Client: r.Client, Log: ctrl.Log},
		r.Client,
		astraConnector,
		connectorDeployable)
	err := deployerCtrl.deployResources(ctx)
	if err != nil {
		// Failed deploying we want status to reflect that for at least 30 seconds before it's requeued so
		// anyone watching can be informed
		log.V(3).Info("Requeue after 30 seconds, so that status reflects error")
		return ctrl.Result{RequeueAfter: time.Minute * conf.Config.ErrorTimeout()}, err
	}

	_ = r.updateAstraConnectorStatus(ctx, astraConnector, DeployedComponents)
	// No need to requeue due to success
	return ctrl.Result{}, nil
}

func (r *AstraConnectorController) deleteConnectorClusterScopedResources(ctx context.Context, astraConnector *v1.AstraConnector) {
	connectorDeployable := connector.NewAstraConnectorNatlessDeployer(*astraConnector)
	deployerCtrl := NewK8sDeployer(k8s.K8sUtil{Client: r.Client, Log: ctrl.Log},
		r.Client,
		astraConnector,
		connectorDeployable)
	deployerCtrl.deleteClusterScopedResources(ctx)
}
