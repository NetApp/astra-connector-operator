/*
 * Copyright (c) 2023. NetApp, Inc. All Rights Reserved.
 */

package v1_test

import (
	"github.com/NetApp-Polaris/astra-connector-operator/mocks"
	"github.com/stretchr/testify/mock"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

func TestAstraConnector_ValidateCreateAstraConnector(t *testing.T) {
	ai := &v1.AstraConnector{Spec: v1.AstraConnectorSpec{Astra: v1.Astra{AccountId: "6587afff-7515-4c35-8e53-95545e427e31"}}}
	err := ai.ValidateNamespace()

	// Validate that no error occurred
	if err != nil {
		t.Error("Expected no error, but got:", err)
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

	err := ai.ValidateNamespace()

	// Validate that an error occurred
	if err == nil {
		t.Error("Expected an error, but got nil")
	}

	// Validate the error message and field path
	expectedErrMsg := "default namespace not allowed"
	assert.Error(t, err, expectedErrMsg)
}

func TestAstraConnector_ValidateInputs(t *testing.T) {
	ai := &v1.AstraConnector{Spec: v1.AstraConnectorSpec{Astra: v1.Astra{AccountId: "6587afff-7515-4c35-8e53-95545e427e31"}}}
	mockHttpClient := &mocks.HTTPClient{}
	mockHttpClient.On("Do", mock.Anything).Return(&http.Response{StatusCode: 200}, nil).Once()

	err := ai.ValidateTokenAndAccountID(mockHttpClient)

	// Validate that no error occurred
	if err != nil {
		t.Error("Expected no error, but got:", err)
	}
}
