package controllers

const (
	NatssyncClientName        = "natssync-client"
	NatssyncClientSize        = 1
	NatssyncClientPort        = 8080
	NatssyncClientProtocol    = "TCP"
	NatssyncClientKeystoreUrl = "configmap:///configmap-data"

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

	HttpProxyClientName = "httpproxy-client"
	HttpProxyClientsize = 1

	EchoClientName = "echo-client"

	NatssyncClientConfigMapName               = "natssync-client-configmap"
	NatssyncClientConfigMapRoleName           = "natssync-client-configmap-role"
	NatssyncClientConfigMapRoleBindingName    = "natssync-client-configmap-rolebinding"
	NatssyncClientConfigMapServiceAccountName = "natssync-client-configmap-serviceaccount"
	NatssyncClientConfigMapVolumeName         = "natssync-client-configmap-volume"
)
