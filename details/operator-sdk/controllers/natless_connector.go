package controllers

import (
	"context"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/NetApp-Polaris/astra-connector-operator/app/conf"
	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/connector"
	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/model"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

func (r *AstraConnectorController) deployNatlessConnector(ctx context.Context,
	astraConnector *v1.AstraConnector, natsSyncClientStatus *v1.NatsSyncClientStatus) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// let's deploy Astra Connector without Nats
	connectorDeployers := getNatlessDeployers()
	for _, deployer := range connectorDeployers {
		err := r.deployResources(ctx, deployer, astraConnector, natsSyncClientStatus)
		if err != nil {
			// Failed deploying we want status to reflect that for at least 30 seconds before it's requeued so
			// anyone watching can be informed
			log.V(3).Info("Requeue after 30 seconds, so that status reflects error")
			return ctrl.Result{RequeueAfter: time.Minute * conf.Config.ErrorTimeout()}, err
		}
	}

	// if we are registered and have a clusterid let's set up the asup cr
	err := r.createASUPCR(ctx, astraConnector, astraConnector.Spec.Astra.ClusterId)
	if err != nil {
		log.Error(err, FailedASUPCreation)
		natsSyncClientStatus.Status = FailedASUPCreation
		_ = r.updateAstraConnectorStatus(ctx, astraConnector, *natsSyncClientStatus)
		return ctrl.Result{RequeueAfter: time.Minute * conf.Config.ErrorTimeout()}, err
	}

	natsSyncClientStatus.Registered = "true"
	natsSyncClientStatus.AstraConnectorID = "n/a"
	natsSyncClientStatus.AstraClusterId = astraConnector.Spec.Astra.ClusterId
	natsSyncClientStatus.Status = RegisteredWithAstra
	_ = r.updateAstraConnectorStatus(ctx, astraConnector, *natsSyncClientStatus)

	// No need to requeue due to success
	return ctrl.Result{}, nil
}

func getNatlessDeployers() []model.Deployer {
	return []model.Deployer{connector.NewAstraConnectorNatlessDeployer()}
}
