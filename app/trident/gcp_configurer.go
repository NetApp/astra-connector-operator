// Copyright 2023 NetApp, Inc. All Rights Reserved.

package trident

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/NetApp-Polaris/astra-connector-operator/app/trident/model"
	log "github.com/sirupsen/logrus"
	"strings"

	tridentcliapi "github.com/netapp/trident/cli/api"
	tridentdrivers "github.com/netapp/trident/storage_drivers"
	tridentdriversgcp "github.com/netapp/trident/storage_drivers/gcp/api"
	v1 "k8s.io/api/core/v1"
)

func (i *Installer) configureTridentForGCP() error {

	if i.trident.Spec.GCP == nil {
		return errors.New("cannot configure Trident for GCP, no GCP details provided")
	}
	if i.trident.Spec.GCP.Credentials == nil {
		return errors.New("cannot configure Trident for GCP, no GCP credentials provided")
	}

	// Get the Trident pod
	tridentPod, err := i.findTridentPodAndContainer()
	if err != nil {
		return fmt.Errorf("could not find Trident pod; %v", err)
	}

	// Determine which storage classes are needed, which will inform the backend virtual pools
	scNames := i.getSupportedGCPStorageClasses()

	// Add Trident GCP backend
	backendExists, backendName, err := i.tridentHasGCPBackend(tridentPod)
	if err != nil {
		return fmt.Errorf("could not check for Trident GCP backend; %v", err)
	}
	if !backendExists {
		if err := i.createGCPBackend(tridentPod, scNames); err != nil {
			return fmt.Errorf("could not create Trident GCP backend; %v", err)
		}
	} else {
		if err := i.updateGCPBackend(tridentPod, backendName, scNames); err != nil {
			return fmt.Errorf("could not update Trident GCP backend; %v", err)
		}
	}

	// Get the default storage class
	defaultStorageClassName, err := i.getDefaultStorageClassName("")
	if err != nil {
		return fmt.Errorf("could not determine default storage class; %v", err)
	}

	// Create GCP StorageClasses
	if err = i.createOrPatchGCPStorageClasses(scNames, defaultStorageClassName); err != nil {
		return err
	}

	// Create VolumeStorageClass
	err = i.createOrPatchVolumeSnapshotClass(CVSSnapshotClassName)
	if err != nil {
		return fmt.Errorf("could not create volume snapshot class, check if volume snapshot CRDs exist; %v", err)
	}

	return nil
}

// Read GCP extension config map
func (i *Installer) getGCPExtensionConfigMap() (map[string]string, error) {

	configMap, err := i.controlPlaneClients.KubeClient.CoreV1().ConfigMaps(ControlPlaneNamespace).Get(ctx(), GCPConfigMap, getOpts)
	if err != nil {
		log.WithFields(log.Fields{
			"name":      GCPConfigMap,
			"namespace": ControlPlaneNamespace,
			"error":     err,
		}).Error("Could not read GCP config map.")
		return nil, err
	}

	return configMap.Data, nil
}

// Check whether hardware or software GCP storage class(es) are required
func (i *Installer) getSupportedGCPStorageClasses() []string {

	defaultSCNames := []string{
		CVSStorageClassHardwareStandard, CVSStorageClassHardwarePremium, CVSStorageClassHardwareExtreme,
	}

	configMap, err := i.getGCPExtensionConfigMap()
	if err != nil {
		log.Warn("No GCP config map, defaulting to hardware CVS storage classes.")
		return defaultSCNames
	}

	regionMapJSON, ok := configMap[GCPConfigMapRegionKey]
	if !ok {
		log.Warn("No GCP config map region data, defaulting to hardware CVS storage classes.")
		return defaultSCNames
	}
	regionMapJSON = strings.TrimSpace(regionMapJSON)

	regionMap := make(map[string][]string)
	if err := json.Unmarshal([]byte(regionMapJSON), &regionMap); err != nil {
		log.Warn("Invalid GCP config map region data, defaulting to hardware CVS storage classes.")
		return defaultSCNames
	}

	scNames := regionMap[i.trident.Spec.GCP.APIRegion]

	// Neither defaults to hardware
	if scNames == nil || len(scNames) == 0 {
		log.Warn("Empty GCP config map region data, defaulting to hardware CVS storage classes.")
		scNames = defaultSCNames
	}

	log.WithFields(log.Fields{
		"region":         i.trident.Spec.GCP.APIRegion,
		"storageClasses": scNames,
	}).Info("Determined region-based CVS storage class.")

	return scNames
}

func (i *Installer) buildTridentGCPBackend(scNames []string) *tridentdrivers.GCPNFSStorageDriverConfig {

	defaultPool := tridentdrivers.GCPNFSStorageDriverPool{
		Labels:  map[string]string{"cloud": "gcp"},
		Network: i.trident.Spec.GCP.Network,
	}

	// Add tridentInstance metadata labels to backend config
	for key, value := range model.LabelsToMap(i.trident.Metadata.Labels) {
		defaultPool.Labels[key] = value
	}

	var serviceLevelPools []tridentdrivers.GCPNFSStorageDriverPool

	for _, scName := range scNames {

		switch scName {
		case CVSStorageClassHardwareStandard:
			serviceLevelPools = append(serviceLevelPools, tridentdrivers.GCPNFSStorageDriverPool{
				Labels: map[string]string{
					"serviceLevel": tridentdriversgcp.UserServiceLevel1, // standard
					"storageClass": CVSStorageClassHardware,
				},
				ServiceLevel: tridentdriversgcp.UserServiceLevel1,
				StorageClass: CVSStorageClassHardware,
			})

		case CVSStorageClassHardwarePremium:
			serviceLevelPools = append(serviceLevelPools, tridentdrivers.GCPNFSStorageDriverPool{
				Labels: map[string]string{
					"serviceLevel": tridentdriversgcp.UserServiceLevel2, // premium
					"storageClass": CVSStorageClassHardware,
				},
				ServiceLevel: tridentdriversgcp.UserServiceLevel2,
				StorageClass: CVSStorageClassHardware,
			})

		case CVSStorageClassHardwareExtreme:
			serviceLevelPools = append(serviceLevelPools, tridentdrivers.GCPNFSStorageDriverPool{
				Labels: map[string]string{
					"serviceLevel": tridentdriversgcp.UserServiceLevel3, // extreme
					"storageClass": CVSStorageClassHardware,
				},
				ServiceLevel: tridentdriversgcp.UserServiceLevel3,
				StorageClass: CVSStorageClassHardware,
			})

		case CVSStorageClassSoftwareStandard:
			serviceLevelPools = append(serviceLevelPools, tridentdrivers.GCPNFSStorageDriverPool{
				Labels: map[string]string{
					"serviceLevel": tridentdriversgcp.PoolServiceLevel1, // standardsw
					"storageClass": CVSStorageClassSoftware,
				},
				ServiceLevel: tridentdriversgcp.PoolServiceLevel1,
				StorageClass: CVSStorageClassSoftware,
			})

		}
	}

	return &tridentdrivers.GCPNFSStorageDriverConfig{
		CommonStorageDriverConfig: &tridentdrivers.CommonStorageDriverConfig{
			Version:           1,
			StorageDriverName: "gcp-cvs",
			BackendName:       TridentGCPBackendNameAstra,
		},
		ProjectNumber:           i.trident.Spec.GCP.ProjectNumber,
		HostProjectNumber:       i.trident.Spec.GCP.HostProjectNumber,
		APIKey:                  *i.trident.Spec.GCP.Credentials,
		APIRegion:               i.trident.Spec.GCP.APIRegion,
		APIURL:                  i.trident.Spec.GCP.APIURL,
		APIAudienceURL:          i.trident.Spec.GCP.APIAudienceURL,
		ProxyURL:                i.trident.Spec.ProxyURL,
		NfsMountOptions:         i.trident.Spec.NFSMountOptions,
		GCPNFSStorageDriverPool: defaultPool,
		Storage:                 serviceLevelPools,
	}
}

func (i *Installer) tridentHasGCPBackend(tridentPod *v1.Pod) (bool, string, error) {

	commandArgs := []string{"tridentctl", "get", "backends", "-o", "json"}
	response, err := i.clients.K8SClient.Exec(tridentPod.Name, TridentCSIContainer, tridentPod.Namespace, commandArgs)
	if err != nil {
		return false, "", fmt.Errorf("could not communicate with Trident pod; %v", err)
	}

	var backends tridentcliapi.MultipleBackendResponse
	if err := json.Unmarshal(response, &backends); err != nil {
		return false, "", fmt.Errorf("invalid trident backend response: %v", err)
	}

	for _, backend := range backends.Items {
		if backend.Name == TridentGCPBackendNameAstra {
			return true, TridentGCPBackendNameAstra, nil
		}
	}
	for _, backend := range backends.Items {
		if backend.Name == TridentGCPBackendNamePolaris {
			return true, TridentGCPBackendNamePolaris, nil
		}
	}

	return false, "", nil
}

func (i *Installer) createGCPBackend(tridentPod *v1.Pod, scNames []string) error {

	gcpConfig := i.buildTridentGCPBackend(scNames)
	configJSONBytes, err := json.Marshal(gcpConfig)
	if err != nil {
		return err
	}

	commandArgs := []string{
		"tridentctl", "create", "backend", "--base64",
		base64.StdEncoding.EncodeToString(configJSONBytes),
	}
	_, err = i.clients.K8SClient.Exec(tridentPod.Name, TridentCSIContainer, tridentPod.Namespace, commandArgs)
	return err
}

func (i *Installer) updateGCPBackend(tridentPod *v1.Pod, backendName string, scNames []string) error {

	gcpConfig := i.buildTridentGCPBackend(scNames)
	configJSONBytes, err := json.Marshal(gcpConfig)
	if err != nil {
		return err
	}

	commandArgs := []string{
		"tridentctl", "update", "backend", backendName, "--base64",
		base64.StdEncoding.EncodeToString(configJSONBytes),
	}
	_, err = i.clients.K8SClient.Exec(tridentPod.Name, TridentCSIContainer, tridentPod.Namespace, commandArgs)
	return err
}

func (i *Installer) createOrPatchGCPStorageClasses(scNames []string, defaultStorageClassName string) error {

	var serviceLevel string
	var isDefault, isHardware bool

	for _, scName := range scNames {

		switch scName {

		case CVSStorageClassHardwareStandard:
			serviceLevel = tridentdriversgcp.UserServiceLevel1
			isHardware = true

		case CVSStorageClassHardwarePremium:
			serviceLevel = tridentdriversgcp.UserServiceLevel2
			isHardware = true

		case CVSStorageClassHardwareExtreme:
			serviceLevel = tridentdriversgcp.UserServiceLevel3
			isHardware = true

		case CVSStorageClassSoftwareStandard:
			serviceLevel = tridentdriversgcp.UserServiceLevel1
			isHardware = false
		}

		isDefault = scName == defaultStorageClassName

		scYAML := GetGCPStorageClassYAML(scName, "gcp-cvs", serviceLevel, isDefault, isHardware)

		log.WithFields(log.Fields{
			"storageClass": scName,
			"isDefault":    isDefault,
			"isHardware":   isHardware,
			"serviceLevel": serviceLevel,
		}).Info("Creating/patching storage class.")

		if err := i.createOrPatchCVSStorageClass(scName, scYAML); err != nil {
			return err
		}
	}

	return nil
}
