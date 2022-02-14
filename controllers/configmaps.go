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

func (r *AstraAgentReconciler) CreateConfigMaps(m *cachev1.AstraAgent, ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	for cmName, deploymentName := range common.ConfigMapsList {
		deployerObj, err := deployer.Factory(deploymentName)
		if err != nil {
			log.Error(err, "Failed to create deployer")
			return err
		}
		foundCM := &corev1.ConfigMap{}
		log.Info("Finding ConfigMap", "Namespace", m.Namespace, "Name", cmName)
		err = r.Get(ctx, types.NamespacedName{Name: cmName, Namespace: m.Namespace}, foundCM)
		if err != nil && errors.IsNotFound(err) {
			// Define a new configmap
			// Use reflection to call the method
			//in := make([]reflect.Value, 1)
			//in[0] = reflect.ValueOf(m)
			//method := reflect.ValueOf(r).MethodByName(funcName)
			//val := method.Call(in)
			//configMP := val[0].Interface().(*corev1.ConfigMap)
			//errCall := val[1].Interface()
			configMP, err := deployerObj.GetConfigMapObject(m)
			if err != nil {
				log.Error(err, "Failed to get configmap object")
				return err
			}
			log.Info("Creating a new ConfigMap", "Namespace", configMP.Namespace, "Name", configMP.Name)
			err = r.Create(ctx, configMP)
			if err != nil {
				log.Error(err, "Failed to create new ConfigMap", "Namespace", configMP.Namespace, "Name", configMP.Name)
				return err
			}
			err = ctrl.SetControllerReference(m, configMP, r.Scheme)
			if err != nil {
				return err
			}
		} else if err != nil {
			log.Error(err, "Failed to get ConfigMap")
			return err
		}
	}
	return nil
}
