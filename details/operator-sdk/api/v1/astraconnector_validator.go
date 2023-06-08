// Copyright (c) 2023 NetApp, Inc. All Rights Reserved.

package v1

import "k8s.io/apimachinery/pkg/util/validation/field"

func (ai *AstraConnector) ValidateCreateAstraConnector() field.ErrorList {
	// TODO - Add validations here
	astraConnectorLog.Info("Creating AstraConnector resource")
	return nil
}

func (ai *AstraConnector) ValidateUpdateAstraConnector() field.ErrorList {
	// TODO - Add validations here
	astraConnectorLog.Info("Updating AstraConnector resource")
	return nil
}
