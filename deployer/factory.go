/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package deployer

import (
	"fmt"

	"github.com/NetApp/astra-connector-operator/common"
	"github.com/NetApp/astra-connector-operator/echo_client"

	"github.com/NetApp/astra-connector-operator/httpproxy_client"

	"github.com/NetApp/astra-connector-operator/nats"
	"github.com/NetApp/astra-connector-operator/natssync_client"
)

// Factory returns a deployer based on the deploymentName.
// An error will be returned if the provider is unsupported.
func Factory(
	deploymentName string,
) (Deployer, error) {
	switch deploymentName {
	case common.NatsName:
		return nats.NewNatsDeployer(), nil
	case common.NatssyncClientName:
		return natssync_client.NewNatssyncClientDeployer(), nil
	case common.HttpProxyClientName:
		return httpproxy_client.NewHttpproxyClientDeployer(), nil
	case common.EchoClientName:
		return echo_client.NewEchoClientDeployer(), nil
	default:
		return nil, fmt.Errorf("unknown deployer %s", deploymentName)
	}
}
