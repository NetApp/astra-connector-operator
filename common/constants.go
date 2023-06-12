/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package common

const (
	DefaultImageRegistry = "theotw"

	AstraConnectName         = "astraconnect"
	AstraConnectSize         = 1
	AstraConnectDefaultImage = "astra-connector:0.3"

	NatsSyncClientName                  = "natssync-client"
	NatsSyncClientSize                  = 1
	NatsSyncClientPort                  = 8080
	NatsSyncClientProtocol              = "TCP"
	NatsSyncClientKeystoreUrl           = "configmap:///configmap-data"
	NatsSyncClientDefaultImage          = "natssync-client:0.9.202202170408"
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
	NatsDefaultSize        = 2
	NatsDefaultImage       = "nats:2.6.1-alpine3.14"

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
