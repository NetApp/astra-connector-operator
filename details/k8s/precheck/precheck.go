// Copyright 2023 NetApp, Inc. All Rights Reserved.

package precheck

import (
	"context"
	"github.com/NetApp-Polaris/astra-connector-operator/details/k8s"
	"github.com/go-logr/logr"
)

type SetWarning func(message string) error

type PrecheckClient struct {
	k8sUtil k8s.K8sUtil
	log     logr.Logger
}

func NewCheckClient(ctx context.Context, log logr.Logger, k8sUtil k8s.K8sUtil) *PrecheckClient {
	return &PrecheckClient{
		k8sUtil: k8sUtil,
		log:     log,
	}
}
