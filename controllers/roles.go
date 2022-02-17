/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package controllers

import (
	"context"
	v1 "github.com/NetApp/astraagent-operator/api/v1"
	"github.com/NetApp/astraagent-operator/common"
	"github.com/NetApp/astraagent-operator/deployer"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *AstraAgentReconciler) CreateRoles(m *v1.AstraAgent, ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	deployerObj, err := deployer.Factory("natssync-client")
	if err != nil {
		log.Error(err, "Failed to create deployer")
		return err
	}
	foundRole := &rbacv1.Role{}
	log.Info("Finding ConfigMap Role", "Namespace", m.Namespace, "Name", common.NatssyncClientConfigMapRoleName)
	err = r.Get(ctx, types.NamespacedName{Name: common.NatssyncClientConfigMapRoleName, Namespace: m.Namespace}, foundRole)
	if err != nil && errors.IsNotFound(err) {
		// Define a new Role
		configMPRole, err := deployerObj.GetRoleObject(m)
		if err != nil {
			log.Error(err, "Failed to get configmap role object")
			return err
		}
		// Set astraAgent instance as the owner and controller
		err = ctrl.SetControllerReference(m, configMPRole, r.Scheme)
		if err != nil {
			return err
		}
		log.Info("Creating a new Role", "Namespace", configMPRole.Namespace, "Name", configMPRole.Name)
		err = r.Create(ctx, configMPRole)
		if err != nil {
			log.Error(err, "Failed to create new Role", "Namespace", configMPRole.Namespace, "Name", configMPRole.Name)
			return err
		}
	} else if err != nil {
		log.Error(err, "Failed to get Role")
		return err
	}
	return nil
}
