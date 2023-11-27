// Copyright 2023 NetApp, Inc. All Rights Reserved.

package trident

import (
	"context"
	"errors"
	"fmt"
	tridentErrors "github.com/NetApp-Polaris/astra-connector-operator/app/trident/errors"
	"github.com/NetApp-Polaris/astra-connector-operator/app/trident/kubernetes"
	"github.com/NetApp-Polaris/astra-connector-operator/app/trident/model"
	tridentutils "github.com/netapp/trident/utils"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	torc "github.com/netapp/trident/operator/controllers/orchestrator/apis/netapp/v1"
	tridentversion "github.com/netapp/trident/utils/version"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	listOpts   = metav1.ListOptions{}
	getOpts    = metav1.GetOptions{}
	updateOpts = metav1.UpdateOptions{}
	patchOpts  = metav1.PatchOptions{}

	propagationPolicy = metav1.DeletePropagationBackground
	deleteOpts        = metav1.DeleteOptions{PropagationPolicy: &propagationPolicy}

	minOperatorVersion = tridentversion.MustParseDate(TridentOperatorMinVersion)

	ctx = context.TODO
)

const (
	TridentCSILabel     = "app=controller.csi.trident.netapp.io"
	TridentCSIContainer = "trident-main"
)

type Installer struct {
	controlPlaneClients     *kubernetes.Clients
	clients                 *kubernetes.Clients
	trident                 *model.TridentInstance
	operatorNamespace       string
	existingOperatorVersion *tridentversion.Version
	expectedOperatorVersion *tridentversion.Version
	timeout                 time.Duration
}

func NewInstaller(
	controlPlaneClients, clients *kubernetes.Clients, trident *model.TridentInstance,
) (*Installer, error) {

	var err error

	installer := &Installer{
		controlPlaneClients: controlPlaneClients,
		clients:             clients,
		trident:             trident,
		timeout:             K8sTimeout,
	}

	// Determine which images we are deploying
	if err = installer.getTridentImages(ctx()); err != nil {
		return nil, err
	}

	// Extract the version of the expected trident-operator
	installer.expectedOperatorVersion, err = getVersionFromImage(installer.trident.State.TridentOperatorImage)
	if err != nil {
		return nil, err
	}

	// Find the existing trident-operator, if any
	operatorInstalled, namespace, err := clients.K8SClient.CheckDeploymentExistsByLabel(TridentOperatorLabel, true)
	if err != nil {
		return nil, err
	} else if namespace == "<multiple>" {
		return nil, errors.New("trident-operator found in multiple namespaces")
	} else if operatorInstalled {
		installer.operatorNamespace = namespace
	} else {
		installer.operatorNamespace = TridentOperatorNamespace
	}

	// Limit all further installation to the desired namespace
	clients.K8SClient.SetNamespace(installer.operatorNamespace)

	// Extract the version of the existing trident-operator, if any
	if operatorInstalled {
		installer.existingOperatorVersion, err = installer.getTridentOperatorVersion()
		if err != nil {
			return nil, err
		}
	}

	// Check for unsupported versions
	if installer.expectedOperatorVersion.ToMajorMinorVersion().LessThan(minOperatorVersion) {
		return nil, fmt.Errorf("trident-operator version %s is too old", installer.expectedOperatorVersion.String())
	}

	if trident.Spec.GCP == nil && trident.Spec.ANF == nil {
		log.Debug("Installer configured with no cloud provider.")
	} else if trident.Spec.GCP != nil && trident.Spec.ANF != nil {
		return nil, errors.New("configuring multiple cloud providers is not supported")
	}

	log.WithFields(log.Fields{
		"expectedOperatorVersion": installer.expectedOperatorVersion,
		"existingOperatorVersion": installer.existingOperatorVersion,
	}).Debug("Initialized installer.")

	return installer, nil
}

// getTridentOperatorVersion finds the trident-operator pod and determines the version from the pod's image
func (i *Installer) getTridentOperatorVersion() (*tridentversion.Version, error) {

	pod, err := i.clients.K8SClient.GetPodByLabel(TridentOperatorLabel, false, v1.PodRunning)
	if err != nil {
		return nil, err
	}

	for _, container := range pod.Spec.Containers {
		if container.Name == TridentOperatorContainerName {
			return getVersionFromImage(container.Image)
		}
	}

	return nil, fmt.Errorf("cannot find %s container in trident-operator pod", TridentOperatorContainerName)
}

// getTridentImages determines the images to deploy based on the contents of the tridentInstance object an a configmap
func (i *Installer) getTridentImages(ctx context.Context) error {

	// Copy images to State (Spec takes precedence, then ConfigMap)

	if i.trident.Spec.TridentImage != "" {
		i.trident.State.TridentImage = i.trident.Spec.TridentImage
	} else {
		i.trident.State.TridentImage = imageMap[TridentImageKey]
	}

	if i.trident.Spec.TridentAutosupportImage != "" {
		i.trident.State.TridentAutosupportImage = i.trident.Spec.TridentAutosupportImage
	} else {
		i.trident.State.TridentAutosupportImage = imageMap[TridentAutosupportImageKey]
	}

	if i.trident.Spec.TridentOperatorImage != "" {
		i.trident.State.TridentOperatorImage = i.trident.Spec.TridentOperatorImage
	} else {
		i.trident.State.TridentOperatorImage = imageMap[TridentOperatorImageKey]

	}

	if i.trident.Spec.ACPImage != "" {
		i.trident.State.ACPImage = i.trident.Spec.ACPImage
	} else {
		i.trident.State.ACPImage = imageMap[ACPImageKey]
	}

	// Validate all four image names are known.
	if i.trident.State.TridentImage == "" {
		return errors.New("could not find Trident image in trident-svc configmap")
	}
	if i.trident.State.TridentAutosupportImage == "" {
		return errors.New("could not find Trident autosupport image in trident-svc configmap")
	}
	if i.trident.State.TridentOperatorImage == "" {
		return errors.New("could not find Trident operator image in trident-svc configmap")
	}
	if i.trident.State.ACPImage == "" {
		return errors.New("could not find ACP image in trident-svc configmap")
	}

	return nil
}

// InstallOrPatchTridentOperator patches all components of the trident-operator deployment, as well as the CR that
// instructs the operator to deploy Trident.  It then configures Trident in accordance with the tridentInstance object.
func (i *Installer) InstallOrPatchTridentOperator() error {

	// These operations are idempotent, so we can retry them if they fail.
	attempt := 0
	bo := backoff.NewExponentialBackOff()
	bo.MaxElapsedTime = K8sTimeout
	err := backoff.Retry(func() error {
		attempt++
		// All checks succeeded, so proceed with installation
		log.WithField("namespace", i.operatorNamespace).Infof("Starting Trident operator installation, attempt %d.", attempt)

		// Create namespace
		if err := i.createTridentOperatorNamespace(); err != nil {
			return err
		}

		// Create the RBAC objects
		if err := i.createTridentOperatorRBACObjects(); err != nil {
			return err
		}

		// Create the CRDs
		if err := i.createAndEnsureCRDs(); err != nil {
			return fmt.Errorf("could not create the Trident operator CRD; %v", err)
		}

		// Delete the old image pull secret if it exists
		if err := i.deleteLegacyTridentOperatorImagePullSecret(); err != nil {
			return fmt.Errorf("could not delete the old Trident operator image pull secret; %v", err)
		}

		// Create the image pull secret
		if err := i.createOrPatchTridentOperatorImagePullSecret(); err != nil {
			return fmt.Errorf("could not create the Trident operator image pull secret; %v", err)
		}

		// Create the operator deployment
		if err := i.createOrPatchTridentOperatorDeployment(); err != nil {
			return fmt.Errorf("could not create the Trident operator deployment; %v", err)
		}

		// Wait for operator to become available
		if err := i.waitForTridentOperatorPod(); err != nil {
			return fmt.Errorf("installation of the Trident operator failed or is not yet complete; %v", err)
		}

		// Remove any CRs in a terminal state
		if err := i.deleteTerminatedCRs(); err != nil {
			return fmt.Errorf("could not clean up Trident CRs; %v", err)
		}

		// Create the CR that will kick off the Trident installation
		if err := i.createOrPatchCR(); err != nil {
			return fmt.Errorf("could not create the Trident operator deployment; %v", err)
		}

		// Wait for the installation to complete
		if err := i.waitForTridentInstall(); err != nil {
			return fmt.Errorf("installation of Trident failed or is not yet complete; %v", err)
		}

		// Wait for Trident to be running and responding
		if err := i.waitForTridentControllerPod(); err != nil {
			return fmt.Errorf("installation of Trident failed or is not yet complete; %v", err)
		}

		// Do cloud-specific configuration
		if i.trident.Spec.GCP != nil {
			if err := i.configureTridentForGCP(); err != nil {
				return fmt.Errorf("could not configure Trident for GCP; %v", err)
			}
		}
		if i.trident.Spec.ANF != nil {
			// todo oscar uncomment below
			//if err := i.configureTridentForANF(); err != nil {
			//	return fmt.Errorf("could not configure Trident for ANF; %v", err)
			//}
		}
		return nil
	}, bo)

	return err
}

// createTridentOperatorNamespace creates the namespace for trident-operator if it does not exist
func (i *Installer) createTridentOperatorNamespace() error {

	if namespaceExists, err := i.clients.K8SClient.CheckNamespaceExists(i.operatorNamespace); err != nil {
		return fmt.Errorf("could not check for Trident operator namespace; %v", err)
	} else if namespaceExists {
		log.WithFields(log.Fields{"namespace": i.operatorNamespace}).Info("Trident operator namespace exists.")
		return nil
	}

	newNamespaceYAML := GetNamespaceYAML(i.operatorNamespace)

	if err := i.clients.K8SClient.CreateObjectByYAML(newNamespaceYAML); err != nil {
		return fmt.Errorf("could not create Trident operator namespace; %v", err)
	}

	log.WithFields(log.Fields{"namespace": i.operatorNamespace}).Info("Created Trident operator namespace.")
	return nil
}

// createTridentOperatorRBACObjects creates/patches the RBAC objects needed by trident-operator
func (i *Installer) createTridentOperatorRBACObjects() error {

	// Create service account
	if err := i.createOrPatchTridentOperatorServiceAccount(); err != nil {
		return err
	}

	// Create cluster role
	if err := i.createOrPatchTridentOperatorClusterRole(); err != nil {
		return err
	}

	// Create cluster role binding
	if err := i.createOrPatchTridentOperatorClusterRoleBinding(); err != nil {
		return err
	}

	// If OpenShift, add Trident to security context constraint(s)
	//if i.clients.K8SClient.Flavor() == clients.FlavorOpenShift {
	//	if returnError = i.clients.K8SClient.AddTridentUserToOpenShiftSCC("trident-csi", "privileged"); returnError != nil {
	//		returnError = fmt.Errorf("could not modify security context constraint; %v", returnError)
	//		return
	//	}
	//	log.WithFields(log.Fields{
	//		"scc":  "privileged",
	//		"user": "trident-csi",
	//	}).Info("Added security context constraint user.")
	//}

	return nil
}

// createOrPatchTridentOperatorServiceAccount creates/patches the ServiceAccount needed by trident-operator
func (i *Installer) createOrPatchTridentOperatorServiceAccount() error {

	createServiceAccount := true
	var currentServiceAccount *corev1.ServiceAccount

	if serviceAccount, err := i.clients.K8SClient.GetServiceAccountByLabel(TridentOperatorLabel, true); err != nil {
		if tridentErrors.IsNotFoundError(err) {
			log.Info("Trident operator service account not found.")
		} else {
			return err
		}
	} else {
		log.WithFields(log.Fields{
			"serviceAccount": serviceAccount.Name,
			"namespace":      serviceAccount.Namespace,
		}).Info("Trident operator service account found by label.")

		currentServiceAccount = serviceAccount
		createServiceAccount = false

		// Service account found by label, so ensure there isn't a namespace clash
		if i.operatorNamespace != serviceAccount.Namespace {
			log.Errorf("a Trident operator service account was found in namespace '%s', "+
				"not in specified namespace '%s', deleting this service account and creating a new one",
				serviceAccount.Namespace, i.operatorNamespace)

			currentServiceAccount = serviceAccount
			createServiceAccount = true

			// Delete the service account
			if err = i.clients.K8SClient.DeleteServiceAccountByLabel(TridentOperatorLabel); err != nil {
				log.WithFields(log.Fields{
					"serviceAccount": serviceAccount.Name,
					"namespace":      serviceAccount.Namespace,
					"label":          TridentOperatorLabel,
					"error":          err,
				}).Warn("Could not delete service account.")
				return fmt.Errorf("could not delete Trident operator service account in another namespace; %v", err)
			} else {
				log.Info("Deleted Trident operator service account.")
			}
		}
	}

	newServiceAccountYAML := GetServiceAccountYAML(TridentOperatorLabelValue, i.operatorNamespace)

	if createServiceAccount {
		if err := i.clients.K8SClient.CreateObjectByYAML(newServiceAccountYAML); err != nil {
			return fmt.Errorf("could not create operator service account; %v", err)
		}
		log.Info("Created operator service account.")
	} else if err := i.patchTridentServiceAccount(currentServiceAccount, []byte(newServiceAccountYAML)); err != nil {
		return err
	}

	return nil
}

// createOrPatchTridentOperatorClusterRole creates/patches the ClusterRole needed by trident-operator
func (i *Installer) createOrPatchTridentOperatorClusterRole() error {

	createClusterRole := true
	var currentClusterRole *rbacv1.ClusterRole

	if clusterRole, err := i.clients.K8SClient.GetClusterRoleByLabel(TridentOperatorLabel); err != nil {
		if tridentErrors.IsNotFoundError(err) {
			log.Info("Trident operator cluster role not found.")
		} else {
			return err
		}
	} else {
		log.WithFields(log.Fields{
			"clusterRole": clusterRole.Name,
		}).Info("Trident operator cluster role found by label.")

		currentClusterRole = clusterRole
		createClusterRole = false
	}

	newClusterRoleYAML := GetClusterRoleYAML(i.clients.K8SClient, TridentOperatorLabelValue)

	if createClusterRole {
		if err := i.clients.K8SClient.CreateObjectByYAML(newClusterRoleYAML); err != nil {
			return fmt.Errorf("could not create Trident operator cluster role; %v", err)
		}
		log.Info("Created Trident operator cluster role.")
	} else if err := i.patchTridentClusterRole(currentClusterRole, []byte(newClusterRoleYAML)); err != nil {
		return err
	}

	return nil
}

// createOrPatchTridentOperatorClusterRoleBinding creates/patches the ClusterRoleBinding needed by trident-operator
func (i *Installer) createOrPatchTridentOperatorClusterRoleBinding() error {

	createClusterRoleBinding := true
	var currentClusterRoleBinding *rbacv1.ClusterRoleBinding

	if clusterRoleBinding, err := i.clients.K8SClient.GetClusterRoleBindingByLabel(TridentOperatorLabel); err != nil {
		if tridentErrors.IsNotFoundError(err) {
			log.Info("Trident operator cluster role binding not found.")
		} else {
			return err
		}
	} else {
		log.WithFields(log.Fields{
			"clusterRoleBinding": clusterRoleBinding.Name,
		}).Info("Trident operator cluster role binding found by label.")

		currentClusterRoleBinding = clusterRoleBinding
		createClusterRoleBinding = false
	}

	newClusterRoleBindingYAML := GetClusterRoleBindingYAML(i.clients.K8SClient.Flavor(),
		TridentOperatorLabelValue, i.operatorNamespace)

	if createClusterRoleBinding {
		if err := i.clients.K8SClient.CreateObjectByYAML(newClusterRoleBindingYAML); err != nil {
			return fmt.Errorf("could not create Trident operator cluster role binding; %v", err)
		}
		log.Info("Created Trident operator cluster role binding.")
	} else if err := i.patchTridentClusterRoleBinding(currentClusterRoleBinding, []byte(newClusterRoleBindingYAML)); err != nil {
		return err
	}

	return nil
}

// createAndEnsureCRDs ensures that the Torc CRD needed by trident-operator exists and is fully established
func (i *Installer) createAndEnsureCRDs() (returnError error) {
	return i.createCRD(TridentOrchestratorCRDName, GetTridentOrchestratorCRDYAML())
}

// createCRD creates a CRD if it does not exist and waits for it to be established.  This method is CRD independent.
func (i *Installer) createCRD(crdName, crdYAML string) error {

	// Discover CRD data
	crdExist, err := i.clients.K8SClient.CheckCRDExists(crdName)
	if err != nil {
		return fmt.Errorf("unable to identify if %v CRD exists; %v", crdName, err)
	}

	if crdExist {
		log.Infof("Trident operator %v CRD present.", crdName)
	} else {
		// Create the CRDs and wait for them to be registered in Kubernetes
		log.Infof("Installer will create a fresh %v CRD.", crdName)

		if err := i.clients.K8SClient.CreateObjectByYAML(crdYAML); err != nil {
			return fmt.Errorf("could not create custom resource %v in %s; %v", crdName, i.operatorNamespace, err)
		}

		log.WithFields(log.Fields{
			"namespace": i.operatorNamespace,
		}).Infof("Created custom resource definition %v.", crdName)
	}

	// Wait for the CRD to be fully established
	if err := i.ensureCRDEstablished(crdName); err != nil {
		return fmt.Errorf("CRDs not established; %v", err)
	}

	return nil
}

// ensureCRDEstablished waits until a CRD is Established.
func (i *Installer) ensureCRDEstablished(crdName string) error {

	checkCRDEstablished := func() error {
		crd, err := i.clients.K8SClient.GetCRD(crdName)
		if err != nil {
			return err
		}
		for _, condition := range crd.Status.Conditions {
			if condition.Type == apiextensionsv1.Established {
				switch condition.Status {
				case apiextensionsv1.ConditionTrue:
					return nil
				default:
					return fmt.Errorf("CRD %s Established condition is %s", crdName, condition.Status)
				}
			}
		}
		return fmt.Errorf("CRD %s Established condition is not yet available", crdName)
	}

	checkCRDNotify := func(err error, duration time.Duration) {
		log.WithFields(log.Fields{
			"CRD":   crdName,
			"error": err,
		}).Debug("CRD not yet established, waiting.")
	}

	checkCRDBackoff := backoff.NewExponentialBackOff()
	checkCRDBackoff.InitialInterval = 5 * time.Second
	checkCRDBackoff.RandomizationFactor = 0.1
	checkCRDBackoff.Multiplier = 1.414
	checkCRDBackoff.MaxInterval = 15 * time.Second
	checkCRDBackoff.MaxElapsedTime = i.timeout

	log.WithField("CRD", crdName).Debug("Waiting for CRD to be established.") // Trace

	if err := backoff.RetryNotify(checkCRDEstablished, checkCRDBackoff, checkCRDNotify); err != nil {
		return fmt.Errorf("CRD was not established after %3.2f seconds", i.timeout.Seconds())
	}

	log.WithField("CRD", crdName).Debug("CRD established.")
	return nil
}

// deleteLegacyTridentOperatorImagePullSecret removes the old image pull Secret needed by trident-operator, if it exists
func (i *Installer) deleteLegacyTridentOperatorImagePullSecret() error {

	secrets, err := i.clients.K8SClient.GetSecretsByLabel(TridentOperatorLabel, false)
	if err != nil {
		return err
	}

	// Delete any secrets that match our app label but have the wrong name.

	for _, secret := range secrets {
		if secret.Name != TridentOperatorImagePullSecretName {
			log.WithField("name", secret.Name).Debug("Deleting Trident operator image pull secret.")
			if deleteErr := i.clients.K8SClient.DeleteSecret(secret.Name); deleteErr != nil {
				log.WithField("name", secret.Name).WithError(deleteErr).Error(
					"Could not delete legacy Trident operator image pull secret.")
			} else {
				log.WithField("name", secret.Name).WithError(deleteErr).Debug(
					"Deleted legacy Trident operator image pull secret.")
			}
		}
	}

	return nil
}

// createOrPatchTridentOperatorImagePullSecret creates/patches the image pull Secret needed by trident-operator
func (i *Installer) createOrPatchTridentOperatorImagePullSecret() error {

	createSecret := true
	var currentSecret *v1.Secret

	if secret, err := i.clients.K8SClient.GetSecretByLabel(TridentOperatorLabel, false); err != nil {
		if tridentErrors.IsNotFoundError(err) {
			log.Info("Trident operator image pull secret not found.")
		} else {
			return err
		}
	} else {
		log.WithFields(log.Fields{
			"name":      secret.Name,
			"namespace": secret.Namespace,
		}).Info("Trident operator image pull secret found by label.")

		currentSecret = secret
		createSecret = false

		// Secret found by label, so ensure there isn't a namespace clash
		if i.operatorNamespace != secret.Namespace {
			log.Errorf("a Trident operator image pull secret was found in namespace '%s', "+
				"not in specified namespace '%s', deleting this secret and creating a new one.",
				secret.Namespace, i.operatorNamespace)

			createSecret = true

			// Delete the secret
			if err = i.clients.K8SClient.DeleteSecretByLabel(TridentOperatorLabel); err != nil {
				log.WithFields(log.Fields{
					"secret":    secret.Name,
					"namespace": secret.Namespace,
					"label":     TridentOperatorLabel,
					"error":     err,
				}).Warn("Could not delete Trident operator image pull secret.")
				return fmt.Errorf("could not delete Trident operator image pull secret in another namespace; %v", err)
			} else {
				log.Info("Deleted Trident operator image pull secret.")
			}
		}
	}

	newSecretYAML := GetImagePullSecretYAML(
		TridentOperatorImagePullSecretName, i.operatorNamespace, TridentOperatorLabelValue)

	if createSecret {
		// Create the secret
		if err := i.clients.K8SClient.CreateObjectByYAML(newSecretYAML); err != nil {
			return fmt.Errorf("could not create Trident operator image pull secret; %v", err)
		}
		log.Info("Created Trident operator image pull secret.")
	} else if err := i.patchTridentImagePullSecret(currentSecret, []byte(newSecretYAML)); err != nil {
		return err
	}

	return nil
}

// createOrPatchTridentOperatorDeployment creates/patches the Deployment of trident-operator
func (i *Installer) createOrPatchTridentOperatorDeployment() error {

	createDeployment := true
	var currentDeployment *appsv1.Deployment

	if deployment, err := i.clients.K8SClient.GetDeploymentByLabel(TridentOperatorLabel, true); err != nil {
		if tridentErrors.IsNotFoundError(err) {
			log.Info("Trident operator deployment not found.")
		} else {
			return err
		}
	} else {
		log.WithFields(log.Fields{
			"deployment": deployment.Name,
			"namespace":  deployment.Namespace,
		}).Info("Trident operator deployment found by label.")

		currentDeployment = deployment
		createDeployment = false

		// Deployment found by label, so ensure there isn't a namespace clash
		if i.operatorNamespace != deployment.Namespace {
			log.Errorf("a Trident operator deployment was found in namespace '%s', "+
				"not in specified namespace '%s', deleting this deployment and creating a new one",
				deployment.Namespace, i.operatorNamespace)

			createDeployment = true

			// Delete the deployment
			if err = i.clients.K8SClient.DeleteDeploymentByLabel(TridentOperatorLabel); err != nil {
				log.WithFields(log.Fields{
					"deployment": deployment.Name,
					"namespace":  deployment.Namespace,
					"label":      TridentOperatorLabel,
					"error":      err,
				}).Warn("Could not delete Trident operator deployment.")
				return fmt.Errorf("could not delete Trident operator deployment in another namespace; %v", err)
			} else {
				log.Info("Deleted Trident operator deployment.")
			}
		}
	}

	newDeploymentYAML := GetDeploymentYAML(
		i.operatorNamespace, i.trident.State.TridentOperatorImage, TridentOperatorLabelValue, "text", true)

	if createDeployment {
		// Create the deployment
		if err := i.clients.K8SClient.CreateObjectByYAML(newDeploymentYAML); err != nil {
			return fmt.Errorf("could not create Trident operator deployment; %v", err)
		}
		log.Info("Created Trident operator deployment.")
	} else if err := i.patchTridentDeployment(currentDeployment, []byte(newDeploymentYAML)); err != nil {
		return err
	}

	return nil
}

// waitForTridentOperatorPod waits for the trident-operator Pod to be created and the container Ready & Running
func (i *Installer) waitForTridentOperatorPod() error {

	var deployment *appsv1.Deployment
	var err error

	checkOperatorInstalled := func() error {

		deployment, err = i.clients.K8SClient.GetDeploymentByLabel(TridentOperatorLabel, false)
		if err != nil {
			return err
		}

		// If this is an upgrade, there will be multiple pods for a brief time.  Calling GetPodByLabel
		// ensures only one is running.
		pod, err := i.clients.K8SClient.GetPodByLabel(TridentOperatorLabel, false, v1.PodRunning)
		if err != nil {
			return err
		}

		// Ensure the pod hasn't been deleted.
		if pod.DeletionTimestamp != nil {
			return errors.New("trident-operator pod is terminating")
		}

		// Ensure the pod spec contains the correct image.  The running container may report a different
		// image name if there are multiple tags for the same image hash, but the pod spec should be correct.
		for _, container := range pod.Spec.Containers {
			if container.Name == TridentOperatorContainerName {
				if container.Image != i.trident.State.TridentOperatorImage {
					return fmt.Errorf("operator pod spec reports a different image (%s) than the tridentInstance "+
						"requires (%s)", container.Image, i.trident.State.TridentOperatorImage)
				}
			}
		}

		// Ensure the only running container is the correct image.
		for _, container := range pod.Status.ContainerStatuses {
			if container.Name == TridentOperatorContainerName {
				if container.State.Running == nil {
					return errors.New("operator container is not running")
				} else if !container.Ready {
					return errors.New("operator container is not ready")
				}
				return nil
			}
		}

		return fmt.Errorf("container %s not found in operator deployment", TridentOperatorContainerName)
	}

	checkOperatorNotify := func(err error, duration time.Duration) {
		log.WithFields(log.Fields{
			"error": err,
		}).Debug("Trident operator has not yet reached a terminal or installed state, waiting.")
	}

	checkOperatorBackoff := backoff.NewExponentialBackOff()
	checkOperatorBackoff.InitialInterval = 5 * time.Second
	checkOperatorBackoff.RandomizationFactor = 0.1
	checkOperatorBackoff.Multiplier = 1.414
	checkOperatorBackoff.MaxInterval = 15 * time.Second
	checkOperatorBackoff.MaxElapsedTime = i.timeout

	log.Debug("Waiting for operator to reach a terminal or installed state.") // Trace

	if err := backoff.RetryNotify(checkOperatorInstalled, checkOperatorBackoff, checkOperatorNotify); err != nil {
		return fmt.Errorf("operator failed to reach an installed state within %3.2f seconds; %v", i.timeout.Seconds(), err)
	}

	log.WithFields(log.Fields{
		"name":              deployment.Name,
		"namespace":         deployment.Namespace,
		"availableReplicas": deployment.Status.AvailableReplicas,
	}).Info("Trident operator is running.")

	return nil
}

// Get all tridentorchestrator CRs
func (i *Installer) getTorcCRs() (*torc.TridentOrchestratorList, error) {
	return i.clients.TorcClient.TridentV1().TridentOrchestrators().List(ctx(), listOpts)
}

// GetTorcCR returns what should be exactly one Torc CR, else return an error
func (i *Installer) GetTorcCR() (*torc.TridentOrchestrator, error) {

	crList, err := i.getTorcCRs()
	if err != nil {
		return nil, err
	}

	switch len(crList.Items) {
	case 0:
		return nil, tridentErrors.NotFoundErr("no Torc CRs found, expected one")
	case 1:
		return &crList.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple Torc CRs found, expected one")
	}
}

// deleteErroredCRs deletes all Torc CRs in error state.
func (i *Installer) deleteErroredCRs() error {

	allCRs, err := i.getTorcCRs()
	if err != nil {
		return err
	}

	otherCRs := make([]torc.TridentOrchestrator, 0)
	errorCRs := make([]torc.TridentOrchestrator, 0)

	for _, cr := range allCRs.Items {
		if cr.Status.Status == AppStatusError {
			err = i.clients.TorcClient.TridentV1().TridentOrchestrators().Delete(ctx(), cr.Name, deleteOpts)
			if err != nil {
				errorCRs = append(errorCRs, cr)
				log.Error(err)
			}
		} else {
			otherCRs = append(otherCRs, cr)
		}
	}

	if len(otherCRs) > 1 {
		return errors.New("multiple non-errored Torc CRs detected, waiting for operator to fix them")
	} else if len(errorCRs) > 0 {
		return errors.New("failed to delete one or more CRs")
	}

	return nil
}

// deleteTerminatedCRs deletes all Torc CRs in a terminal state.
func (i *Installer) deleteTerminatedCRs() error {

	allCRs, err := i.getTorcCRs()
	if err != nil {
		return err
	}

	otherCRs := make([]torc.TridentOrchestrator, 0)
	errorCRs := make([]torc.TridentOrchestrator, 0)

	for _, cr := range allCRs.Items {

		switch cr.Status.Status {
		case AppStatusUninstalled, AppStatusUninstalledAll, AppStatusFailed, AppStatusError:
			err = i.clients.TorcClient.TridentV1().TridentOrchestrators().Delete(ctx(), cr.Name, deleteOpts)
			if err != nil {
				errorCRs = append(errorCRs, cr)
				log.Error(err)
			}
		default:
			otherCRs = append(otherCRs, cr)
		}
	}

	if len(otherCRs) > 1 {
		return errors.New("multiple non-terminated Torc CRs detected, waiting for operator to fix them")
	} else if len(errorCRs) > 0 {
		return errors.New("failed to delete one or more CRs")
	}

	return nil
}

// createOrPatchCR creates/patches the Torc CR that trident-operator needs to install Trident.
func (i *Installer) createOrPatchCR() error {

	createCR := true
	var currentCR *torc.TridentOrchestrator

	crList, err := i.getTorcCRs()
	if err != nil {
		return err
	}

	switch len(crList.Items) {
	case 0:
		log.Info("Torc CR not found.")

	case 1:
		currentCR = &crList.Items[0]
		createCR = false

		log.WithFields(log.Fields{"name": currentCR.Name}).Info("Torc CR found.")

	default:
		return errors.New("multiple Torc CRs found")
	}

	if createCR {

		// Create the CR
		newCRYAML := GetTridentOrchestratorCRYAML(
			true, false, false, i.operatorNamespace, i.trident.Spec.SerialNumber,
			i.trident.Spec.ClusterName, "text", i.trident.State.TridentImage,
			i.trident.State.TridentAutosupportImage, i.trident.Spec.ProxyURL, "", i.trident.State.ACPImage)

		if err := i.clients.K8SClient.CreateObjectByYAML(newCRYAML); err != nil {
			return fmt.Errorf("could not create Torc CR; %v", err)
		}
		log.WithField("yaml", newCRYAML).Info("Created Torc CR.")

	} else if currentCR != nil {

		// Patch the CR, restoring everything we don't want folks editing
		currentCR.Spec.Debug = true
		currentCR.Spec.EnableACP = true
		currentCR.Spec.AutosupportImage = i.trident.State.TridentAutosupportImage
		currentCR.Spec.AutosupportProxy = i.trident.Spec.ProxyURL
		currentCR.Spec.AutosupportSerialNumber = i.trident.Spec.SerialNumber
		currentCR.Spec.AutosupportHostname = i.trident.Spec.ClusterName
		currentCR.Spec.TridentImage = i.trident.State.TridentImage
		currentCR.Spec.ACPImage = i.trident.State.ACPImage
		currentCR.Spec.ImagePullSecrets = []string{TridentOperatorImagePullSecretName}
		currentCR.Spec.Uninstall = false
		currentCR.Spec.Wipeout = nil

		_, err := i.clients.TorcClient.TridentV1().TridentOrchestrators().Update(ctx(), currentCR, updateOpts)
		if err != nil {
			return err
		}

		log.WithFields(log.Fields{
			"name":                    currentCR.Name,
			"namespace":               currentCR.Spec.Namespace,
			"debug":                   currentCR.Spec.Debug,
			"enableACP":               currentCR.Spec.EnableACP,
			"acpImage":                currentCR.Spec.ACPImage,
			"ipv6":                    currentCR.Spec.IPv6,
			"windows":                 currentCR.Spec.Windows,
			"logFormat":               currentCR.Spec.LogFormat,
			"autosupportImage":        currentCR.Spec.AutosupportImage,
			"autosupportProxy":        currentCR.Spec.AutosupportProxy,
			"autosupportSerialNumber": currentCR.Spec.AutosupportSerialNumber,
			"autosupportHostname":     currentCR.Spec.AutosupportHostname,
			"tridentImage":            currentCR.Spec.TridentImage,
			"imageRegistry":           currentCR.Spec.ImageRegistry,
			"imagePullSecrets":        currentCR.Spec.ImagePullSecrets,
			"uninstall":               currentCR.Spec.Uninstall,
			"wipeout":                 currentCR.Spec.Wipeout,
		}).Debug("Patched Trident CR.")
	}

	return nil
}

// waitForTridentInstall monitors the Torc CR to know when trident-operator has finished installing Trident.
func (i *Installer) waitForTridentInstall() error {

	checkTridentInstalled := func() error {

		cr, err := i.GetTorcCR()
		if tridentErrors.IsNotFoundError(err) {
			log.Debug("Torc CR not found.")
			return nil
		} else if err != nil {
			return err
		}

		log.WithField("state", cr.Status.Status).Debug("Torc CR state.")

		switch cr.Status.Status {
		case AppStatusInstalled:
			if cr.Status.CurrentInstallationParams.TridentImage != cr.Spec.TridentImage {
				return fmt.Errorf("tridentorchestrator CR status reports a different Trident image (%s) "+
					"than the spec requires (%s); %s; %s", cr.Status.CurrentInstallationParams.TridentImage,
					cr.Spec.TridentImage, cr.Status.Status, cr.Status.Message)
			}
			return nil
		case AppStatusNoStatus:
			return fmt.Errorf("tridentorchestrator CR has not yet reported any status")
		case AppStatusInstalling, AppStatusUninstalling, AppStatusUpdating:
			return fmt.Errorf("tridentorchestrator CR is in a transitional state (%s); %s",
				cr.Status.Status, cr.Status.Message)
		case AppStatusUninstalled, AppStatusUninstalledAll, AppStatusError:
			// Trident isn't installed, or Torc CR is in a terminal state, stop waiting
			err := fmt.Errorf("tridentorchestrator CR reached a terminal state (%s); %s",
				cr.Status.Status, cr.Status.Message)
			return backoff.Permanent(err)
		case AppStatusFailed:
			// Torc CR is in a Failed state, which is usually recoverable, so don't stop waiting
			return fmt.Errorf("tridentorchestrator CR reached a non-terminal failed state (%s); %s",
				cr.Status.Status, cr.Status.Message)
		default:
			// Torc CR is in an Unknown state (shouldn't happen)
			return fmt.Errorf("tridentorchestrator CR reached an unknown state (%s); %s",
				cr.Status.Status, cr.Status.Message)
		}
	}

	checkCRNotify := func(err error, duration time.Duration) {
		log.WithFields(log.Fields{
			"error": err,
		}).Debug("tridentorchestrator CR has not yet reached a terminal or installed state, waiting.")
	}

	checkCRBackoff := backoff.NewExponentialBackOff()
	checkCRBackoff.InitialInterval = 5 * time.Second
	checkCRBackoff.RandomizationFactor = 0.1
	checkCRBackoff.Multiplier = 1.414
	checkCRBackoff.MaxInterval = 15 * time.Second
	checkCRBackoff.MaxElapsedTime = i.timeout

	log.Debug("Waiting for Torc CR to reach a terminal or installed state.") // Trace

	if err := backoff.RetryNotify(checkTridentInstalled, checkCRBackoff, checkCRNotify); err != nil {
		return fmt.Errorf("CR failed to reach an installed state within %3.2f seconds; %v", i.timeout.Seconds(), err)
	}

	return nil
}

// waitForTridentControllerPod ensures there is only one Trident pod, that it hasn't been deleted, that
// the Trident controller container is running/ready and has the expected version, and that Trident's
// REST interface is available.  This method is intended to provide resiliency in the face of a Trident
// pod that has restarted for any reason, such as a change to the tridentorchestrator CR, etc.
func (i *Installer) waitForTridentControllerPod() error {

	var deployment *appsv1.Deployment
	var err error

	checkTridentInstalled := func() error {

		deployment, err = i.clients.K8SClient.GetDeploymentByLabel(TridentCSILabel, false)
		if err != nil {
			return err
		}

		// If this is an upgrade, there will be multiple pods for a brief time.  Calling GetPodByLabel
		// ensures only one is running.
		pod, err := i.clients.K8SClient.GetPodByLabel(TridentCSILabel, false, v1.PodRunning)
		if err != nil {
			return err
		}

		// Ensure the pod hasn't been deleted.
		if pod.DeletionTimestamp != nil {
			return errors.New("trident pod is terminating")
		}

		// Ensure the pod spec contains the correct image.  The running container may report a different
		// image name if there are multiple tags for the same image hash, but the pod spec should be correct.
		for _, container := range pod.Spec.Containers {
			if container.Name == TridentCSIContainer {
				if container.Image != i.trident.State.TridentImage {
					return fmt.Errorf("trident pod spec reports a different image (%s) than the tridentInstance "+
						"requires (%s)", container.Image, i.trident.State.TridentImage)
				}
			}
		}

		// Ensure the Trident controller container is the correct image.
		tridentContainerOK := false
		for _, container := range pod.Status.ContainerStatuses {
			if container.Name == TridentCSIContainer {
				if container.State.Running == nil {
					return errors.New("trident container is not running")
				} else if !container.Ready {
					return errors.New("trident container is not ready")
				}
				tridentContainerOK = true
			}
		}

		if !tridentContainerOK {
			return fmt.Errorf("container %s not found in trident deployment", TridentCSIContainer)
		}

		// Ensure Trident is responding to REST calls
		return i.testTridentRESTInterface(pod)
	}

	checkTridentNotify := func(err error, duration time.Duration) {
		log.WithFields(log.Fields{
			"error": err,
		}).Debug("Trident has not yet reached a terminal or installed state, waiting.")
	}

	checkTridentBackoff := backoff.NewExponentialBackOff()
	checkTridentBackoff.InitialInterval = 5 * time.Second
	checkTridentBackoff.RandomizationFactor = 0.1
	checkTridentBackoff.Multiplier = 1.414
	checkTridentBackoff.MaxInterval = 15 * time.Second
	checkTridentBackoff.MaxElapsedTime = i.timeout

	log.Debug("Waiting for Trident controller to reach a terminal or installed state.")

	if err := backoff.RetryNotify(checkTridentInstalled, checkTridentBackoff, checkTridentNotify); err != nil {
		return fmt.Errorf("trident failed to reach an installed state within %3.2f seconds; %v", i.timeout.Seconds(), err)
	}

	log.WithFields(log.Fields{
		"name":              deployment.Name,
		"namespace":         deployment.Namespace,
		"availableReplicas": deployment.Status.AvailableReplicas,
	}).Info("Trident is running.")

	return nil
}

// findTridentPodAndContainer returns the Pod that contains the installed instance of Trident.  This is used
// while configuring Trident, so previous installation steps should have already been run to ensure this is
// the *correct* Pod and not one that is being replaced.
func (i *Installer) findTridentPodAndContainer() (*v1.Pod, error) {

	if pod, err := i.clients.K8SClient.GetPodByLabel(TridentCSILabel, true, v1.PodRunning); err != nil {
		log.Info("Trident controller pod not found.")
		return nil, err
	} else {
		return pod, nil
	}
}

// testTridentRESTInterface exec's "tridentctl version" inside the Trident Pod to ensure Trident is responding.
func (i *Installer) testTridentRESTInterface(tridentPod *v1.Pod) error {

	commandArgs := []string{"tridentctl", "version"}
	response, err := i.clients.K8SClient.Exec(tridentPod.Name, TridentCSIContainer, tridentPod.Namespace, commandArgs)
	if err != nil {
		log.Error("Trident REST interface not available.")
		return err
	} else {
		log.WithField("response", string(response)).Debug("Trident REST interface available.")
		return nil
	}
}

func getVersionFromImage(imageName string) (*tridentversion.Version, error) {

	// Remove the domain, since it may contain a colon
	_, remainder := tridentutils.SplitImageDomain(imageName)

	if !strings.Contains(remainder, ":") {
		return nil, fmt.Errorf("cannot get version from image %s", imageName)
	}
	return tridentversion.ParseDate(strings.Split(remainder, ":")[1])
}
