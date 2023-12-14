/*
 * Copyright (c) 2023. NetApp, Inc. All Rights Reserved.
 */

package k8s

import (
	"context"
	"k8s.io/apimachinery/pkg/types"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	rbacv1 "k8s.io/api/rbac/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/NetApp-Polaris/astra-connector-operator/util"
)

type K8sUtil struct {
	Client client.Client
	log    logr.Logger
}

type K8sUtilInterface interface {
	CreateOrUpdateResource(context.Context, client.Object, client.Object) error
	DeleteResource(context.Context, client.Object) error
	VersionGet() (string, error)
	IsCRDInstalled(string) bool
}

func NewK8sUtil(c client.Client, log logr.Logger) K8sUtilInterface {
	return &K8sUtil{Client: c, log: log}
}

// CreateOrUpdateResource creates a role, provided a namespace and name
// If it finds a role with the same name as the provided argument, it will return that instead
func (r *K8sUtil) CreateOrUpdateResource(ctx context.Context, resource client.Object, owner client.Object) error {
	if isNamespaceScoped(resource) && !util.IsNil(owner) {
		err := ctrl.SetControllerReference(owner, resource, r.Client.Scheme())
		if err != nil {
			return err
		}
	}

	// Define the MutateFn function
	mutateFn := func() error {
		// TODO https://jira.ngage.netapp.com/browse/ASTRACTL-27555
		// Apply any desired changes to the deployment object here
		// For example, you can update the environment variables, container image, etc.
		// want to remove duplicated code like each deployer setting image and secret can be do once here
		return nil
	}

	// Use the ctrl.CreateOrUpdate function with the MutateFn function
	_, err := ctrl.CreateOrUpdate(ctx, r.Client, resource, mutateFn)
	return err
}

func (r *K8sUtil) DeleteResource(ctx context.Context, resource client.Object) error {
	return r.Client.Delete(ctx, resource)
}

func isNamespaceScoped(obj client.Object) bool {
	switch obj.(type) {
	case *rbacv1.ClusterRole, *rbacv1.ClusterRoleBinding:
		return false
	default:
		return true
	}
}

// VersionGet returns the server version of the k8s cluster.
func (r *K8sUtil) VersionGet() (string, error) {
	restConfig, err := ctrl.GetConfig()
	if err != nil {
		return "", errors.Wrap(err, "error getting kubeconfig")
	}
	dClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return "", errors.Wrap(err, "error creating discovery client")
	}
	versionInfo, err := dClient.ServerVersion()
	if err != nil {
		return "", errors.Wrap(err, "error getting server version")
	}
	r.log.V(3).Info("versionInfo", "versionInfo", versionInfo)
	return versionInfo.GitVersion, nil
}

// IsCRDInstalled returns the server version of the k8s cluster.
func (r *K8sUtil) IsCRDInstalled(crdName string) bool {
	crd := &apiextv1.CustomResourceDefinition{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: crdName}, crd)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.log.V(3).Info(crdName + " CRD does not exist")
			return false
		} else {
			r.log.V(3).Info("Failed to get CRD: "+crdName, err)
			return false
		}
	} else {
		r.log.V(3).Info(crdName + " CRD exists")
		return true

	}
}
