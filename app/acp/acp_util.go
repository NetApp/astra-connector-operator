package acp

import (
	"context"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"time"

	torcV1 "github.com/netapp/trident/operator/controllers/orchestrator/apis/netapp/v1"
)

func CheckForACP(ctx context.Context, client rest.Interface) (bool, error) {
	torcResources := &torcV1.TridentOrchestratorList{}
	err := client.Get().
		Resource("tridentorchestrators").
		Timeout(time.Second * 15).
		Do(ctx).
		Into(torcResources)
	if err != nil {
		return false, err
	}

	if len(torcResources.Items) == 0 {
		// This is likely the TORC was removed from the cluster
		log.Debugf("No TORC resources could be found on cluster.")
		return false, nil
	} else if torcResources.Items[0].Status.ACPVersion == "" {
		// ACP is either disabled, uninstalled, or failed
		log.WithField("TORC Installation Status:", torcResources.Items[0].Status.Status).
			Infof("Cluster Protection State is partial since ACP is not detected")
		return false, nil
	}
	return true, nil
}
