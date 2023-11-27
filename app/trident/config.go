// Copyright 2023 NetApp, Inc. All Rights Reserved.
package trident

import "time"

const (
	K8sTimeout = time.Minute * 5

	TridentOperatorMinVersion       = "21.01.0"
	TridentMinVersionWithACPSupport = "23.10.0"

	ControlPlaneNamespace = "pcloud"
	GCPConfigMap          = "cloud-extension-gcp-configmap"
	GCPConfigMapRegionKey = "regionStorageClass.json"
	ANFConfigMapRegionKey = "regionStorageClass.json"

	TridentSvcConfigMap          = "trident-svc-configmap"
	TridentSvcConfigMapImagesKey = "trident-svc.json"
	TridentImageKey              = "tridentImage"
	TridentAutosupportImageKey   = "tridentAutosupportImage"
	TridentOperatorImageKey      = "tridentOperatorImage"
	ACPImageKey                  = "acpImage"

	TridentOperatorLabelKey   = "app"
	TridentOperatorLabelValue = "operator.trident.netapp.io"
	TridentOperatorLabel      = TridentOperatorLabelKey + "=" + TridentOperatorLabelValue

	TridentOperatorNamespace              = "trident"
	TridentOperatorServiceAccount         = "trident-operator"
	TridentOperatorClusterRoleName        = "trident-operator"
	TridentOperatorClusterRoleBindingName = "trident-operator"
	TridentOperatorDeploymentName         = "trident-operator"
	TridentOperatorContainerName          = "trident-operator"
	TridentOperatorPodSecurityPolicy      = "tridentoperatorpods"
	TridentOperatorImagePullSecretName    = "trident-imagepullsecrets"

	TridentOrchestratorCRDName = "tridentorchestrators.trident.netapp.io"

	AppStatusNoStatus       = ""
	AppStatusInstalling     = "Installing"
	AppStatusInstalled      = "Installed"
	AppStatusUninstalling   = "Uninstalling"
	AppStatusUninstalled    = "Uninstalled"
	AppStatusUninstalledAll = "UninstalledAll"
	AppStatusFailed         = "Failed"
	AppStatusUpdating       = "Updating"
	AppStatusError          = "Error"

	TridentGCPBackendNamePolaris        = "gcp-polaris"
	TridentGCPBackendNameAstra          = "gcp-astra"
	TridentANFBackendNameAstra          = "anf-astra"
	TridentANFSubvolumeBackendNameAstra = "anf-subvolume-astra"

	ArtifactoryImagePullCredsBase64 = "eyJhdXRocyI6eyJuZXRhcHBkb3dubG9hZHMuamZyb2cuaW8iOnsidXNlcm5hbWUiOiJ0cmlkZW50LWRvd25sb2FkZXJzLWZvci1hc3RyYSIsInBhc3N3b3JkIjoiZjI5eWwzQWZHUm5HZS1ncGVGUVljZ3FlZHFLV2FEcVZTNUlwYkdBcVF0cyIsImVtYWlsIjoicHJvamVjdGFzdHJhLnN1cHBvcnRAbmV0YXBwLmNvbSIsImF1dGgiOiJkSEpwWkdWdWRDMWtiM2R1Ykc5aFpHVnljeTFtYjNJdFlYTjBjbUU2WmpJNWVXd3pRV1pIVW01SFpTMW5jR1ZHVVZsalozRmxaSEZMVjJGRWNWWlROVWx3WWtkQmNWRjBjdz09In19fQ=="

	CVSStorageClassHardwareStandard = "netapp-cvs-perf-standard"
	CVSStorageClassHardwarePremium  = "netapp-cvs-perf-premium"
	CVSStorageClassHardwareExtreme  = "netapp-cvs-perf-extreme"
	CVSStorageClassSoftwareStandard = "netapp-cvs-standard"

	CVSStorageClassHardware = "hardware"
	CVSStorageClassSoftware = "software"

	ANFStorageClassHardwareStandard = "netapp-anf-perf-standard"
	ANFStorageClassHardwarePremium  = "netapp-anf-perf-premium"
	ANFStorageClassHardwareUltra    = "netapp-anf-perf-ultra"

	ANFStorageClassHardwareSubvolumeStandard = "netapp-anf-subvolume-standard"
	ANFStorageClassHardwareSubvolumePremium  = "netapp-anf-subvolume-premium"
	ANFStorageClassHardwareSubvolumeUltra    = "netapp-anf-subvolume-ultra"

	CVSSnapshotClassName              = "netapp-cvs-snapshot-class"
	ANFSnapshotClassName              = "netapp-anf-snapshot-class"
	OntapNasEconomyStorageClassDriver = "ontap-nas-economy"
)
