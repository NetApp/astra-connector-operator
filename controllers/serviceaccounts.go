package controllers

import (
	"context"
	"github.com/NetApp/astraagent-operator/common"
	"github.com/NetApp/astraagent-operator/deployer"
	ctrl "sigs.k8s.io/controller-runtime"

	cachev1 "github.com/NetApp/astraagent-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *AstraAgentReconciler) CreateServiceAccounts(m *cachev1.AstraAgent, ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	for saName, deploymentName := range common.ServiceAccountsList {
		foundSA := &corev1.ServiceAccount{}
		deployerObj, err := deployer.Factory(deploymentName)
		if err != nil {
			log.Error(err, "Failed to create deployer")
			return err
		}
		log.Info("Finding ServiceAccount", "Namespace", m.Namespace, "Name", saName)
		err = r.Get(ctx, types.NamespacedName{Name: saName, Namespace: m.Namespace}, foundSA)
		if err != nil && errors.IsNotFound(err) {
			// Define a new ServiceAccount
			// Use reflection to call the method
			//in := make([]reflect.Value, 1)
			//in[0] = reflect.ValueOf(m)
			//method := reflect.ValueOf(r).MethodByName(funcName)
			//val := method.Call(in)
			//configMPSA := val[0].Interface().(*corev1.ServiceAccount)
			//errCall := val[1].Interface()
			configMPSA, err := deployerObj.GetServiceAccountObject(m)
			if err != nil {
				log.Error(err, "Failed to get service account object")
				return err
			}
			log.Info("Creating a new ServiceAccount", "Namespace", configMPSA.Namespace, "Name", configMPSA.Name)
			err = r.Create(ctx, configMPSA)
			if err != nil {
				log.Error(err, "Failed to create new ServiceAccount", "Namespace", configMPSA.Namespace, "Name", configMPSA.Name)
				return err
			}
			// Set astraAgent instance as the owner and controller
			err = ctrl.SetControllerReference(m, configMPSA, r.Scheme)
			if err != nil {
				return err
			}
		} else if err != nil {
			log.Error(err, "Failed to get ServiceAccount")
			return err
		}
	}
	return nil
}
