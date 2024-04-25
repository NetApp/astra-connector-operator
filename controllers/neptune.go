package controllers

import (
	"context"
	"github.com/NetApp-Polaris/astra-connector-operator/api/v1"
	"github.com/NetApp-Polaris/astra-connector-operator/k8s"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/NetApp-Polaris/astra-connector-operator/app/conf"
	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/neptune"
)

func (r *AstraNeptuneController) deployNeptune(ctx context.Context,
	astraNeptune *v1.AstraNeptune) (ctrl.Result, error) {

	// TODO CRD will be installed as part of our crd install
	// check if they are installed if not error here or maybe a pre-check

	// Deploy Neptune
	neptuneDeployable := neptune.NewNeptuneClientDeployerV2(astraNeptune)

	deployerCtrl := NewK8sDeployer(k8s.K8sUtil{Client: r.Client, Log: ctrl.Log},
		r.Client,
		astraNeptune,
		neptuneDeployable)
	err := deployerCtrl.deployResources(ctx)
	if err != nil {
		// Failed deploying we want status to reflect that for at least 30 seconds before it's requeued so
		// anyone watching can be informed
		return ctrl.Result{RequeueAfter: time.Minute * conf.Config.ErrorTimeout()}, err
	}

	// if we are registered and have a clusterid let's set up the asup cr
	err = r.createASUPCR(ctx, astraNeptune, "1")
	if err != nil {
		ctrl.Log.Error(err, FailedASUPCreation)
		_ = r.updateAstraNeptuneStatus(ctx, astraNeptune, FailedASUPCreation)
		return ctrl.Result{RequeueAfter: time.Minute * conf.Config.ErrorTimeout()}, err
	}
	_ = r.updateAstraNeptuneStatus(ctx, astraNeptune, DeployedComponents)
	// No need to requeue due to success
	return ctrl.Result{}, nil
}

func (r *AstraNeptuneController) deleteNeptuneClusterScopedResources(ctx context.Context, astraNeptune *v1.AstraNeptune) {
	neptuneDeployable := neptune.NewNeptuneClientDeployerV2(astraNeptune)

	deployerCtrl := NewK8sDeployer(k8s.K8sUtil{Client: r.Client, Log: ctrl.Log},
		r.Client,
		astraNeptune,
		neptuneDeployable)

	deployerCtrl.deleteClusterScopedResources(ctx)
}
