/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package common

import (
	_ "embed"
	"strings"
)

const (
	DefaultImageRegistry = "netappdownloads.jfrog.io/docker-astra-control-staging/arch30/neptune"
	AstraImageRegistry   = "cr.astra.netapp.io"

	AstraConnectName                 = "astraconnect"
	AstraConnectorOperatorRepository = "netapp/astra-connector-operator"

	DefaultCloudAstraControlURL = "https://astra.netapp.io"

	NeptuneName = "neptune-controller-manager"

	NeptuneMetricServicePort     = 8443
	NeptuneMetricServiceProtocol = "TCP"
	NeptuneReplicas              = 1

	RbacProxyImage = "kube-rbac-proxy:v0.14.1"
)

// Embed image tags

//go:embed "neptune_manager_tag.txt"
var embeddedNeptuneImageTag string

//go:embed "connector_version.txt"
var embeddedConnectorImageTag string

//go:embed "neptune_asup_tag.txt"
var embeddedAsupImageTag string

var (
	// NeptuneImageTag is the trimmed version of the embedded string.
	NeptuneImageTag   = strings.TrimSpace(embeddedNeptuneImageTag)
	ConnectorImageTag = strings.TrimSpace(embeddedConnectorImageTag)
	AsupImageTag      = strings.TrimSpace(embeddedAsupImageTag)
)

func GetNeptuneRepositories() []string {
	return []string{"controller", "exechook", "resourcebackup", "resourcedelete", "resourcerestore", "resourcesummaryupload", "restic"}
}
