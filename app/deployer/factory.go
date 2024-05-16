/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package deployer

import (
	"fmt"

	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/connector"
	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/model"
	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/neptune"
	"github.com/NetApp-Polaris/astra-connector-operator/common"
)

// Factory returns a deployer based on the deploymentName.
// An error will be returned if the provider is unsupported.
func Factory(
	deploymentName string,
) (model.Deployer, error) {
	switch deploymentName {
	case common.AstraConnectName:
		return connector.NewAstraConnectorDeployer(), nil
	case common.NeptuneName:
		return neptune.NewNeptuneClientDeployerV2(), nil

	default:
		return nil, fmt.Errorf("unknown deployer %s", deploymentName)
	}
}
