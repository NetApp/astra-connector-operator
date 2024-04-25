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
