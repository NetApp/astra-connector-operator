// Copyright 2020 NetApp, Inc. All Rights Reserved.

package trident

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/ghodss/yaml"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	v12 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	commontypes "k8s.io/apimachinery/pkg/types"
)

func (i *Installer) patchTridentServiceAccount(currentServiceAccount *v1.ServiceAccount,
	newServiceAccountYAML []byte) error {

	// Identify the deltas
	patchBytes, err := i.genericPatch(currentServiceAccount, newServiceAccountYAML, &v1.ServiceAccount{})
	if err != nil {
		return fmt.Errorf("error in creating the two-way merge patch for Trident operator service account %q: %v",
			currentServiceAccount.Name, err)
	}

	// Apply the patch to the current Service Account
	err = i.clients.K8SClient.PatchServiceAccountByLabel(TridentOperatorLabel, patchBytes)
	if err != nil {
		return fmt.Errorf("could not patch Trident operator service account; %v", err)
	}
	log.Debug("Patched Trident operator service account.")

	return nil
}

func (i *Installer) patchTridentClusterRole(currentClusterRole *v12.ClusterRole,
	newClusterRoleYAML []byte) error {

	// Identify the deltas
	patchBytes, err := i.genericPatch(currentClusterRole, newClusterRoleYAML, &v12.ClusterRole{})
	if err != nil {
		return fmt.Errorf("error in creating the two-way merge patch for Trident operator cluster role %q: %v",
			currentClusterRole.Name, err)
	}

	// Apply the patch to the current Cluster Role
	err = i.clients.K8SClient.PatchClusterRoleByLabel(TridentOperatorLabel, patchBytes)
	if err != nil {
		return fmt.Errorf("could not patch Trident operator cluster role; %v", err)
	}
	log.Debug("Patched Trident operator cluster role.")

	return nil
}

func (i *Installer) patchTridentClusterRoleBinding(currentClusterRoleBinding *v12.ClusterRoleBinding,
	newClusterRoleBindingYAML []byte) error {

	// Identify the deltas
	patchBytes, err := i.genericPatch(currentClusterRoleBinding, newClusterRoleBindingYAML, &v12.ClusterRoleBinding{})
	if err != nil {
		return fmt.Errorf("error in creating the two-way merge patch for Trident operator cluster role binding %q: %v",
			currentClusterRoleBinding.Name, err)
	}

	// Apply the patch to the current Cluster Role Binding
	err = i.clients.K8SClient.PatchClusterRoleBindingByLabel(TridentOperatorLabel, patchBytes)
	if err != nil {
		return fmt.Errorf("could not patch Trident operator cluster role binding; %v", err)
	}
	log.Debug("Patched Trident operator cluster role binding.")

	return nil
}

func (i *Installer) patchTridentPodSecurityPolicy(currentPSP *v1beta1.PodSecurityPolicy, newPSPYAML []byte) error {

	// Identify the deltas
	patchBytes, err := i.genericPatch(currentPSP, newPSPYAML, &v1beta1.PodSecurityPolicy{})
	if err != nil {
		return fmt.Errorf("error in creating the two-way merge patch for Trident operator pod security policy %q: %v",
			currentPSP.Name, err)
	}

	// Apply the patch to the current Pod Security Policy
	err = i.clients.K8SClient.PatchPodSecurityPolicyByLabel(TridentOperatorLabel, patchBytes)
	if err != nil {
		return fmt.Errorf("could not patch Trident operator pod security policy; %v", err)
	}
	log.Debug("Patched Trident operator pod security policy.")

	return nil
}

func (i *Installer) patchTridentImagePullSecret(currentSecret *v1.Secret, newSecretYAML []byte) error {

	// Identify the deltas
	patchBytes, err := i.genericPatch(currentSecret, newSecretYAML, &v1.Secret{})
	if err != nil {
		return fmt.Errorf("error in creating the two-way merge patch for Trident operator image pull secret %q: %v",
			currentSecret.Name, err)
	}

	// Apply the patch to the current secret
	err = i.clients.K8SClient.PatchSecretByLabel(TridentOperatorLabel, patchBytes)
	if err != nil {
		return fmt.Errorf("could not patch Trident operator image pull secret; %v", err)
	}
	log.Debug("Patched Trident image pull secret.")

	return nil
}

func (i *Installer) patchTridentDeployment(currentDeployment *appsv1.Deployment, newDeploymentYAML []byte) error {

	// Identify the deltas
	patchBytes, err := i.genericPatch(currentDeployment, newDeploymentYAML, &appsv1.Deployment{})
	if err != nil {
		return fmt.Errorf("error in creating the two-way merge patch for Trident operator Deployment %q: %v",
			currentDeployment.Name, err)
	}

	// Apply the patch to the current deployment
	err = i.clients.K8SClient.PatchDeploymentByLabel(TridentOperatorLabel, patchBytes)
	if err != nil {
		return fmt.Errorf("could not patch Trident operator deployment; %v", err)
	}
	log.Debug("Patched Trident operator deployment.")

	return nil
}

func (i *Installer) patchStorageClass(currentStorageClass *storagev1.StorageClass, newStorageClassYAML []byte) error {

	// Identify the deltas
	patchBytes, err := i.genericPatch(currentStorageClass, newStorageClassYAML, &storagev1.StorageClass{})
	if err != nil {
		return fmt.Errorf("error in creating the two-way merge patch for StorageClass %q: %v",
			currentStorageClass.Name, err)
	}

	// Apply the patch to the current storage class
	if _, err := i.clients.KubeClient.StorageV1().StorageClasses().Patch(
		ctx(), currentStorageClass.Name, commontypes.StrategicMergePatchType, patchBytes, patchOpts); err != nil {
		return fmt.Errorf("could not patch StorageClass; %v", err)
	}
	log.WithField("storageClass", currentStorageClass.Name).Debug("Patched StorageClass.")

	return nil
}

func (i *Installer) patchVolumeSnapshotClass(currentVSC *snapshotv1.VolumeSnapshotClass, newVSCYAML []byte) error {

	// Identify the deltas
	patchBytes, err := i.genericPatch(currentVSC, newVSCYAML, &snapshotv1.VolumeSnapshotClass{})
	if err != nil {
		return fmt.Errorf("error in creating the two-way merge patch for VolumeSnapshotClass %q: %v",
			currentVSC.Name, err)
	}

	// Apply the patch to the current volume snapshot class
	if _, err := i.clients.SnapClient.SnapshotV1().VolumeSnapshotClasses().Patch(
		ctx(), currentVSC.Name, commontypes.MergePatchType, patchBytes, patchOpts); err != nil {
		return fmt.Errorf("could not patch VolumeSnapshotClass; %v", err)
	}
	log.Debug("Patched VolumeSnapshotClass.")

	return nil
}

func (i *Installer) genericPatch(original interface{}, modifiedYAML []byte, _ interface{}) ([]byte, error) {

	// Get existing object in JSON format
	originalJSON, err := json.Marshal(original)
	if err != nil {
		return nil, fmt.Errorf("error in marshaling current object; %v", err)
	}

	// Convert new object from YAML to JSON format
	modifiedJSON, err := yaml.YAMLToJSON(modifiedYAML)
	if err != nil {
		return nil, fmt.Errorf("could not convert new object from YAML to JSON; %v", err)
	}

	// Identify the deltas
	return jsonpatch.MergePatch(originalJSON, modifiedJSON)

	// Old alternative:
	//return strategicpatch.CreateTwoWayMergePatch(originalJSON, modifiedJSON, dataStruct)
}
