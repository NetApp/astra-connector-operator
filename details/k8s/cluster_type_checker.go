package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	FlavorAKS        = "aks"
	FlavorGKE        = "gke"
	FlavorKubernetes = "k8s"
	FlavorOpenShift  = "openshift"
	FlavorRKE        = "rke"
	FlavorRKE2       = "rke2"
	FlavorTanzu      = "tanzu"
	FlavorAnthos     = "anthos"

	openShiftApiServerName = "openshift-apiserver"
)

type ClusterTypeCheckerInterface interface {
	DetermineClusterType() string
}

type ClusterTypeChecker struct {
	K8sUtil K8sUtilInterface
	Log     logr.Logger
}

func NewClusterTypeChecker(k8sUtil K8sUtilInterface, log logr.Logger) ClusterTypeCheckerInterface {
	return &ClusterTypeChecker{K8sUtil: k8sUtil, Log: log}
}

func (c *ClusterTypeChecker) DetermineClusterType() string {
	if c.isAnthosFlavor() {
		return FlavorAnthos
	}

	if c.isOpenshiftFlavor() {
		return FlavorOpenShift
	}

	if c.isRKEFlavor() {
		return FlavorRKE
	}

	if c.isRKE2Flavor() {
		return FlavorRKE2
	}

	if c.isTanzuFlavor() {
		return FlavorTanzu
	}

	if c.isGKEFlavor() {
		return FlavorGKE
	}

	if c.isAKSFlavor() {
		return FlavorAKS
	}

	return FlavorKubernetes
}

// isOpenshiftFlavor - tries to hit an api registered by the openshift operator
// https://confluence.ngage.netapp.com/display/POLARIS/OpenShift+Questions
func (c *ClusterTypeChecker) isOpenshiftFlavor() bool {

	_, err := c.getOpenshiftVersion()
	if err != nil {
		if errors.IsNotFound(err) {
			return false
		}
		c.Log.Error(err, "error requesting for openshift version endpoint")
		return false
	}

	return true
}

func (c *ClusterTypeChecker) getOpenshiftVersion() (string, error) {
	// struct modeling only required information present in openshift API response
	var openshiftVersionSchema struct {
		Status struct {
			Versions []struct {
				Name    string
				Version string
			}
		}
	}

	versionBytes, err := c.K8sUtil.RESTGet("apis/config.openshift.io/v1/clusteroperators/openshift-apiserver")
	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(versionBytes, &openshiftVersionSchema); err != nil {
		return "", fmt.Errorf("failed to unmarshal version bytes: %v", err)
	}

	for _, versionObject := range openshiftVersionSchema.Status.Versions {
		if versionObject.Name == openShiftApiServerName {
			return versionObject.Version, nil
		}
	}

	return "", fmt.Errorf("failed to find version object for '%v'", openShiftApiServerName)
}

// isRKEFlavor - checks for the presence of the rancher API
func (c *ClusterTypeChecker) isRKEFlavor() bool {
	// Querying the base URL will return a list of supported API versions. If we wanted to we could
	// parse the response, extract the latest version and use it to query cluster CRs to ensure one
	// actually exists, but for now the presence of the API is sufficient for determining it is Rancher
	_, err := c.K8sUtil.RESTGet("apis/management.cattle.io")

	if err != nil {
		if errors.IsNotFound(err) {
			return false
		}
		c.Log.Error(err, "error querying the rancher API")
		return false
	}
	return true
}

// isRKE2Flavor - checks for the presence of the RKE2 base: k3s API
func (c *ClusterTypeChecker) isRKE2Flavor() bool {
	_, err := c.K8sUtil.RESTGet("apis/k3s.cattle.io")

	if err != nil {
		if errors.IsNotFound(err) {
			return false
		}
		c.Log.Error(err, "error querying the RKE2 API")
		return false
	}
	return true
}

// isTanzuFlavor - checks for the presence of the tanzu API
func (c *ClusterTypeChecker) isTanzuFlavor() bool {
	_, err := c.K8sUtil.RESTGet("apis/core.antrea.tanzu.vmware.com")

	if err != nil {
		if errors.IsNotFound(err) {
			return false
		}
		c.Log.Error(err, "error querying the TKG API")
		return false
	}
	return true
}

// isAKSFlavor - checks cluster roles for AKS service resource
// https://docs.microsoft.com/en-us/azure/aks/concepts-identity#clusterrolebinding
func (c *ClusterTypeChecker) isAKSFlavor() bool {
	const aksService = "aks-service"
	aksRoleBinding, err := c.K8sUtil.K8sClientset().RbacV1().ClusterRoles().Get(context.Background(), aksService, metav1.GetOptions{})
	blah, err := c.K8sUtil.K8sClientset().RbacV1().ClusterRoles().List(context.TODO(), metav1.ListOptions{})

	c.Log.WithValues(blah).Info("Hi")
	if err != nil {
		c.Log.Error(err, "Unable to get AKS cluster role. Assuming not AKS")
		return false
	}

	return aksRoleBinding != nil
}

// isGKEFlavor - checks for 'gke' in the client git version
// i.e. "gitVersion": "v1.20.6-gke.1000",
func (c *ClusterTypeChecker) isGKEFlavor() bool {
	version, err := c.K8sUtil.VersionGet()
	if err != nil {
		return false
	}

	return strings.Contains(strings.ToLower(version), FlavorGKE)
}

// isAnthosFlavor - checks for the presence of the Anthos API
func (c *ClusterTypeChecker) isAnthosFlavor() bool {
	_, err := c.K8sUtil.RESTGet("apis/anthos.gke.io")

	if err != nil {
		if errors.IsNotFound(err) {
			c.Log.Error(err, "error querying the Anthos API")
		}
		return false
	}
	return true
}
