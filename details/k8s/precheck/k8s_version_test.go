package precheck_test

import (
	"github.com/NetApp-Polaris/astra-connector-operator/mocks"
	testutil "github.com/NetApp-Polaris/astra-connector-operator/test/test-util"
	"testing"

	"github.com/NetApp-Polaris/astra-connector-operator/details/k8s/precheck"
	semver "github.com/hashicorp/go-version"
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
			k8sVersion := semver.Must(semver.NewSemver(tc.k8sVersion))
			mockK8sUtil := mocks.NewK8sUtilInterface(t)
			precheckClient := precheck.NewPrecheckClient(log, mockK8sUtil)

			mockK8sUtil.On("VersionGet").Return(k8sVersion, nil)

			precheckClient.RunK8sVersionCheck()
		})
	}
}
