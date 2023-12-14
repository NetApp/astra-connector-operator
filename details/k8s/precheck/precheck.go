// Copyright 2023 NetApp, Inc. All Rights Reserved.

package precheck

import (
	"github.com/go-logr/logr"

	"github.com/NetApp-Polaris/astra-connector-operator/details/k8s"
)

type SetWarning func(message string) error

type PrecheckClient struct {
	k8sUtil k8s.K8sUtilInterface
	log     logr.Logger
}

func NewPrecheckClient(log logr.Logger, k8sUtil k8s.K8sUtilInterface) *PrecheckClient {
	return &PrecheckClient{
		k8sUtil: k8sUtil,
		log:     log,
	}
}

func (p *PrecheckClient) Run() []error {
	var errList []error
	errList = append(errList, p.RunK8sVersionCheck())
	errList = append(errList, p.RunK8sCRDCheck())

	return errList
}
