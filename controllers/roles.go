package controllers

import (
	"context"
	"github.com/NetApp/astraagent-operator/common"
	"github.com/NetApp/astraagent-operator/deployer"
	ctrl "sigs.k8s.io/controller-runtime"

	cachev1 "github.com/NetApp/astraagent-operator/api/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *AstraAgentReconciler) CreateRoles(m *cachev1.AstraAgent, ctx context.Context) error {
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
		//configMPRole, errCall := r.ConfigMapRole(m)
		configMPRole, err := deployerObj.GetRoleObject(m)
		if err != nil {
			log.Error(err, "Failed to get configmap role object")
			return err
		}
		log.Info("Creating a new Role", "Namespace", configMPRole.Namespace, "Name", configMPRole.Name)
		err = r.Create(ctx, configMPRole)
		if err != nil {
			log.Error(err, "Failed to create new Role", "Namespace", configMPRole.Namespace, "Name", configMPRole.Name)
			return err
		}
		// Set astraAgent instance as the owner and controller
		err = ctrl.SetControllerReference(m, configMPRole, r.Scheme)
		if err != nil {
			return err
		}
	} else if err != nil {
		log.Error(err, "Failed to get Role")
		return err
	}
	return nil
}
