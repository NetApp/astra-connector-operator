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
	err := p.RunK8sVersionCheck()
	if err != nil {
		errList = append(errList, err)
	}

	err = p.RunK8sCRDCheck()
	if err != nil {
		errList = append(errList, err)
	}

	return errList
}
