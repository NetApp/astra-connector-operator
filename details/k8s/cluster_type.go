package k8s

import (
	"context"
	"encoding/json"
	"fmt"
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
	FlavorTanzu      = "tanzu"
	FlavorAnthos     = "anthos"

	openShiftApiServerName = "openshift-apiserver"
)

func (r *K8sUtil) DetermineClusterType() string {
	if r.isAnthosFlavor() {
		return FlavorAnthos
	}

	if r.isOpenshiftFlavor() {
		return FlavorOpenShift
	}

	if r.isRKEFlavor() || r.isRKE2Flavor() {
		return FlavorRKE
	}

	if r.isTanzuFlavor() {
		return FlavorTanzu
	}

	if r.isGKEFlavor() {
		return FlavorGKE
	}

	if r.isAKSFlavor() {
		return FlavorAKS
	}

	return FlavorKubernetes
}

// isOpenshiftFlavor - tries to hit an api registered by the openshift operator
// https://confluence.ngage.netapp.com/display/POLARIS/OpenShift+Questions
func (r *K8sUtil) isOpenshiftFlavor() bool {

	_, err := r.getOpenshiftVersion()
	if err != nil {
		if errors.IsNotFound(err) {
			return false
		}
		r.Log.Error(err, "error requesting for openshift version endpoint")
		return false
	}

	return true
}

func (r *K8sUtil) getOpenshiftVersion() (string, error) {
	// struct modeling only required information present in openshift API response
	var openshiftVersionSchema struct {
		Status struct {
			Versions []struct {
				Name    string
				Version string
			}
		}
	}

	versionBytes, err := r.Interface.Discovery().RESTClient().Get().
		AbsPath("apis/config.openshift.io/v1/clusteroperators/openshift-apiserver").DoRaw(context.TODO())
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
func (r *K8sUtil) isRKEFlavor() bool {
	// Querying the base URL will return a list of supported API versions. If we wanted to we could
	// parse the response, extract the latest version and use it to query cluster CRs to ensure one
	// actually exists, but for now the presence of the API is sufficient for determining it is Rancher
	_, err := r.Interface.Discovery().RESTClient().Get().
		AbsPath("apis/management.cattle.io").DoRaw(context.TODO())

	if err != nil {
		if errors.IsNotFound(err) {
			return false
		}
		r.Log.Error(err, "error querying the rancher API")
		return false
	}
	return true
}

// isRKE2Flavor - checks for the presence of the RKE2 base: k3s API
func (r *K8sUtil) isRKE2Flavor() bool {
	_, err := r.Interface.Discovery().RESTClient().Get().
		AbsPath("apis/k3s.cattle.io").DoRaw(context.TODO())

	if err != nil {
		if errors.IsNotFound(err) {
			return false
		}
		r.Log.Error(err, "error querying the RKE2 API")
		return false
	}
	return true
}

// isTanzuFlavor - checks for the presence of the tanzu API
func (r *K8sUtil) isTanzuFlavor() bool {
	_, err := r.Interface.Discovery().RESTClient().Get().
		AbsPath("apis/core.antrea.tanzu.vmware.com").DoRaw(context.TODO())

	if err != nil {
		if errors.IsNotFound(err) {
			return false
		}
		r.Log.Error(err, "error querying the TKG API")
		return false
	}
	return true
}

// isAKSFlavor - checks cluster roles for AKS service resource
// https://docs.microsoft.com/en-us/azure/aks/concepts-identity#clusterrolebinding
func (r *K8sUtil) isAKSFlavor() bool {
	const aksService = "aks-service"
	aksRoleBinding, err := r.Interface.RbacV1().ClusterRoles().Get(context.Background(), aksService, metav1.GetOptions{})
	if err != nil {
		r.Log.Error(err, "Unable to get AKS cluster role. Assuming not AKS")
		return false
	}

	return aksRoleBinding != nil
}

// isGKEFlavor - checks for 'gke' in the client git version
// i.e. "gitVersion": "v1.20.6-gke.1000",
func (r *K8sUtil) isGKEFlavor() bool {
	versionInfo, err := r.Interface.Discovery().ServerVersion()
	if err != nil {
		return false
	}

	return strings.Contains(strings.ToLower(versionInfo.GitVersion), FlavorGKE)
}

// isAnthosFlavor - checks for the presence of the Anthos API
func (r *K8sUtil) isAnthosFlavor() bool {
	_, err := r.Interface.Discovery().RESTClient().Get().
		AbsPath("apis/anthos.gke.io").DoRaw(context.TODO())

	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Error(err, "error querying the Anthos API")
		}
		return false
	}
	return true
}
