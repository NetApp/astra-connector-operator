package controllers

import (
	"context"
	"reflect"

	cachev1 "github.com/NetApp/astraagent-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *AstraAgentReconciler) CreateServiceAccounts(m *cachev1.AstraAgent, ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	serviceaccounts := map[string]string{
		NatssyncClientConfigMapServiceAccountName: "ServiceAccountForNatssyncClientConfigMap",
		NatsServiceAccountName:                    "ServiceAccountForNats",
	}

	for sas, funcName := range serviceaccounts {
		foundSA := &corev1.ServiceAccount{}
		log.Info("Finding ServiceAccount", "Namespace", m.Namespace, "Name", sas)
		err := r.Get(ctx, types.NamespacedName{Name: sas, Namespace: m.Namespace}, foundSA)
		if err != nil && errors.IsNotFound(err) {
			// Define a new ServiceAccount
			// Use reflection to call the method
			in := make([]reflect.Value, 1)
			in[0] = reflect.ValueOf(m)
			method := reflect.ValueOf(r).MethodByName(funcName)
			val := method.Call(in)
			configMPSA := val[0].Interface().(*corev1.ServiceAccount)
			errCall := val[1].Interface()
			if errCall != nil {
				log.Error(errCall.(error), "Failed to get service account object")
				return errCall.(error)
			}
			log.Info("Creating a new ServiceAccount", "Namespace", configMPSA.Namespace, "Name", configMPSA.Name)
			err = r.Create(ctx, configMPSA)
			if err != nil {
				log.Error(err, "Failed to create new ServiceAccount", "Namespace", configMPSA.Namespace, "Name", configMPSA.Name)
				return err
			}
		} else if err != nil {
			log.Error(err, "Failed to get ServiceAccount")
			return err
		}
	}
	return nil
}
