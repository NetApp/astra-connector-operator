package precheck

import (
	"github.com/pkg/errors"
)

func (p *PrecheckClient) RunK8sCRDCheck() error {
	if !p.k8sUtil.IsCRDInstalled("volumesnapshotclasses.snapshot.storage.k8s.io") {
		return errors.New("Could not find volumesnapshotclasses CRD")
	}
	return nil
}
