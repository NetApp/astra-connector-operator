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

func (r *AstraAgentReconciler) CreateServices(m *cachev1.AstraAgent, ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	services := map[string]string{
		NatssyncClientName:     "ServiceForNatssyncClient",
		NatsName:               "ServiceForNats",
		NatsClusterServiceName: "ClusterServiceForNats",
	}

	for service, funcName := range services {
		foundSer := &corev1.Service{}
		log.Info("Finding Service", "Namespace", m.Namespace, "Name", service)
		err := r.Get(ctx, types.NamespacedName{Name: service, Namespace: m.Namespace}, foundSer)
		if err != nil && errors.IsNotFound(err) {
			// Define a new service
			// Use reflection to call the method
			in := make([]reflect.Value, 1)
			in[0] = reflect.ValueOf(m)
			method := reflect.ValueOf(r).MethodByName(funcName)
			val := method.Call(in)
			serv := val[0].Interface().(*corev1.Service)
			errCall := val[1].Interface()
			if errCall != nil {
				log.Error(errCall.(error), "Failed to get service object")
				return errCall.(error)
			}
			log.Info("Creating a new Service", "Namespace", serv.Namespace, "Name", serv.Name)
			err = r.Create(ctx, serv)
			if err != nil {
				log.Error(err, "Failed to create new Service", "Namespace", serv.Namespace, "Name", serv.Name)
				return err
			}
		} else if err != nil {
			log.Error(err, "Failed to get Service")
			return err
		}
	}
	return nil
}
