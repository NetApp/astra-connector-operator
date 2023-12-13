// Copyright 2023 NetApp, Inc. All Rights Reserved.

package precheck

import (
	"errors"
	"fmt"

	semver "github.com/hashicorp/go-version"
)

const (
	MinKubernetesVersion = "1.24.0"
	MaxKubernetesVersion = "1.27.0"
)

func (p *PrecheckClient) RunK8sVersionCheck() error {
	versionString, err := p.k8sUtil.VersionGet()
	if err != nil {
		p.log.Error(err, "failed to get k8s version of host cluster")
		return err
	}

	k8sVersion, err := semver.NewSemver(versionString)
	if err != nil {
		p.log.Error(err, "failed to parse k8s version string", "version string", versionString)
		return err
	}

	if warning := isSupported(*k8sVersion); warning == nil {
		p.log.Info("detected valid k8s version")
		return nil
	} else {
		return errors.New(*warning)
	}
}

func isSupported(k8sVersion semver.Version) *string {
	minVersion := semver.Must(semver.NewSemver(MinKubernetesVersion))
	maxVersion := semver.Must(semver.NewSemver(MaxKubernetesVersion))

	if k8sVersion.GreaterThanOrEqual(minVersion) && k8sVersion.LessThan(maxVersion) {
		return nil
	}

	message := fmt.Sprintf(
		"Cluster isn't running a supported version of kubernetes. "+
			"Use a supported kubernetes version in the following range: %v to %v.",
		MinKubernetesVersion,
		MaxKubernetesVersion,
	)

	return &message
}
