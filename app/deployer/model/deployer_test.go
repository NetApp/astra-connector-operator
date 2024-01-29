package model_test

import (
	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/model"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMutateFunc(t *testing.T) {
	mutFunc := model.NonMutateFn()
	assert.Nil(t, mutFunc)
}
