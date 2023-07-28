package controllers

import (
	"context"
	"time"

	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/neptune"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *AstraConnectorController) deployNeptune(ctx context.Context,
	astraConnector *v1.AstraConnector, natsSyncClientStatus *v1.NatsSyncClientStatus) (ctrl.Result, error) {

	// TODO CRD will be installed as part of our crd install
	// check if they are installed if not error here or maybe a pre-check

	// Deploy Neptune
	neptuneDeployer := neptune.NewNeptuneClientDeployer()
	err := r.deployResources(ctx, neptuneDeployer, astraConnector, natsSyncClientStatus)
	if err != nil {
		// Failed deploying we want status to reflect that for at least 30 seconds before it's requeued so
		// anyone watching can be informed
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	return ctrl.Result{}, nil
}
