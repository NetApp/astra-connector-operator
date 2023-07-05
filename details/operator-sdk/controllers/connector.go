package controllers

import (
	"context"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

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
			return ctrl.Result{RequeueAfter: 30 * time.Second}, err
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
		return ctrl.Result{}, err
	}
	if len(foundCM.Data) != 0 {
		registered = true
		astraConnectorID, err = registerUtil.GetConnectorIDFromConfigMap(foundCM.Data)
		if err != nil {
			log.Error(err, FailedLocationIDGet)
			natsSyncClientStatus.Status = FailedLocationIDGet
			_ = r.updateAstraConnectorStatus(ctx, astraConnector, *natsSyncClientStatus)
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
		}
		if astraConnectorID == "" {
			log.Error(err, EmptyLocationIDGet)
			natsSyncClientStatus.Status = EmptyLocationIDGet
			_ = r.updateAstraConnectorStatus(ctx, astraConnector, *natsSyncClientStatus)
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
		}
	}

	// RegisterClient
	if !astraConnector.Spec.Astra.Unregister {
		if registered {
			log.Info("natsSyncClient already registered", "astraConnectorID", astraConnectorID)
		} else {
			log.Info("Registering natsSyncClient")
			astraConnectorID, err = registerUtil.RegisterNatsSyncClient()
			if err != nil {
				log.Error(err, FailedRegisterNSClient)
				natsSyncClientStatus.Status = FailedRegisterNSClient
				_ = r.updateAstraConnectorStatus(ctx, astraConnector, *natsSyncClientStatus)
				return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
			}

			log.Info("natsSyncClient ConnectorID", "astraConnectorID", astraConnectorID)
		}
		natsSyncClientStatus.AstraConnectorID = astraConnectorID
		natsSyncClientStatus.Status = RegisterNSClient

		if astraConnector.Spec.Astra.TokenRef == "" || astraConnector.Spec.Astra.AccountId == "" || astraConnector.Spec.Astra.ClusterName == "" {
			log.Info("Skipping cluster registration with Astra, incomplete Astra details provided TokenRef/AccountId/ClusterName")
		} else {
			log.Info("Registering cluster with Astra")
			err = registerUtil.RegisterClusterWithAstra(astraConnectorID)
			if err != nil {
				log.Error(err, FailedConnectorIDAdd)
				natsSyncClientStatus.Status = FailedConnectorIDAdd
				_ = r.updateAstraConnectorStatus(ctx, astraConnector, *natsSyncClientStatus)
				return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
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
				return ctrl.Result{Requeue: true}, err
			}
			log.Info(UnregisterNSClient)
		} else {
			log.Info("Already unregistered with Astra")
		}
		natsSyncClientStatus.Registered = "false"
		natsSyncClientStatus.AstraConnectorID = ""
		natsSyncClientStatus.Status = UnregisterNSClient
	}

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
