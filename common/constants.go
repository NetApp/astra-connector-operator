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

	AstraConnectName                 = "astraconnect"
	AstraConnectorOperatorRepository = "netapp/astra-connector-operator"
	AstraConnectTagFile              = "common/connector_version.txt"

	NatsSyncClientName                  = "natssync-client"
	NatsSyncClientPort                  = 8080
	NatsSyncClientProtocol              = "TCP"
	NatsSyncClientKeystoreUrl           = "configmap:///configmap-data"
	NatsSyncClientDefaultImage          = "natssync-client:2.2.202402012115"
	NatsSyncClientDefaultCloudBridgeURL = "https://astra.netapp.io"

	NatsName               = "nats"
	NatsClusterServiceName = "nats-cluster"
	NatsConfigMapName      = "nats-configmap"
	NatsServiceAccountName = "nats-serviceaccount"
	NatsRoleName           = "nats-role"
	NatsRoleBindingName    = "nats-rolebinding"
	NatsVolumeName         = "nats-configmap-volume"
	NatsClientPort         = 4222
	NatsClusterPort        = 6222
	NatsMonitorPort        = 8222
	NatsMetricsPort        = 7777
	NatsGatewaysPort       = 7522
	NatsDefaultReplicas    = 1
	// NatsDefaultImage when changing default image push image to jfrog as well
	NatsDefaultImage = "nats:2.10.1-alpine3.18"
	NatsMaxPayload   = 8388608

	NatsSyncClientConfigMapName               = "natssync-client-configmap"
	NatsSyncClientConfigMapRoleName           = "natssync-client-configmap-role"
	NatsSyncClientConfigMapRoleBindingName    = "natssync-client-configmap-rolebinding"
	NatsSyncClientConfigMapServiceAccountName = "natssync-client-configmap-serviceaccount"
	NatsSyncClientConfigMapVolumeName         = "natssync-client-configmap-volume"

	NeptuneName                  = "neptune-controller-manager"
	NeptuneDefaultTag            = "e056f69"
	NeptuneTagFile               = "common/neptune_manager_tag.txt"
	NeptuneMetricServicePort     = 8443
	NeptuneMetricServiceProtocol = "TCP"
	NeptuneReplicas   = 1

	AstraPrivateCloudType = "private"
	AstraPrivateCloudName = "private"

	ConnectorNeptuneCapability = "neptuneV1"
	ConnectorV2Capability      = "connectorV2" // V2 refers specifically to Arch 3.0 connector and beyond
	ConnectorWatcherCapability = "watcherV1"

	AstraClustersAPIVersion        = "1.4"
	AstraManagedClustersAPIVersion = "1.2"
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
	return []string{"controller", "resourcesummaryupload", "resourcerestore", "resourcedelete", "resourcebackup", "exechook"}
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
