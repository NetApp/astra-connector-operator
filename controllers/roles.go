package controllers

import (
	"context"

	cachev1 "github.com/NetApp/astraagent-operator/api/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *AstraAgentReconciler) CreateRoles(m *cachev1.AstraAgent, ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	foundRole := &rbacv1.Role{}
	log.Info("Finding ConfigMap Role", "Namespace", m.Namespace, "Name", NatssyncClientConfigMapRoleName)
	err := r.Get(ctx, types.NamespacedName{Name: NatssyncClientConfigMapRoleName, Namespace: m.Namespace}, foundRole)
	if err != nil && errors.IsNotFound(err) {
		// Define a new Role
		configMPRole, errCall := r.ConfigMapRole(m)
		if errCall != nil {
			log.Error(errCall.(error), "Failed to get configmap role object")
			return errCall.(error)
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
