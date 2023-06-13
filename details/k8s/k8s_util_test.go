/*
 * Copyright (c) 2023. NetApp, Inc. All Rights Reserved.
 */

package k8s_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NetApp-Polaris/astra-connector-operator/details/k8s"
	testutil "github.com/NetApp-Polaris/astra-connector-operator/test/test-util"
)

func createResourceHandlerWithFakeClient(t *testing.T, initObjs ...client.Object) (k8s.K8sUtilInterface, client.Client) {
	fakeClient := testutil.CreateFakeClient(initObjs...)
	k8sUtil := k8s.NewK8sUtil(fakeClient)
	return k8sUtil, fakeClient
}

func TestNewResourceHandler(t *testing.T) {
	handler, _ := createResourceHandlerWithFakeClient(t)

	assert.NotNil(t, handler)
}
