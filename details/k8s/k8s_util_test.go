/*
 * Copyright (c) 2023. NetApp, Inc. All Rights Reserved.
 */

package k8s_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NetApp-Polaris/astra-connector-operator/details/k8s"
	testutil "github.com/NetApp-Polaris/astra-connector-operator/test/test-util"
)

var ctx = context.Background()

func createResourceHandlerWithFakeClient(initObjs ...client.Object) (k8s.K8sUtilInterface, client.Client) {
	fakeClient := testutil.CreateFakeClient(initObjs...)
	k8sUtil := k8s.NewK8sUtil(fakeClient)
	return k8sUtil, fakeClient
}

func TestNewResourceHandler(t *testing.T) {
	handler, _ := createResourceHandlerWithFakeClient()

	assert.NotNil(t, handler)
}

func TestCreateOrUpdateResource(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = rbacv1.AddToScheme(scheme)

	k8sUtil, k8sClient := createResourceHandlerWithFakeClient()

	t.Run("create namespace scoped resource", func(t *testing.T) {
		role := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-role",
				Namespace: "test-namespace",
			},
		}

		err := k8sUtil.CreateOrUpdateResource(ctx, role, nil)
		assert.NoError(t, err)

		var createdRole rbacv1.Role
		err = k8sClient.Get(ctx, client.ObjectKey{Name: role.Name, Namespace: role.Namespace}, &createdRole)
		assert.NoError(t, err)
		assert.Equal(t, role.Name, createdRole.Name)
	})

	t.Run("create cluster scoped resource", func(t *testing.T) {
		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cluster-role",
			},
		}

		err := k8sUtil.CreateOrUpdateResource(ctx, clusterRole, nil)
		assert.NoError(t, err)

		var createdClusterRole rbacv1.ClusterRole
		err = k8sClient.Get(ctx, client.ObjectKey{Name: clusterRole.Name}, &createdClusterRole)
		assert.NoError(t, err)
		assert.Equal(t, clusterRole.Name, createdClusterRole.Name)
	})

	t.Run("update namespace scoped resource", func(t *testing.T) {
		role := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-role",
				Namespace: "test-namespace",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"*"},
					Resources: []string{"*"},
					Verbs:     []string{"*"},
				},
			},
		}

		err := k8sUtil.CreateOrUpdateResource(ctx, role, nil)
		assert.NoError(t, err)

		var updatedRole rbacv1.Role
		err = k8sClient.Get(ctx, client.ObjectKey{Name: role.Name, Namespace: role.Namespace}, &updatedRole)
		assert.NoError(t, err)
		assert.Equal(t, role.Name, updatedRole.Name)
		assert.Equal(t, role.Rules, updatedRole.Rules)
	})

	t.Run("update cluster scoped resource", func(t *testing.T) {
		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cluster-role",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"*"},
					Resources: []string{"*"},
					Verbs:     []string{"*"},
				},
			},
		}

		err := k8sUtil.CreateOrUpdateResource(ctx, clusterRole, nil)
		assert.NoError(t, err)

		var updatedClusterRole rbacv1.ClusterRole
		err = k8sClient.Get(ctx, client.ObjectKey{Name: clusterRole.Name}, &updatedClusterRole)
		assert.NoError(t, err)
		assert.Equal(t, clusterRole.Name, updatedClusterRole.Name)
		assert.Equal(t, clusterRole.Rules, updatedClusterRole.Rules)
	})
}
