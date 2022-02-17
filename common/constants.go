/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package common

const (
	DefaultImageRegistry = "theotw"

	NatssyncClientName                  = "natssync-client"
	NatssyncClientSize                  = 1
	NatssyncClientPort                  = 8080
	NatssyncClientProtocol              = "TCP"
	NatssyncClientKeystoreUrl           = "configmap:///configmap-data"
	NatssyncClientDefaultImage          = "natssync-client:0.9.202202170408"
	NatssyncClientDefaultCloudBridgeURL = "https://integration.astra.netapp.io"

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

	HttpProxyClientName         = "httpproxy-client"
	HttpProxyClientsize         = 1
	HttpProxyClientDefaultImage = "httpproxylet:0.9.202202170408"

	EchoClientName         = "echo-client"
	EchoClientDefaultSize  = 1
	EchoClientDefaultImage = "echo-proxylet:0.9.202202170408"

	NatssyncClientConfigMapName               = "natssync-client-configmap"
	NatssyncClientConfigMapRoleName           = "natssync-client-configmap-role"
	NatssyncClientConfigMapRoleBindingName    = "natssync-client-configmap-rolebinding"
	NatssyncClientConfigMapServiceAccountName = "natssync-client-configmap-serviceaccount"
	NatssyncClientConfigMapVolumeName         = "natssync-client-configmap-volume"

	AstraDefaultCloudType = "Azure"
)

// ServicesList - serviceName: deploymentName
var ServicesList = map[string]string{
	NatsName:               NatsName,
	NatsClusterServiceName: NatsName,
	NatssyncClientName:     NatssyncClientName,
}

// ConfigMapsList - configMapName: deploymentName
var ConfigMapsList = map[string]string{
	NatsConfigMapName:           NatsName,
	NatssyncClientConfigMapName: NatssyncClientName,
}

// ServiceAccountsList - serviceAccountName: deploymentName
var ServiceAccountsList = map[string]string{
	NatssyncClientConfigMapServiceAccountName: NatssyncClientName,
	NatsServiceAccountName:                    NatsName,
}

// DeploymentsList - deploymentNames
var DeploymentsList = []string{NatssyncClientName, HttpProxyClientName, EchoClientName}
