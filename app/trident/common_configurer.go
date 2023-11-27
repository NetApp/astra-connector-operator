package trident

import (
	"fmt"
	"github.com/NetApp-Polaris/astra-connector-operator/app/trident/kubernetes"
	log "github.com/sirupsen/logrus"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AnnDefaultStorageClass     = "storageclass.kubernetes.io/is-default-class"
	AnnDefaultStorageClassBeta = "storageclass.beta.kubernetes.io/is-default-class"
)

func (i *Installer) createOrPatchCVSStorageClass(name, scYAML string) error {

	createStorageClass := true
	var currentStorageClass *storagev1.StorageClass

	if storageClass, err := i.clients.KubeClient.StorageV1().StorageClasses().Get(ctx(), name, getOpts); err != nil {
		if statusErr, ok := err.(*apierrors.StatusError); ok && statusErr.Status().Reason == metav1.StatusReasonNotFound {
			log.WithField("name", name).Info("StorageClass not found.")
		} else {
			return err
		}
	} else {
		log.WithFields(log.Fields{
			"storageClass": storageClass.Name,
		}).Info("StorageClass found by name.")

		currentStorageClass = storageClass
		createStorageClass = false
	}

	// If the storage class exists, try to patch it.  If that fails, then delete it and fall
	// through to the create block.
	if !createStorageClass {

		if err := i.patchStorageClass(currentStorageClass, []byte(scYAML)); err == nil {
			return nil
		} else {
			log.WithFields(log.Fields{
				"storageClass": name,
				"error":        err,
			}).Error("Could not patch storage class, will delete/recreate.")
		}

		if err := i.clients.KubeClient.StorageV1().StorageClasses().Delete(ctx(), name, deleteOpts); err != nil {
			return fmt.Errorf("could not delete storage class %s; %v", name, err)
		} else {
			log.WithField("storageClass", name).Info("Deleted storage class.")
		}
	}

	if err := i.clients.K8SClient.CreateObjectByYAML(scYAML); err != nil {
		return fmt.Errorf("could not create storage class %s; %v", name, err)
	}

	log.WithFields(log.Fields{
		"storageClass": name,
	}).Info("Created storage class.")

	return nil
}

// getDefaultStorageClassName determines what the default storage class should be.  If a non-empty value is passed
// in, that is always the value returned.  Otherwise, the storage classes are read from the cluster and the first
// one with the default annotation set to true is returned.  If there is no default, an empty string is returned.
func (i *Installer) getDefaultStorageClassName(explicitDefaultStorageClassName string) (string, error) {

	if explicitDefaultStorageClassName != "" {
		return explicitDefaultStorageClassName, nil
	}

	storageClassList, err := i.clients.KubeClient.StorageV1().StorageClasses().List(ctx(), listOpts)
	if err != nil {
		return "", err
	}

	// To simplify this code, just get rid of any beta storage class default annotations now
	for _, storageClass := range storageClassList.Items {
		if storageClass.Annotations[AnnDefaultStorageClass] == "true" {
			return storageClass.Name, nil
		}
	}

	return "", nil
}

func updateStorageClassDefaultAnnotations(
	clients *kubernetes.Clients, defaultStorageClass string, skipList []string,
) error {

	// Bail out if there is no specified default storage class
	if defaultStorageClass == "" {
		log.Info("No default storage class set, not updating any default storage class indicators.")
		return nil
	}

	log.Debug("Checking for other default storage classes.")

	storageClassList, err := clients.KubeClient.StorageV1().StorageClasses().List(ctx(), listOpts)
	if err != nil {
		return err
	}

	anyChanged := false

	// To simplify this code, just get rid of any beta storage class default annotations now
	for _, storageClass := range storageClassList.Items {

		log.WithField("name", storageClass.Name).Debug("Checking for beta default storage class.")

		// Skip the CVS storage classes we have already handled during the create/patch process
		if SliceContainsString(skipList, storageClass.Name) {
			log.WithField("name", storageClass.Name).Debug("Ignoring one of our own storage classes.")
			continue
		}

		if _, ok := storageClass.Annotations[AnnDefaultStorageClassBeta]; ok {
			delete(storageClass.Annotations, AnnDefaultStorageClassBeta)
			if _, err := clients.KubeClient.StorageV1().StorageClasses().Update(ctx(), &storageClass, updateOpts); err != nil {
				log.WithField("error", err).Error("Could not remove beta default storage class annotation.")
				continue
			} else {
				anyChanged = true
				log.WithField("name", storageClass.Name).Debug("Removed beta default annotation from storage class.")
			}
		}
	}

	// If we changed any storage classes, we must re-retrieve the list since we can't modify a previous object version
	if anyChanged {
		if storageClassList, err = clients.KubeClient.StorageV1().StorageClasses().List(ctx(), listOpts); err != nil {
			return err
		}
	}

	// Now that there are no beta annotations, go through again and set/clear default annotations as needed
	for _, storageClass := range storageClassList.Items {

		log.WithField("name", storageClass.Name).Debug("Checking for default storage class.")

		// Skip the CVS storage classes we have already handled during the create/patch process
		if SliceContainsString(skipList, storageClass.Name) {
			log.WithField("name", storageClass.Name).Debug("Ignoring one of our own CVS storage classes.")
			continue
		}

		if storageClass.Name == defaultStorageClass {

			// Ensure map isn't nil just in case we need to modify it
			if storageClass.Annotations == nil {
				storageClass.Annotations = make(map[string]string)
			}

			// Add default StorageClass annotation if not present and true
			if storageClass.Annotations[AnnDefaultStorageClass] != "true" {
				storageClass.Annotations[AnnDefaultStorageClass] = "true"
				if _, err := clients.KubeClient.StorageV1().StorageClasses().Update(ctx(), &storageClass, updateOpts); err != nil {
					log.WithField("error", err).Error("Could not add default annotation to storage class.")
				} else {
					log.WithField("name", storageClass.Name).Debug("Added default annotation to storage class.")
				}
			} else {
				log.WithField("name", storageClass.Name).Debug("No default annotation change needed for storage class.")
			}
		} else {
			// Remove default StorageClass annotation if present & true
			if storageClass.Annotations[AnnDefaultStorageClass] == "true" {
				storageClass.Annotations[AnnDefaultStorageClass] = "false"
				if _, err := clients.KubeClient.StorageV1().StorageClasses().Update(ctx(), &storageClass, updateOpts); err != nil {
					log.WithField("error", err).Error("Could not remove default annotation from storage class.")
				} else {
					log.WithField("name", storageClass.Name).Debug("Removed default annotation from storage class.")
				}
			} else {
				log.WithField("name", storageClass.Name).Debug("No default annotation change needed for storage class.")
			}
		}
	}

	return nil
}

func (i *Installer) createOrPatchVolumeSnapshotClass(vscName string) error {

	createVSC := true
	var currentVSC *snapshotv1.VolumeSnapshotClass

	if vsc, err := i.clients.SnapClient.SnapshotV1().VolumeSnapshotClasses().Get(ctx(), vscName, getOpts); err != nil {
		if statusErr, ok := err.(*apierrors.StatusError); ok && statusErr.Status().Reason == metav1.StatusReasonNotFound {
			log.WithField("name", vscName).Info("VolumeSnapshotClass not found.")
		} else {
			return err
		}
	} else {
		log.WithFields(log.Fields{
			"volumeSnapshotClass": vsc.Name,
		}).Info("VolumeSnapshotClass found by name.")

		currentVSC = vsc
		createVSC = false
	}

	vscYAML := GetVolumeSnapshotClassYAML(vscName)

	if createVSC {
		if err := i.clients.K8SClient.CreateObjectByYAML(vscYAML); err != nil {
			return fmt.Errorf("could not create volume snapshot class, check if volume snapshot CRDs exist; %v", err)
		}
		log.WithField("name", vscName).Info("Created volume snapshot class.")
	} else if err := i.patchVolumeSnapshotClass(currentVSC, []byte(vscYAML)); err != nil {
		return err
	}

	return nil
}

// SliceContainsString checks to see if a []string contains a string
func SliceContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
