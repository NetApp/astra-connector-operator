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

func (r *AstraAgentReconciler) CreateRoleBindings(m *v1.AstraAgent, ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	deployerObj, err := deployer.Factory("natssync-client")
	if err != nil {
		log.Error(err, "Failed to create deployer")
		return err
	}
	foundRoleB := &rbacv1.RoleBinding{}
	log.Info("Finding ConfigMap RoleBinding", "Namespace", m.Namespace, "Name", common.NatssyncClientConfigMapRoleBindingName)
	err = r.Get(ctx, types.NamespacedName{Name: common.NatssyncClientConfigMapRoleBindingName, Namespace: m.Namespace}, foundRoleB)
	if err != nil && errors.IsNotFound(err) {
		// Define a new RoleBinding
		roleB, err := deployerObj.GetRoleBindingObject(m)
		if err != nil {
			log.Error(err, "Failed to get rolebinding object")
			return err
		}
		// Set astraAgent instance as the owner and controller
		err = ctrl.SetControllerReference(m, roleB, r.Scheme)
		if err != nil {
			return err
		}
		log.Info("Creating a new RoleBinding", "Namespace", roleB.Namespace, "Name", roleB.Name)
		err = r.Create(ctx, roleB)
		if err != nil {
			log.Error(err, "Failed to create new RoleBinding", "Namespace", roleB.Namespace, "Name", roleB.Name)
			return err
		}
	} else if err != nil {
		log.Error(err, "Failed to get RoleBinding")
		return err
	}
	return nil
}
