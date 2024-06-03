/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package common

import (
	_ "embed"
	"strings"

	"github.com/NetApp-Polaris/astra-connector-operator/app/conf"
)

const (
	DefaultImageRegistry = "netappdownloads.jfrog.io/docker-astra-control-staging/arch30/neptune"
	AstraImageRegistry   = "cr.astra.netapp.io"

	AstraConnectName                 = "astraconnect"
	AstraConnectorOperatorRepository = "netapp/astra-connector-operator"

	NatsSyncClientDefaultImage          = "natssync-client:2.2.202402012115"
	NatsSyncClientDefaultCloudBridgeURL = "https://astra.netapp.io"

	NeptuneName = "neptune-controller-manager"

	NeptuneMetricServicePort     = 8443
	NeptuneMetricServiceProtocol = "TCP"
	NeptuneReplicas              = 1

	ConnectorNeptuneCapability = "neptuneV1"
	ConnectorV2Capability      = "connectorV2" // V2 refers specifically to Arch 3.0 connector and beyond
	ConnectorWatcherCapability = "watcherV1"

	RbacProxyImage = "kube-rbac-proxy:v0.14.1"
)

// Embed image tags

//go:embed "neptune_manager_tag.txt"
var embeddedNeptuneImageTag string

//go:embed "connector_version.txt"
var embeddedConnectorImageTag string

//go:embed "neptune_asup_tag.txt"
var embeddedAsupImageTag string

var (
	// NeptuneImageTag is the trimmed version of the embedded string.
	NeptuneImageTag   = strings.TrimSpace(embeddedNeptuneImageTag)
	ConnectorImageTag = strings.TrimSpace(embeddedConnectorImageTag)
	AsupImageTag      = strings.TrimSpace(embeddedAsupImageTag)
)

func GetNeptuneRepositories() []string {
	return []string{"controller", "exechook", "resourcebackup", "resourcedelete", "resourcerestore", "resourcesummaryupload", "restic"}
}

func GetConnectorCapabilities() []string {
	capabilities := []string{
		ConnectorV2Capability,
		ConnectorWatcherCapability,
	}

	if conf.Config.FeatureFlags().DeployNeptune() {
		capabilities = append(capabilities, ConnectorNeptuneCapability)
	}
	return capabilities
}
