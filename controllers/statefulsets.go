/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package controllers

import (
	"context"
	v1 "github.com/NetApp/astraagent-operator/api/v1"
	"github.com/NetApp/astraagent-operator/common"
	"github.com/NetApp/astraagent-operator/deployer"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *AstraAgentReconciler) CreateStatefulSets(m *v1.AstraAgent, ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	foundSet := &appsv1.StatefulSet{}

	var replicaSize int32 = common.NatsDefaultSize
	if m.Spec.Nats.Size > 2 {
		replicaSize = m.Spec.Nats.Size
	}

	log.Info("Finding StatefulSet", "Namespace", m.Namespace, "Name", common.NatsName)
	err := r.Get(ctx, types.NamespacedName{Name: common.NatsName, Namespace: m.Namespace}, foundSet)
	if err != nil && errors.IsNotFound(err) {
		// Define a new statefulset
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
		// Set astraAgent instance as the owner and controller
		err = ctrl.SetControllerReference(m, set, r.Scheme)
		if err != nil {
			return err
		}
		log.Info("Creating a new StatefulSet", "Namespace", set.Namespace, "Name", set.Name)
		err = r.Create(ctx, set)
		if err != nil {
			log.Error(err, "Failed to create new StatefulSet", "Namespace", set.Namespace, "Name", set.Name)
			return err
		}
	} else if err != nil {
		log.Error(err, "Failed to get nats StatefulSet")
		return err
	}

	// Ensure the nats statefulset size is the same as the spec
	natsSize := replicaSize
	if foundSet.Spec.Replicas != nil && *foundSet.Spec.Replicas != natsSize {
		foundSet.Spec.Replicas = &natsSize
		err = r.Update(ctx, foundSet)
		if err != nil {
			log.Error(err, "Failed to update StatefulSet", "Namespace", foundSet.Namespace, "Name", foundSet.Name)
			return err
		}
	}

	return nil
}
