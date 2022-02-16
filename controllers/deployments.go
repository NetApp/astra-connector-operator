/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package controllers

import (
	"context"
	"github.com/NetApp/astraagent-operator/common"
	"github.com/NetApp/astraagent-operator/deployer"
	ctrl "sigs.k8s.io/controller-runtime"

	cachev1 "github.com/NetApp/astraagent-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *AstraAgentReconciler) CreateDeployments(m *cachev1.AstraAgent, ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	for _, deployment := range common.DeploymentsList {
		foundDep := &appsv1.Deployment{}
		deployerObj, err := deployer.Factory(deployment)
		if err != nil {
			log.Error(err, "Failed to create deployer")
			return err
		}
		log.Info("Finding Deployment", "Namespace", m.Namespace, "Name", deployment)
		err = r.Get(ctx, types.NamespacedName{Name: deployment, Namespace: m.Namespace}, foundDep)
		if err != nil && errors.IsNotFound(err) {
			// Define a new deployment
			dep, err := deployerObj.GetDeploymentObject(m, ctx)
			if err != nil {
				log.Error(err, "Failed to get Deployment object")
				return err
			}

			log.Info("Creating a new Deployment", "Namespace", dep.Namespace, "Name", dep.Name)
			err = r.Create(ctx, dep)
			if err != nil {
				log.Error(err, "Failed to create new Deployment", "Namespace", dep.Namespace, "Name", dep.Name)
				return err
			}
			// Set astraAgent instance as the owner and controller
			err = ctrl.SetControllerReference(m, dep, r.Scheme)
			if err != nil {
				return err
			}
		} else if err != nil {
			log.Error(err, "Failed to get Deployment")
			return err
		}

		// Ensure the deployment size is the same as the spec
		size := int32(common.NatssyncClientSize)
		if foundDep.Spec.Replicas != nil && *foundDep.Spec.Replicas != size {
			foundDep.Spec.Replicas = &size
			err = r.Update(ctx, foundDep)
			if err != nil {
				log.Error(err, "Failed to update Deployment", "Namespace", foundDep.Namespace, "Name", foundDep.Name)
				return err
			}
		}
	}
	return nil
}
