package conf_test

import (
	"github.com/NetApp-Polaris/astra-connector-operator/app/conf"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetPodSecurityContext(t *testing.T) {
	sc := conf.GetPodSecurityContext()
	assert.NotNil(t, sc)
	assert.True(t, *sc.RunAsNonRoot)
}

func TestGetSecurityContext(t *testing.T) {
	sc := conf.GetSecurityContext()
	assert.NotNil(t, sc)
	assert.True(t, *sc.RunAsNonRoot)
	assert.NotEqual(t, int64(0), *sc.RunAsUser)
	assert.True(t, *sc.ReadOnlyRootFilesystem)
}

func TestImmutableConfiguration(t *testing.T) {
	// Initialize a test configuration
	config := conf.ImmutableConfiguration{}

	// Test each method
	if config.Host() != "" {
		t.Errorf("Expected empty, got %s", config.Host())
	}

	if config.AppRoot() != "" {
		t.Errorf("Expected empty, got %s", config.AppRoot())
	}

	// TODO ADD test
}

func TestImmutableFeatureFlags(t *testing.T) {
	// Initialize a test feature flag configuration
	flags := conf.ImmutableFeatureFlags{}

	if flags.DeployNatsConnector() != false {
		t.Errorf("Expected true, got %v", flags.DeployNatsConnector())
	}

	if flags.DeployNeptune() != false {
		t.Errorf("Expected false, got %v", flags.DeployNeptune())
	}

	// TODO add test
}
