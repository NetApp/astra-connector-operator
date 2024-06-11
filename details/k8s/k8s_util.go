/*
 * Copyright (c) 2023. NetApp, Inc. All Rights Reserved.
 */

package k8s

import (
	"context"
	"github.com/go-logr/logr"
	semver "github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/NetApp-Polaris/astra-connector-operator/util"
)

type K8sUtil struct {
	client.Client
	kubernetes.Interface
	Log logr.Logger
}

type K8sUtilInterface interface {
	CreateOrUpdateResource(context.Context, client.Object, client.Object, controllerutil.MutateFn) (string, error)
	DeleteResource(context.Context, client.Object) error
	VersionGet() (string, string, error)
	IsCRDInstalled(string) bool
	RESTGet(string) ([]byte, error)
	K8sClientset() kubernetes.Interface
}

func NewK8sUtil(c client.Client, i kubernetes.Interface, log logr.Logger) K8sUtilInterface {
	return &K8sUtil{Client: c, Interface: i, Log: log}
}

// CreateOrUpdateResource creates a role, provided a namespace and name
// If it finds a role with the same name as the provided argument, it will return that instead
func (r *K8sUtil) CreateOrUpdateResource(ctx context.Context, resource client.Object, owner client.Object, f controllerutil.MutateFn) (string, error) {
	if isNamespaceScoped(resource) && !util.IsNil(owner) {
		err := ctrl.SetControllerReference(owner, resource, r.Client.Scheme())
		if err != nil {
			return "", err
		}
	}

	// Use the ctrl.CreateOrUpdate function with the MutateFn function
	operationResult, err := ctrl.CreateOrUpdate(ctx, r.Client, resource, f)
	return string(operationResult), err
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
func (r *K8sUtil) VersionGet() (string, string, error) {
	restConfig, err := ctrl.GetConfig()
	if err != nil {
		return "", "", errors.Wrap(err, "error getting kubeconfig")
	}
	dClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return "", "", errors.Wrap(err, "error creating discovery client")
	}
	versionInfo, err := dClient.ServerVersion()
	if err != nil {
		return "", "", errors.Wrap(err, "error getting server version")
	}
	fullVersionString := versionInfo.String()
	semanticVersion, err := semver.NewSemver(fullVersionString)
	if err != nil {
		r.Log.Error(err, "failed to parse k8s server version")
		return "", "", err
	}
	r.Log.V(3).Info("versionInfo", "versionInfo", versionInfo)
	return fullVersionString, semanticVersion.String(), nil
}

// IsCRDInstalled returns the server version of the k8s cluster.
func (r *K8sUtil) IsCRDInstalled(crdName string) bool {
	crd := &apiextv1.CustomResourceDefinition{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: crdName}, crd)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.Log.V(3).Info(crdName + " CRD does not exist")
			return false
		} else {
			r.Log.V(3).Info("Failed to get CRD: "+crdName, err)
			print(err.Error())
			return false
		}
	} else {
		r.Log.V(3).Info(crdName + " CRD exists")
		return true

	}
}

// RESTGet Makes a GET request on the K8s Rest Client and returns the raw byte array
func (r *K8sUtil) RESTGet(path string) ([]byte, error) {
	return r.Interface.Discovery().RESTClient().Get().AbsPath(path).DoRaw(context.TODO())
}

// K8sClientset Returns the k8s Clientset
func (r *K8sUtil) K8sClientset() kubernetes.Interface {
	return r.Interface
}
