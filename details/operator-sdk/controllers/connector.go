package controllers

import (
	"context"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/NetApp-Polaris/astra-connector-operator/common"
	"github.com/NetApp-Polaris/astra-connector-operator/deployer/connector"
	"github.com/NetApp-Polaris/astra-connector-operator/deployer/model"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
	"github.com/NetApp-Polaris/astra-connector-operator/register"
)

func (r *AstraConnectorController) deployConnector(ctx context.Context,
	astraConnector *v1.AstraConnector, natssyncClientStatus *v1.NatssyncClientStatus) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	// k8sUtil := k8s.NewK8sUtil(ctx, r.Client, log)

	// let's deploy Nats, NatsSyncClient and Astra Connector in that order
	connectorDeployers := []model.Deployer{connector.NewNatsDeployer(), connector.NewNatsSyncClientDeployer(), connector.NewAstraConnectorDeployer()}
	for _, deployer := range connectorDeployers {
		err := r.deployResources(ctx, deployer, astraConnector, natssyncClientStatus)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Let's register the cluster now
	registerUtil := register.NewClusterRegisterUtil(astraConnector, &http.Client{}, r.Client, log, context.Background())
	registered := false
	log.Info("Checking for natssync-client configmap")
	foundCM := &corev1.ConfigMap{}
	astraConnectorID := ""
	err := r.Get(ctx, types.NamespacedName{Name: common.NatssyncClientConfigMapName, Namespace: astraConnector.Namespace}, foundCM)
	if len(foundCM.Data) != 0 {
		registered = true
		astraConnectorID, err = registerUtil.GetConnectorIDFromConfigMap(foundCM.Data)
		if err != nil {
			log.Error(err, FailedLocationIDGet)
			natssyncClientStatus.Status = FailedLocationIDGet
			r.updateAstraConnectorStatus(ctx, astraConnector, *natssyncClientStatus)
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
		}
		if astraConnectorID == "" {
			log.Error(err, EmptyLocationIDGet)
			natssyncClientStatus.Status = EmptyLocationIDGet
			r.updateAstraConnectorStatus(ctx, astraConnector, *natssyncClientStatus)
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
		}
	}

	// RegisterClient
	if !astraConnector.Spec.ConnectorSpec.Astra.Unregister {
		if registered {
			log.Info("natssync-client already registered", "astraConnectorID", astraConnectorID)
		} else {
			log.Info("Registering natssync-client")
			astraConnectorID, err = registerUtil.RegisterNatsSyncClient()
			if err != nil {
				log.Error(err, FailedRegisterNSClient)
				natssyncClientStatus.Status = FailedRegisterNSClient
				r.updateAstraConnectorStatus(ctx, astraConnector, *natssyncClientStatus)
				return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
			}

			log.Info("natssync-client ConnectorID", "astraConnectorID", astraConnectorID)
		}
		natssyncClientStatus.AstraConnectorID = astraConnectorID
		natssyncClientStatus.Status = RegisterNSClient

		if astraConnector.Spec.ConnectorSpec.Astra.Token == "" || astraConnector.Spec.ConnectorSpec.Astra.AccountID == "" || astraConnector.Spec.ConnectorSpec.Astra.ClusterName == "" {
			log.Info("Skipping cluster registration with Astra, incomplete Astra details provided Token/AccountID/ClusterName")
		} else {
			log.Info("Registering cluster with Astra")
			err = registerUtil.RegisterClusterWithAstra(astraConnectorID)
			if err != nil {
				log.Error(err, FailedConnectorIDAdd)
				natssyncClientStatus.Status = FailedConnectorIDAdd
				r.updateAstraConnectorStatus(ctx, astraConnector, *natssyncClientStatus)
				return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
			}
			log.Info("Registered cluster with Astra")
		}
		natssyncClientStatus.Registered = "true"
		natssyncClientStatus.Status = "Registered with Astra"
	} else {
		if registered {
			// TODO: When unregistering, we should not be unManaging and deleting the cluster
			// TODO: resource from the Astra, as this would remove all the backups and everything
			// TODO: Instead we only unregister the Nats Sync Client
			//if astraConnector.Spec.ConnectorSpec.Astra.Token == "" || astraConnector.Spec.ConnectorSpec.Astra.AccountID == "" {
			//	log.Info("Skipping cluster unregister with Astra, incomplete Astra details provided Token/AccountID")
			//} else {
			//	if astraConnector.Spec.ConnectorSpec.Astra.ClusterName != "" {
			//		log.Info("Unregistering the cluster with Astra")
			//		err = registerUtil.DeleteClusterFromAstra()
			//		if err != nil {
			//			log.Error(err, FailedConnectorIDRemove)
			//			natssyncClientStatus.Status = FailedConnectorIDRemove
			//			r.updateAstraConnectorStatus(ctx, astraConnector, natssyncClientStatus)
			//			return ctrl.Result{Requeue: true}, err
			//		}
			//		natssyncClientStatus.Status = UnregisterFromAstra
			//		log.Info(UnregisterFromAstra)
			//	} else {
			//		log.Info("Skipping unregistering the Astra cluster, no cluster name available")
			//	}
			//}

			log.Info("Unregistering natssync-client")
			err = registerUtil.UnRegisterNatsSyncClient()
			if err != nil {
				log.Error(err, FailedUnRegisterNSClient)
				natssyncClientStatus.Status = FailedUnRegisterNSClient
				r.updateAstraConnectorStatus(ctx, astraConnector, *natssyncClientStatus)
				return ctrl.Result{Requeue: true}, err
			}
			log.Info(UnregisterNSClient)
		} else {
			log.Info("Already unregistered with Astra")
		}
		natssyncClientStatus.Registered = "false"
		natssyncClientStatus.AstraConnectorID = ""
		natssyncClientStatus.Status = UnregisterNSClient
	}

	return ctrl.Result{}, nil
}
