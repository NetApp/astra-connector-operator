/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package controllers

import (
	"context"
	"fmt"
	"reflect"

	v1 "github.com/NetApp/astra-connector-operator/api/v1"
	"github.com/NetApp/astra-connector-operator/common"
	"github.com/NetApp/astra-connector-operator/deployer"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *AstraConnectorReconciler) CreateStatefulSets(m *v1.AstraConnector, natssyncClientStatus v1.NatssyncClientStatus, ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	foundSet := &appsv1.StatefulSet{}
	deployerObj, err := deployer.Factory("nats")
	if err != nil {
		log.Error(err, "Failed to create deployer")
		return err
	}
	set, err := deployerObj.GetStatefulsetObject(m, ctx)
	if err != nil {
		log.Error(err, "Failed to get statefulset object")
		return err
	}

	log.Info("Finding StatefulSet", "Namespace", m.Namespace, "Name", common.NatsName)
	err = r.Get(ctx, types.NamespacedName{Name: common.NatsName, Namespace: m.Namespace}, foundSet)
	if err != nil && errors.IsNotFound(err) {
		// Define a new statefulset

		// Set astraConnector instance as the owner and controller
		err = ctrl.SetControllerReference(m, set, r.Scheme)
		if err != nil {
			return err
		}
		statusMsg := fmt.Sprintf(CreateStatefulSet, set.Namespace, set.Name)
		log.Info(statusMsg)
		natssyncClientStatus.Status = statusMsg
		r.updateAstraConnectorStatus(ctx, m, natssyncClientStatus)
		err = r.Create(ctx, set)
		if err != nil {
			log.Error(err, "Failed to create new StatefulSet", "Namespace", set.Namespace, "Name", set.Name)
			return err
		}
	} else if err != nil {
		log.Error(err, "Failed to get nats StatefulSet")
		return err
	}

	// Ensure the nats statefulset is the same as the spec
	if &foundSet.Spec != nil && !reflect.DeepEqual(foundSet.Spec, set.Spec) {
		foundSet.Spec = set.Spec
		log.Info("Updating the StatefulSet", "Namespace", foundSet.Namespace, "Name", foundSet.Name)
		err = r.Update(ctx, foundSet)
		if err != nil {
			log.Error(err, "Failed to update StatefulSet", "Namespace", foundSet.Namespace, "Name", foundSet.Name)
			return err
		}
	}

	return nil
}
