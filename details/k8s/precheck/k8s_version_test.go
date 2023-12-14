package precheck_test

import (
	"github.com/NetApp-Polaris/astra-connector-operator/mocks"
	testutil "github.com/NetApp-Polaris/astra-connector-operator/test/test-util"
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/NetApp-Polaris/astra-connector-operator/details/k8s/precheck"
)

func TestIsSupported(t *testing.T) {
	testCases := []struct {
		name          string
		k8sVersion    string
		expectedValid bool
	}{
		{
			name:          "Minimum supported version",
			k8sVersion:    "1.24.0",
			expectedValid: true,
		},
		{
			name:          "Maximum supported version",
			k8sVersion:    "1.27.0",
			expectedValid: false,
		},
		{
			name:          "Within supported range",
			k8sVersion:    "1.25.0",
			expectedValid: true,
		},
		{
			name:          "Below supported range",
			k8sVersion:    "1.23.0",
			expectedValid: false,
		},
		{
			name:          "Above supported range",
			k8sVersion:    "1.27.1",
			expectedValid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			log := testutil.CreateLoggerForTesting(t)
			mockK8sUtil := mocks.NewK8sUtilInterface(t)
			precheckClient := precheck.NewPrecheckClient(log, mockK8sUtil)

			mockK8sUtil.On("VersionGet").Return(tc.k8sVersion, nil)

			err := precheckClient.RunK8sVersionCheck()
			if tc.expectedValid {
				assert.Nil(t, err, "We expected no error ")
			} else {
				assert.NotNil(t, err)
			}

		})
	}
}
