/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package common

import "github.com/NetApp-Polaris/astra-connector-operator/app/conf"

const (
	DefaultImageRegistry = "netappdownloads.jfrog.io/docker-astra-control-staging/arch30/neptune"

	AstraConnectName            = "astraconnect"
	AstraConnectDefaultReplicas = 1
	AstraConnectDefaultImage    = "astra-connector:1.0.202310202233"

	NatsSyncClientName                  = "natssync-client"
	NatsSyncClientDefaultReplicas       = 1
	NatsSyncClientPort                  = 8080
	NatsSyncClientProtocol              = "TCP"
	NatsSyncClientKeystoreUrl           = "configmap:///configmap-data"
	NatsSyncClientDefaultImage          = "natssync-client:2.1.202309262120"
	NatsSyncClientDefaultCloudBridgeURL = "https://astra.netapp.io"

	NatsName               = "nats"
	NatsClusterServiceName = "nats-cluster"
	NatsConfigMapName      = "nats-configmap"
	NatsServiceAccountName = "nats-serviceaccount"
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

	NeptuneName                          = "neptune-controller-manager"
	NeptuneLeaderElectionRoleName        = "neptune-leader-election-role"
	NeptuneLeaderElectionRoleBindingName = "neptune-leader-election-rolebinding"
	NeptuneClusterRoleName               = "neptune-manager-role"
	NeptuneMetricServicePort             = 8443
	NeptuneMetricServiceProtocol         = "TCP"
	NeptuneDefaultImage                  = "controller:e056f69"

	AstraPrivateCloudType = "private"
	AstraPrivateCloudName = "private"

	ConnectorNeptuneCapability = "neptuneV1"
	ConnectorRelayCapability   = "relayV1"
	ConnectorWatcherCapability = "watcherV1"

	AstraClustersAPIVersion        = "1.4"
	AstraManagedClustersAPIVersion = "1.2"
)

func GetConnectorCapabilities() []string {
	capabilities := []string{
		ConnectorRelayCapability,
		ConnectorWatcherCapability,
	}

	if conf.Config.FeatureFlags().DeployNeptune() {
		capabilities = append(capabilities, ConnectorNeptuneCapability)
	}
	return capabilities
}
