package controllers

import (
	"context"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/NetApp-Polaris/astra-connector-operator/app/conf"
	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/neptune"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

func (r *AstraConnectorController) deployNeptune(ctx context.Context,
	astraConnector *v1.AstraConnector, astraConnectorStatus *v1.AstraConnectorStatus) (ctrl.Result, error) {

	// TODO CRD will be installed as part of our crd install
	// check if they are installed if not error here or maybe a pre-check

	// Deploy Neptune
	neptuneDeployer := neptune.NewNeptuneClientDeployerV2()
	err := r.deployResources(ctx, neptuneDeployer, astraConnector, astraConnectorStatus)
	if err != nil {
		// Failed deploying we want status to reflect that for at least 30 seconds before it's requeued so
		// anyone watching can be informed
		return ctrl.Result{RequeueAfter: time.Minute * conf.Config.ErrorTimeout()}, err
	}

	// No need to requeue due to success
	return ctrl.Result{}, nil
}
