package conf_test

import (
	"testing"

	"github.com/NetApp-Polaris/astra-connector-operator/app/conf"
	"github.com/stretchr/testify/assert"
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
