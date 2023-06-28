/*
 * Copyright (c) 2023. NetApp, Inc. All Rights Reserved.
 */

package v1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

func TestAstraConnector_ValidateCreateAstraConnector(t *testing.T) {
	ai := &v1.AstraConnector{}
	err := ai.ValidateCreateAstraConnector()

	// Validate that no error occurred
	if len(err) != 0 {
		t.Errorf("Expected no errors, but got %d", len(err))
	}
}

func TestAstraConnector_ValidateUpdateAstraConnector(t *testing.T) {
	ai := &v1.AstraConnector{}
	err := ai.ValidateUpdateAstraConnector()

	// Validate that no error occurred
	if err != nil {
		t.Error("Expected no error, but got:", err)
	}
}

func TestAstraConnector_ValidateNamespace(t *testing.T) {
	ai := &v1.AstraConnector{}
	ai.ObjectMeta.Namespace = "default"

	errors := ai.ValidateCreateAstraConnector()

	// Validate that an error occurred
	if errors == nil {
		t.Error("Expected an error, but got nil")
	}

	// Validate the error message and field path
	expectedErrMsg := "default namespace not allowed"
	assert.Equal(t, 1, len(errors))
	assert.Equal(t, expectedErrMsg, errors[0].Detail)
	assert.Equal(t, "namespace", errors[0].Field)
}
