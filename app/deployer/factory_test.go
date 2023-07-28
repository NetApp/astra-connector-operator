package deployer_test

import (
	"testing"

	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer"
	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/connector"
	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/neptune"
	"github.com/NetApp-Polaris/astra-connector-operator/common"
	"github.com/stretchr/testify/assert"
)

func TestFactory(t *testing.T) {
	testCases := []struct {
		name           string
		deploymentName string
		expectedType   interface{}
		expectError    bool
	}{
		{
			name:           "NatsName",
			deploymentName: common.NatsName,
			expectedType:   &connector.NatsDeployer{},
			expectError:    false,
		},
		{
			name:           "NatssyncClientName",
			deploymentName: common.NatsSyncClientName,
			expectedType:   &connector.NatsSyncClientDeployer{},
			expectError:    false,
		},
		{
			name:           "AstraConnectName",
			deploymentName: common.AstraConnectName,
			expectedType:   &connector.AstraConnectDeployer{},
			expectError:    false,
		},
		{
			name:           "NeptuneName",
			deploymentName: common.NeptuneName,
			expectedType:   &neptune.NeptuneClientDeployer{},
			expectError:    false,
		},
		{
			name:           "Unknown",
			deploymentName: "unknown",
			expectedType:   nil,
			expectError:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			deployer, err := deployer.Factory(tc.deploymentName)

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, deployer)
			} else {
				assert.NoError(t, err)
				assert.IsType(t, tc.expectedType, deployer)
			}
		})
	}
}
