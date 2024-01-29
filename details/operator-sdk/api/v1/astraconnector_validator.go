// Copyright (c) 2023 NetApp, Inc. All Rights Reserved.

package v1

import (
	"context"

	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/NetApp-Polaris/astra-connector-operator/util"
)

var log = ctrllog.FromContext(context.TODO())

func (ai *AstraConnector) ValidateCreateAstraConnector() field.ErrorList {
	var allErrs field.ErrorList

	if err := ai.ValidateNamespace(); err != nil {
		log.V(3).Info("error while creating AstraConnector Instance", "namespace", ai.Namespace, "err", err)
		allErrs = append(allErrs, err)
	}

	if err := ai.validateClusterName(); err != nil {
		log.V(3).Info(
			"error while creating AstraConnector Instance; invalid cluster name",
			"name", ai.Spec.Astra.ClusterName, "err", err,
		)
		allErrs = append(allErrs, err)
	}

	return allErrs
}

func (ai *AstraConnector) ValidateUpdateAstraConnector() field.ErrorList {
	// TODO - Add validations here
	astraConnectorLog.Info("Updating AstraConnector resource")
	return nil
}

// ValidateNamespace Validates the namespace that AstraConnector should be deployed to.
func (ai *AstraConnector) ValidateNamespace() *field.Error {
	namespaceJsonField := util.GetJSONFieldName(&ai.ObjectMeta, &ai.ObjectMeta.Namespace)
	if ai.GetNamespace() == "default" {
		log.Info("Deploying to default namespace is not allowed. Please select a different namespace.")
		return field.Invalid(field.NewPath(namespaceJsonField), ai.Name, "default namespace not allowed")
	}
	return nil
}

// validateClusterName Validates the cluster name that AstraConnector should be deployed to.
func (ai *AstraConnector) validateClusterName() *field.Error {
	// Validate that the name is non-empty and is a valid Kubernetes label.
	name := ai.Spec.Astra.ClusterName
	if !util.IsValidDNS1123Label(name) {
		fieldPath := util.GetJSONFieldName(&ai.Spec.Astra, &ai.Spec.Astra.ClusterName)
		return field.Invalid(field.NewPath(fieldPath), name,
			"names must consist of lower case alphanumeric characters or '-', "+
				"and must start and end with an alphanumeric character (for example 'my-name',  or '123-abc')")
	}
	return nil
}
