package controllers

import (
	"context"

	cachev1 "github.com/NetApp/astraagent-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *AstraAgentReconciler) CreateStatefulSets(m *cachev1.AstraAgent, ctx context.Context) error {
	log := ctrllog.FromContext(ctx)
	foundSet := &appsv1.StatefulSet{}

	var replicaSize int32 = NatsDefaultSize
	if m.Spec.Nats.Size > 2 {
		replicaSize = m.Spec.Nats.Size
	}

	log.Info("Finding StatefulSet", "Namespace", m.Namespace, "Name", NatsName)
	err := r.Get(ctx, types.NamespacedName{Name: NatsName, Namespace: m.Namespace}, foundSet)
	if err != nil && errors.IsNotFound(err) {
		// Define a new statefulset
		set, errCall := r.StatefulsetForNats(m, ctx)
		if errCall != nil {
			log.Error(errCall.(error), "Failed to get statefulset object")
			return errCall.(error)
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
