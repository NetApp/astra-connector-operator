/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package deployer

import (
	"fmt"
	"github.com/NetApp-Polaris/astra-connector-operator/deployer/neptune"

	"github.com/NetApp-Polaris/astra-connector-operator/common"
	"github.com/NetApp-Polaris/astra-connector-operator/deployer/connector"
	"github.com/NetApp-Polaris/astra-connector-operator/deployer/model"
)

// Factory returns a deployer based on the deploymentName.
// An error will be returned if the provider is unsupported.
func Factory(
	deploymentName string,
) (model.Deployer, error) {
	switch deploymentName {
	case common.NatsName:
		return connector.NewNatsDeployer(), nil
	case common.NatssyncClientName:
		return connector.NewNatsSyncClientDeployer(), nil
	case common.AstraConnectName:
		return connector.NewAstraConnectorDeployer(), nil
	case common.NeptuneName:
		return neptune.NewNeptuneClientDeployer(), nil

	default:
		return nil, fmt.Errorf("unknown deployer %s", deploymentName)
	}
}
