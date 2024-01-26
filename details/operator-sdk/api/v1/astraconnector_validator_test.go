/*
 * Copyright (c) 2023. NetApp, Inc. All Rights Reserved.
 */

package v1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

// makeAstraConnector is a test helper that creates an Astra Connector with default values.
func makeAstraConnector(t *testing.T) *v1.AstraConnector {
	t.Helper()
	return &v1.AstraConnector{
		ObjectMeta: v12.ObjectMeta{
			Name:            "astra-connector",
			Namespace:       "astra-connector",
			ResourceVersion: "v1",
		},
		Spec: v1.AstraConnectorSpec{
			Astra: v1.Astra{
				ClusterName: "managed-cluster",
			},
		},
	}
}

func TestAstraConnector_ValidateCreateAstraConnector(t *testing.T) {
	// Validate that no error occurred.
	ai := makeAstraConnector(t)
	err := ai.ValidateCreateAstraConnector()
	assert.Emptyf(t, err, "expected empty error list, but got %v", err)

	// Test with an invalid cluster name and ensure an error occurred.
	ai.Spec.Astra.ClusterName = "INVALID-CLUSTER-NAME"
	err = ai.ValidateCreateAstraConnector()
	assert.NotEmptyf(t, err, "expected non-empty error list, but got %v", err)
}

func TestAstraConnector_ValidateUpdateAstraConnector(t *testing.T) {
	ai := makeAstraConnector(t)
	err := ai.ValidateUpdateAstraConnector()

	// Validate that no error occurred
	if err != nil {
		t.Error("Expected no error, but got:", err)
	}
}

func TestAstraConnector_ValidateNamespace(t *testing.T) {
	ai := makeAstraConnector(t)
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
