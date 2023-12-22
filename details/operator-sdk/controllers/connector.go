package controllers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NetApp-Polaris/astra-connector-operator/details/k8s"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/NetApp-Polaris/astra-connector-operator/app/conf"
	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/connector"
	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/model"
	"github.com/NetApp-Polaris/astra-connector-operator/app/register"
	"github.com/NetApp-Polaris/astra-connector-operator/common"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

func (r *AstraConnectorController) deployConnector(ctx context.Context,
	astraConnector *v1.AstraConnector, natsSyncClientStatus *v1.NatsSyncClientStatus) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// let's deploy Nats, NatsSyncClient and Astra Connector in that order
	connectorDeployers := getDeployers()
	for _, deployer := range connectorDeployers {
		err := r.deployResources(ctx, deployer, astraConnector, natsSyncClientStatus)
		if err != nil {
			// Failed deploying we want status to reflect that for at least 30 seconds before it's requeued so
			// anyone watching can be informed
			log.V(3).Info("Requeue after 30 seconds, so that status reflects error")
			return ctrl.Result{RequeueAfter: time.Minute * conf.Config.ErrorTimeout()}, err
		}
	}

	// Let's register the cluster now
	registerUtil := register.NewClusterRegisterUtil(astraConnector, &http.Client{}, r.Client, log, context.Background())
	registered := false
	log.Info("Checking for natsSyncClient configmap")
	foundCM := &corev1.ConfigMap{}
	astraConnectorID := ""
	err := r.Get(ctx, types.NamespacedName{Name: common.NatsSyncClientConfigMapName, Namespace: astraConnector.Namespace}, foundCM)
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Minute * conf.Config.ErrorTimeout()}, err
	}
	if len(foundCM.Data) != 0 {
		registered = true
		astraConnectorID, err = registerUtil.GetConnectorIDFromConfigMap(foundCM.Data)
		if err != nil {
			log.Error(err, FailedLocationIDGet)
			natsSyncClientStatus.Status = FailedLocationIDGet
			_ = r.updateAstraConnectorStatus(ctx, astraConnector, *natsSyncClientStatus)
			return ctrl.Result{RequeueAfter: time.Minute * conf.Config.ErrorTimeout()}, err
		}
		if astraConnectorID == "" {
			log.Error(err, EmptyLocationIDGet)
			natsSyncClientStatus.Status = EmptyLocationIDGet
			_ = r.updateAstraConnectorStatus(ctx, astraConnector, *natsSyncClientStatus)
			return ctrl.Result{RequeueAfter: time.Minute * conf.Config.ErrorTimeout()}, err
		}
	}

	// RegisterClient
	if !astraConnector.Spec.Astra.Unregister {
		// Check the feature flag first
		if conf.Config.FeatureFlags().SkipAstraRegistration() {
			log.Info("Skipping Nats and Astra registration, feature flag set to not register")
			natsSyncClientStatus.Status = DeployedComponents
			_ = r.updateAstraConnectorStatus(ctx, astraConnector, *natsSyncClientStatus)
			return ctrl.Result{RequeueAfter: time.Minute * conf.Config.ErrorTimeout()}, nil
		}

		if registered {
			log.Info("natsSyncClient already registered", "astraConnectorID", astraConnectorID)
		} else {
			log.Info("Registering natsSyncClient")
			astraConnectorID, errorReason, err := registerUtil.RegisterNatsSyncClient()
			if err != nil {
				log.Error(err, FailedRegisterNSClient)
				natsSyncClientStatus.Status = errorReason
				_ = r.updateAstraConnectorStatus(ctx, astraConnector, *natsSyncClientStatus)
				return ctrl.Result{RequeueAfter: time.Minute * conf.Config.ErrorTimeout()}, err
			}

			log.Info("natsSyncClient ConnectorID", "astraConnectorID", astraConnectorID)
		}
		natsSyncClientStatus.AstraConnectorID = astraConnectorID
		natsSyncClientStatus.Status = RegisterNSClient

		if astraConnector.Spec.Astra.TokenRef == "" || astraConnector.Spec.Astra.AccountId == "" || astraConnector.Spec.Astra.ClusterName == "" {
			log.Info("Skipping cluster registration with Astra, incomplete Astra details provided TokenRef/AccountId/ClusterName")
		} else {
			log.Info("Registering cluster with Astra")

			// Check if there is a cluster ID from:
			// 1. CR Status. If it is in hear it means we have already been through this loop once and know what the ID is
			// 2. Check the CR Spec. If it is in here, use it. It will be validated later.
			// 3. If the clusterID is in neither of the above, leave it "" and the operator will create one and populate the status
			// 4. Save the clusterID to the CR Status
			var clusterId string
			if strings.TrimSpace(natsSyncClientStatus.AstraClusterId) != "" {
				clusterId = natsSyncClientStatus.AstraClusterId
				log.WithValues("clusterID", clusterId).Info("using clusterID from CR Status")
			} else {
				clusterId = astraConnector.Spec.Astra.ClusterId
			}

			var errorReason string
			natsSyncClientStatus.AstraClusterId, errorReason, err = registerUtil.RegisterClusterWithAstra(astraConnectorID, clusterId)
			if err != nil {
				log.Error(err, FailedConnectorIDAdd)
				natsSyncClientStatus.Status = errorReason
				_ = r.updateAstraConnectorStatus(ctx, astraConnector, *natsSyncClientStatus)
				return ctrl.Result{RequeueAfter: time.Minute * conf.Config.ErrorTimeout()}, err
			}
			log.Info("Registered cluster with Astra")
		}
		natsSyncClientStatus.Registered = "true"
		natsSyncClientStatus.Status = "Registered with Astra"
	} else {
		if registered {
			log.Info("Unregistering natsSyncClient")
			err = registerUtil.UnRegisterNatsSyncClient()
			if err != nil {
				log.Error(err, FailedUnRegisterNSClient)
				natsSyncClientStatus.Status = FailedUnRegisterNSClient
				_ = r.updateAstraConnectorStatus(ctx, astraConnector, *natsSyncClientStatus)
				return ctrl.Result{RequeueAfter: time.Minute * conf.Config.ErrorTimeout()}, err
			}
			log.Info(UnregisterNSClient)
		} else {
			log.Info("Already unregistered with Astra")
		}
		natsSyncClientStatus.Registered = "false"
		natsSyncClientStatus.AstraConnectorID = ""
		natsSyncClientStatus.Status = UnregisterNSClient
	}

	// if we are registered and have a clusterid let's set up the asup cr
	if natsSyncClientStatus.Registered == "true" && natsSyncClientStatus.AstraClusterId != "" {
		err = createASUPCR(ctx, astraConnector, r.Client, natsSyncClientStatus.AstraClusterId)
		if err != nil {
			log.Error(err, FailedASUPCreation)
			natsSyncClientStatus.Status = FailedASUPCreation
			_ = r.updateAstraConnectorStatus(ctx, astraConnector, *natsSyncClientStatus)
			return ctrl.Result{RequeueAfter: time.Minute * conf.Config.ErrorTimeout()}, err
		}
	}

	// No need to requeue due to success
	return ctrl.Result{}, nil
}

func getDeployers() []model.Deployer {
	return []model.Deployer{connector.NewNatsDeployer(), connector.NewNatsSyncClientDeployer(), connector.NewAstraConnectorDeployer()}
}

func (r *AstraConnectorController) deleteConnectorClusterScopedResources(ctx context.Context, astraConnector *v1.AstraConnector) {
	connectorDeployers := getDeployers()
	for _, deployer := range connectorDeployers {
		r.deleteClusterScopedResources(ctx, deployer, astraConnector)
	}
}

func createASUPCR(ctx context.Context, astraConnector *v1.AstraConnector, client client.Client, astraClusterID string) error {
	log := ctrllog.FromContext(ctx)
	k8sUtil := k8s.NewK8sUtil(client, log)

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

	return k8sUtil.CreateOrUpdateResource(ctx, cr, astraConnector)
}
