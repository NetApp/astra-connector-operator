package controllers

import (
	"context"
	"github.com/NetApp/astraagent-operator/common"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/NetApp/astraagent-operator/deployer"

	cachev1 "github.com/NetApp/astraagent-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *AstraAgentReconciler) CreateServices(m *cachev1.AstraAgent, ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	for serviceName, deployment := range common.ServicesList {
		deployerObj, err := deployer.Factory(deployment)
		if err != nil {
			log.Error(err, "Failed to create deployer")
			return err
		}
		foundSer := &corev1.Service{}
		log.Info("Finding Service", "Namespace", m.Namespace, "Name", serviceName)
		err = r.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: m.Namespace}, foundSer)
		if err != nil && errors.IsNotFound(err) {
			// Define a new service
			serv, err := deployerObj.GetServiceObject(m, serviceName)
			if err != nil {
				log.Error(err, "Failed to get service object")
				return err
			}
			log.Info("Creating a new Service", "Namespace", serv.Namespace, "Name", serv.Name)
			err = r.Create(ctx, serv)
			if err != nil {
				log.Error(err, "Failed to create new Service", "Namespace", serv.Namespace, "Name", serv.Name)
				return err
			}
			// Set astraAgent instance as the owner and controller
			err = ctrl.SetControllerReference(m, serv, r.Scheme)
			if err != nil {
				return err
			}
		} else if err != nil {
			log.Error(err, "Failed to get Service")
			return err
		}
	}
	return nil
}
