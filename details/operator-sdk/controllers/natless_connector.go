package controllers

import (
	"context"
	"fmt"
	"github.com/NetApp-Polaris/astra-connector-operator/details/k8s"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/NetApp-Polaris/astra-connector-operator/app/conf"
	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/connector"
	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/model"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

func (r *AstraConnectorController) deployNatlessConnector(ctx context.Context,
	astraConnector *v1.AstraConnector, astraConnectorStatus *v1.AstraConnectorStatus) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// let's deploy Astra Connector
	connectorDeployers := getDeployers()
	for _, deployer := range connectorDeployers {
		err := r.deployResources(ctx, deployer, astraConnector, astraConnectorStatus)
		if err != nil {
			// Failed deploying we want status to reflect that for at least 30 seconds before it's requeued so
			// anyone watching can be informed
			log.V(3).Info("Requeue after 30 seconds, so that status reflects error")
			return ctrl.Result{RequeueAfter: time.Minute * conf.Config.ErrorTimeout()}, err
		}
	}

	// No need to requeue due to success
	return ctrl.Result{}, nil
}

func getDeployers() []model.Deployer {
	return []model.Deployer{connector.NewAstraConnectorDeployer()}
}

func (r *AstraConnectorController) createASUPCR(ctx context.Context, astraConnector *v1.AstraConnector, astraClusterID string) error {
	log := ctrllog.FromContext(ctx)
	k8sUtil := k8s.NewK8sUtil(r.Client, r.Clientset, log)

	cr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "astra.netapp.io/v1",
			"kind":       "AutoSupportBundleSchedule",
			"metadata": map[string]interface{}{
				"name":      "asupbundleschedule-" + astraClusterID,
				"namespace": astraConnector.Namespace,
			},
			"spec": map[string]interface{}{
				"enabled": astraConnector.Spec.AutoSupport.Enrolled,
			},
		},
	}
	// Define the MutateFn function
	mutateFn := func() error {
		cr.Object["spec"].(map[string]interface{})["enabled"] = astraConnector.Spec.AutoSupport.Enrolled
		return nil
	}
	result, err := k8sUtil.CreateOrUpdateResource(ctx, cr, astraConnector, mutateFn)
	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Successfully %s AutoSupportBundleSchedule", result))
	return nil
}

func (r *AstraConnectorController) deleteConnectorClusterScopedResources(ctx context.Context, astraConnector *v1.AstraConnector) {
	connectorDeployers := getDeployers()
	for _, deployer := range connectorDeployers {
		r.deleteClusterScopedResources(ctx, deployer, astraConnector)
	}
}
