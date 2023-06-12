/*
 * Copyright (c) 2023. NetApp, Inc. All Rights Reserved.
 */

package k8s_test

import (
	"context"
	testutil "github.com/NetApp-Polaris/astra-installer/test/test-util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/NetApp-Polaris/astra-installer/details/k8s"
	"github.com/NetApp-Polaris/astra-installer/details/operator-sdk/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ctx = context.TODO()

func createResourceHandlerWithFakeClient(t *testing.T, initObjs ...client.Object) (k8s.ResourceHandlerInterface, client.Client) {
	fakeClient := testutil.CreateFakeClient(initObjs...)
	log := testutil.CreateLoggerForTesting(t)
	resourceHandler := k8s.NewResourceHandler(ctx, fakeClient, log)
	return resourceHandler, fakeClient
}

func TestNewResourceHandler(t *testing.T) {
	handler, _ := createResourceHandlerWithFakeClient(t)

	assert.NotNil(t, handler)
}

func TestInstallerGet(t *testing.T) {
	// Create a new AstraInstaller object
	astraInstaller := &v1.AstraInstaller{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-astra",
			Namespace: "test-namespace",
		},
	}

	// Add the AstraInstaller object to the fake client
	handler, _ := createResourceHandlerWithFakeClient(t, astraInstaller)

	// Test the InstallerGet method
	result, err := handler.InstallerGet("test-namespace", "test-astra")
	assert.NoError(t, err)
	assert.Equal(t, "test-astra", result.Name)
	assert.Equal(t, "test-namespace", result.Namespace)
}

func TestGenericGetOrCreate(t *testing.T) {
	// Add the AstraInstaller object to the fake client
	handler, _ := createResourceHandlerWithFakeClient(t)

	t.Run("Test with ConfigMap resource", func(t *testing.T) {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-configmap",
				Namespace: "test-namespace",
			},
		}

		resource, err := handler.GenericGetOrCreate(cm, nil)
		assert.NoError(t, err)
		assert.NotNil(t, resource)
		assert.IsType(t, &corev1.ConfigMap{}, resource)
	})
}
