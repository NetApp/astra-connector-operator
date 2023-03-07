/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package controllers

import (
	"context"
	"fmt"
	"reflect"

	ctrl "sigs.k8s.io/controller-runtime"

	v1 "github.com/NetApp/astra-connector-operator/api/v1"
	"github.com/NetApp/astra-connector-operator/common"
	"github.com/NetApp/astra-connector-operator/deployer"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *AstraConnectorReconciler) CreateDeployments(m *v1.AstraConnector, natssyncClientStatus v1.NatssyncClientStatus, ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	for _, deployment := range common.DeploymentsList {
		foundDep := &appsv1.Deployment{}
		deployerObj, err := deployer.Factory(deployment)
		if err != nil {
			log.Error(err, "Failed to create deployer")
			return err
		}
		// Define a new deployment
		dep, err := deployerObj.GetDeploymentObject(m, ctx)
		if err != nil {
			log.Error(err, "Failed to get Deployment object")
			return err
		}
		log.Info("Finding Deployment", "Namespace", m.Namespace, "Name", deployment)
		err = r.Get(ctx, types.NamespacedName{Name: deployment, Namespace: m.Namespace}, foundDep)
		if err != nil && errors.IsNotFound(err) {
			// Set AstraConnector instance as the owner and controller
			err = ctrl.SetControllerReference(m, dep, r.Scheme)
			if err != nil {
				return err
			}
			statusMsg := fmt.Sprintf(CreateDeployment, dep.Namespace, dep.Name)
			log.Info(statusMsg)
			natssyncClientStatus.Status = statusMsg
			r.updateAstraConnectorStatus(ctx, m, natssyncClientStatus)
			err = r.Create(ctx, dep)
			if err != nil {
				log.Error(err, "Failed to create new Deployment", "Namespace", dep.Namespace, "Name", dep.Name)
				return err
			}
		} else if err != nil {
			log.Error(err, "Failed to get Deployment")
			return err
		}

		// Ensure the deployment is the same as the spec
		if &foundDep.Spec != nil && !reflect.DeepEqual(foundDep.Spec, dep.Spec) {
			foundDep.Spec = dep.Spec
			statusMsg := fmt.Sprintf(UpdateDeployment, foundDep.Namespace, foundDep.Name)
			log.Info(statusMsg)
			natssyncClientStatus.Status = statusMsg
			r.updateAstraConnectorStatus(ctx, m, natssyncClientStatus)
			err = r.Update(ctx, foundDep)
			if err != nil {
				log.Error(err, "Failed to update Deployment", "Namespace", foundDep.Namespace, "Name", foundDep.Name)
				return err
			}
		}
	}
	return nil
}
