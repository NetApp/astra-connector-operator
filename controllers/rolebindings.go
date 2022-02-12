package controllers

import (
	"context"

	cachev1 "github.com/NetApp/astraagent-operator/api/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *AstraAgentReconciler) CreateRoleBindings(m *cachev1.AstraAgent, ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	foundRoleB := &rbacv1.RoleBinding{}
	log.Info("Finding ConfigMap RoleBinding", "Namespace", m.Namespace, "Name", NatssyncClientConfigMapRoleBindingName)
	err := r.Get(ctx, types.NamespacedName{Name: NatssyncClientConfigMapRoleBindingName, Namespace: m.Namespace}, foundRoleB)
	if err != nil && errors.IsNotFound(err) {
		// Define a new RoleBinding
		roleB, errCall := r.ConfigMapRoleBinding(m)
		if errCall != nil {
			log.Error(errCall.(error), "Failed to get rolebinding object")
			return errCall.(error)
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
