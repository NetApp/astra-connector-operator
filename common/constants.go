/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package common

const (
	DefaultImageRegistry = "docker.io"

	AstraConnectName            = "astraconnect"
	AstraConnectDefaultReplicas = 1
	AstraConnectDefaultImage    = "netapp/astra-connector:1.0.202306162024"

	NatsSyncClientName                  = "natssync-client"
	NatsSyncClientDefaultReplicas       = 1
	NatsSyncClientPort                  = 8080
	NatsSyncClientProtocol              = "TCP"
	NatsSyncClientKeystoreUrl           = "configmap:///configmap-data"
	NatsSyncClientDefaultImage          = "theotw/natssync-client:2.1.202305182124"
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
	NatsDefaultReplicas    = 2
	NatsDefaultImage       = "nats:2.8.4-alpine3.15"

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

	AstraDefaultCloudType = "Azure"
	AstraPrivateCloudType = "private"
	AstraPrivateCloudName = "private"

	ConnectorRelayCapability   = "relayV1"
	ConnectorWatcherCapability = "watcherV1"

	AstraClustersAPIVersion        = "1.4"
	AstraManagedClustersAPIVersion = "1.2"
)

func GetConnectorCapabilities() []string {
	return []string{
		ConnectorRelayCapability,
		ConnectorWatcherCapability,
	}
}
