package controllers

import (
	"context"
	"github.com/NetApp-Polaris/astra-connector-operator/app/conf"
	"github.com/NetApp-Polaris/astra-connector-operator/app/trident"
	trident_k8s "github.com/NetApp-Polaris/astra-connector-operator/app/trident/kubernetes"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"time"
)

func (r *AstraConnectorController) deployACP(ctx context.Context,
	astraConnector *v1.AstraConnector, natsSyncClientStatus *v1.NatsSyncClientStatus) (ctrl.Result, error) {

	// TODO CRD will be installed as part of our crd install
	// check if they are installed if not error here or maybe a pre-check

	// Deploy ACP
	cfg, err := config.GetConfig()
	if err != nil {
		// Failed deploying we want status to reflect that for at least 30 seconds before it's requeued so
		// anyone watching can be informed
		return ctrl.Result{RequeueAfter: time.Minute * conf.Config.ErrorTimeout()}, err
	}

	// todo Oscar read ns from spec
	clients, err  := trident_k8s.CreateK8SClients(cfg, "trident")
	if err != nil {
		// Failed deploying we want status to reflect that for at least 30 seconds before it's requeued so
		// anyone watching can be informed
		return ctrl.Result{RequeueAfter: time.Minute * conf.Config.ErrorTimeout()}, err
	}

	trident.NewInstaller()

	trident_k8s.

	// No need to requeue due to success
	return ctrl.Result{}, nil
}
