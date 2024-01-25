package acp

import (
	"context"
	torcV1 "github.com/netapp/trident/operator/controllers/orchestrator/apis/netapp/v1"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func CheckForACP(ctx context.Context, client dynamic.Interface) (bool, error) {
	gvr := schema.GroupVersionResource{
		Group:    torcV1.GroupName,
		Version:  torcV1.GroupVersion,
		Resource: "tridentorchestrators",
	}

	unstructuredList, err := client.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, err
	}

	torcResources := make([]*torcV1.TridentOrchestrator, len(unstructuredList.Items))
	for i, tor := range unstructuredList.Items {
		torcResources[i] = new(torcV1.TridentOrchestrator)
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(tor.Object, torcResources[i])
		if err != nil {
			return false, err
		}
	}

	if len(torcResources) == 0 {
		// This is likely the TORC was removed from the cluster
		log.Debugf("No TORC resources could be found on cluster.")
		return false, nil
	} else if torcResources[0].Status.ACPVersion == "" {
		// ACP is either disabled, uninstalled, or failed
		log.WithField("TORC Installation Status:", torcResources[0].Status.Status).
			Infof("ACP is not detected")
		return false, nil
	}
	return true, nil
}
