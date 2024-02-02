/*
 * Copyright (c) 2023. NetApp, Inc. All Rights Reserved.
 */

package k8s_test

import (
	"context"
	"github.com/NetApp-Polaris/astra-connector-operator/app/deployer/model"
	"testing"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NetApp-Polaris/astra-connector-operator/details/k8s"
	testutil "github.com/NetApp-Polaris/astra-connector-operator/test/test-util"
)

var ctx = context.Background()

func createResourceHandlerWithFakeClient(t *testing.T, initObjs ...client.Object) (k8s.K8sUtilInterface, client.Client) {
	fakeClient := testutil.CreateFakeClient(initObjs...)
	fakeClientSet := fake.NewSimpleClientset()
	log := testutil.CreateLoggerForTesting(t)
	k8sUtil := k8s.NewK8sUtil(fakeClient, fakeClientSet, log)
	return k8sUtil, fakeClient
}

func TestNewResourceHandler(t *testing.T) {
	handler, _ := createResourceHandlerWithFakeClient(t)

	assert.NotNil(t, handler)
}

func TestCreateOrUpdateResource(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = rbacv1.AddToScheme(scheme)

	k8sUtil, k8sClient := createResourceHandlerWithFakeClient(t)

	t.Run("create namespace scoped resource", func(t *testing.T) {
		role := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-role",
				Namespace: "test-namespace",
			},
		}

		resultString, err := k8sUtil.CreateOrUpdateResource(ctx, role, nil, model.NonMutateFn)
		assert.NoError(t, err)
		assert.Equal(t, "created", resultString)

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

		resultString, err := k8sUtil.CreateOrUpdateResource(ctx, clusterRole, nil, model.NonMutateFn)
		assert.NoError(t, err)
		assert.Equal(t, "created", resultString)

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

		resultString, err := k8sUtil.CreateOrUpdateResource(ctx, role, nil, model.NonMutateFn)
		assert.NoError(t, err)
		assert.Equal(t, "unchanged", resultString)

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

		resultString, err := k8sUtil.CreateOrUpdateResource(ctx, clusterRole, nil, model.NonMutateFn)
		assert.NoError(t, err)
		assert.Equal(t, "unchanged", resultString)

		var updatedClusterRole rbacv1.ClusterRole
		err = k8sClient.Get(ctx, client.ObjectKey{Name: clusterRole.Name}, &updatedClusterRole)
		assert.NoError(t, err)
		assert.Equal(t, clusterRole.Name, updatedClusterRole.Name)
		assert.Equal(t, clusterRole.Rules, updatedClusterRole.Rules)
	})
}

func TestDeleteResource(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = rbacv1.AddToScheme(scheme)

	k8sUtil, k8sClient := createResourceHandlerWithFakeClient(t)

	t.Run("create and delete cluster scoped resource", func(t *testing.T) {
		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cluster-role",
			},
		}

		// Create and verify the resource
		resultString, err := k8sUtil.CreateOrUpdateResource(ctx, clusterRole, nil, model.NonMutateFn)
		assert.NoError(t, err)
		assert.Equal(t, "created", resultString)

		var createdClusterRole rbacv1.ClusterRole
		err = k8sClient.Get(ctx, client.ObjectKey{Name: clusterRole.Name}, &createdClusterRole)
		assert.NoError(t, err)
		assert.Equal(t, clusterRole.Name, createdClusterRole.Name)

		// Delete and verify the resource
		err = k8sUtil.DeleteResource(ctx, clusterRole)
		assert.NoError(t, err)

		err = k8sClient.Get(ctx, client.ObjectKey{Name: clusterRole.Name}, &createdClusterRole)
		assert.EqualError(t, err, "clusterroles.rbac.authorization.k8s.io \"test-cluster-role\" not found")
	})

	t.Run("delete non-existing resource", func(t *testing.T) {
		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cluster-role",
			},
		}

		// Delete and verify the resource
		err := k8sUtil.DeleteResource(ctx, clusterRole)
		assert.EqualError(t, err, "clusterroles.rbac.authorization.k8s.io \"test-cluster-role\" not found")
	})
}
