package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	cachev1 "github.com/NetApp/astraagent-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// DeploymentForNatssyncClient returns a Natssync-client Deployment object
func (r *AstraAgentReconciler) DeploymentForNatssyncClient(m *cachev1.AstraAgent, ctx context.Context) (*appsv1.Deployment, error) {
	log := ctrllog.FromContext(ctx)
	ls := labelsForNatssyncClient(NatssyncClientName)

	var natssyncClientImage string
	if m.Spec.NatssyncClient.Image != "" {
		natssyncClientImage = m.Spec.NatssyncClient.Image
	} else {
		log.Info("Defaulting the natssyncClient image", "image", NatssyncClientDefaultImage)
		natssyncClientImage = NatssyncClientDefaultImage
	}

	natssyncCloudBridgeURL := r.getAstraHostURL(m, ctx)
	replicas := int32(NatssyncClientSize)
	keyStoreURLSplit := strings.Split(NatssyncClientKeystoreUrl, "://")
	if len(keyStoreURLSplit) < 2 {
		return nil, errors.New("invalid keyStoreURLSplit provided, format - configmap:///configmap-data")
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NatssyncClientName,
			Namespace: m.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: natssyncClientImage,
						Name:  NatssyncClientName,
						Env: []corev1.EnvVar{
							{
								Name:  "NATS_SERVER_URL",
								Value: r.GetNatsURL(m),
							},
							{
								Name:  "CLOUD_BRIDGE_URL",
								Value: natssyncCloudBridgeURL,
							},
							{
								Name:  "CONFIGMAP_NAME",
								Value: NatssyncClientConfigMapName,
							},
							{
								Name:  "POD_NAMESPACE",
								Value: m.Namespace,
							},
							{
								Name:  "KEYSTORE_URL",
								Value: NatssyncClientKeystoreUrl,
							},
							{
								Name:  "SKIP_TLS_VALIDATION",
								Value: strconv.FormatBool(m.Spec.NatssyncClient.SkipTLSValidation),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      NatssyncClientConfigMapVolumeName,
								MountPath: keyStoreURLSplit[1],
							},
						},
					}},
					ServiceAccountName: NatssyncClientConfigMapServiceAccountName,
					Volumes: []corev1.Volume{
						{
							Name: NatssyncClientConfigMapVolumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: NatssyncClientConfigMapName,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if m.Spec.NatssyncClient.HostAlias {
		hostNamesSplit := strings.Split(natssyncCloudBridgeURL, "://")
		if len(hostNamesSplit) < 2 {
			return nil, errors.New("invalid hostname provided, hostname format - https://hostname")
		}
		dep.Spec.Template.Spec.HostAliases = []corev1.HostAlias{
			{
				IP:        m.Spec.NatssyncClient.HostAliasIP,
				Hostnames: []string{hostNamesSplit[1]},
			},
		}
	}
	// Set astraAgent instance as the owner and controller
	err := ctrl.SetControllerReference(m, dep, r.Scheme)
	if err != nil {
		return nil, err
	}
	return dep, nil
}

// ServiceForNatssyncClient returns a Natssync-client Service object
func (r *AstraAgentReconciler) ServiceForNatssyncClient(m *cachev1.AstraAgent) *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NatssyncClientName,
			Namespace: m.Namespace,
			Labels: map[string]string{
				"app": NatssyncClientName,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Port:     NatssyncClientPort,
					Protocol: NatssyncClientProtocol,
				},
			},
			Selector: map[string]string{
				"app": NatssyncClientName,
			},
		},
	}
	// Set astraAgent instance as the owner and controller
	ctrl.SetControllerReference(m, service, r.Scheme)
	return service
}

// labelsForNatssyncClient returns the labels for selecting the NatssyncClient
func labelsForNatssyncClient(name string) map[string]string {
	return map[string]string{"app": name}
}

// ConfigMapForNatssyncClient returns a ConfigMap object for NatssyncClient
func (r *AstraAgentReconciler) ConfigMapForNatssyncClient(m *cachev1.AstraAgent) *corev1.ConfigMap {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: m.Namespace,
			Name:      NatssyncClientConfigMapName,
		},
	}
	ctrl.SetControllerReference(m, configMap, r.Scheme)
	return configMap
}

// ConfigMapRole returns a ConfigMapRole object for NatssyncClient
func (r *AstraAgentReconciler) ConfigMapRole(m *cachev1.AstraAgent) *rbacv1.Role {
	configMapRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: m.Namespace,
			Name:      NatssyncClientConfigMapRoleName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"get", "list", "patch"},
			},
		},
	}
	ctrl.SetControllerReference(m, configMapRole, r.Scheme)
	return configMapRole
}

// ConfigMapRoleBinding returns a Natssync-Client ConfigMapRoleBinding object
func (r *AstraAgentReconciler) ConfigMapRoleBinding(m *cachev1.AstraAgent) *rbacv1.RoleBinding {
	configMapRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: m.Namespace,
			Name:      NatssyncClientConfigMapRoleBindingName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: NatssyncClientConfigMapServiceAccountName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     NatssyncClientConfigMapRoleName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	ctrl.SetControllerReference(m, configMapRoleBinding, r.Scheme)
	return configMapRoleBinding
}

// ServiceAccountForNatssyncClientConfigMap returns a ServiceAccount object for NatssyncClient
func (r *AstraAgentReconciler) ServiceAccountForNatssyncClientConfigMap(m *cachev1.AstraAgent) *corev1.ServiceAccount {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NatssyncClientConfigMapServiceAccountName,
			Namespace: m.Namespace,
		},
	}
	ctrl.SetControllerReference(m, sa, r.Scheme)
	return sa
}

// getNatssyncClientStatus returns NatssyncClientStatus object
func (r *AstraAgentReconciler) getNatssyncClientStatus(m *cachev1.AstraAgent, ctx context.Context) (cachev1.NatssyncClientStatus, error) {
	pods := &corev1.PodList{}
	lb := labelsForNatssyncClient(NatssyncClientName)
	listOpts := []client.ListOption{
		client.MatchingLabels(lb),
	}
	log := ctrllog.FromContext(ctx)

	if err := r.List(ctx, pods, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "Namespace", m.Namespace)
		return cachev1.NatssyncClientStatus{}, err
	}

	natssyncClientStatus := cachev1.NatssyncClientStatus{}

	if len(pods.Items) < 1 {
		return cachev1.NatssyncClientStatus{}, errors.New("natssync-client pods not found")
	}
	nsClientPod := pods.Items[0]
	// If a pod is terminating, then we can't access the corresponding vault node's status.
	// so we break from here and return an error.
	if nsClientPod.Status.Phase != corev1.PodRunning || nsClientPod.DeletionTimestamp != nil {
		errNew := errors.New("natssync-client not in the desired state")
		log.Error(errNew, "natssync-client pod", "Phase", nsClientPod.Status.Phase, "DeletionTimestamp", nsClientPod.DeletionTimestamp)
		return cachev1.NatssyncClientStatus{}, errNew
	}

	natssyncClientStatus.State = string(nsClientPod.Status.Phase)
	natssyncClientLocationID, err := r.getNatssyncClientRegistrationStatus(r.getNatssyncClientRegistrationURL(m))
	if err != nil {
		log.Error(err, "Failed to get the registration status")
		return cachev1.NatssyncClientStatus{}, err
	}
	natssyncClientVersion, err := r.getNatssyncClientVersion(r.getNatssyncClientAboutURL(m))
	if err != nil {
		log.Error(err, "Failed to get the natssync-client version")
		return cachev1.NatssyncClientStatus{}, err
	}
	natssyncClientStatus.Registered = strconv.FormatBool(natssyncClientLocationID != "")
	natssyncClientStatus.LocationID = natssyncClientLocationID
	natssyncClientStatus.Version = natssyncClientVersion
	return natssyncClientStatus, nil
}

// getNatssyncClientRegistrationURL returns NatssyncClient Registration URL
func (r *AstraAgentReconciler) getNatssyncClientRegistrationURL(m *cachev1.AstraAgent) string {
	natsSyncClientURL := fmt.Sprintf("http://%s.%s:%d/bridge-client/1", NatssyncClientName, m.Namespace, NatssyncClientPort)
	natsSyncClientRegisterURL := fmt.Sprintf("%s/register", natsSyncClientURL)
	return natsSyncClientRegisterURL
}

// getNatssyncClientUnregisterURL returns NatssyncClient Unregister URL
func (r *AstraAgentReconciler) getNatssyncClientUnregisterURL(m *cachev1.AstraAgent) string {
	natsSyncClientURL := fmt.Sprintf("http://%s.%s:%d/bridge-client/1", NatssyncClientName, m.Namespace, NatssyncClientPort)
	natsSyncClientRegisterURL := fmt.Sprintf("%s/unregister", natsSyncClientURL)
	return natsSyncClientRegisterURL
}

// getNatssyncClientAboutURL returns NatssyncClient About URL
func (r *AstraAgentReconciler) getNatssyncClientAboutURL(m *cachev1.AstraAgent) string {
	natsSyncClientURL := fmt.Sprintf("http://%s.%s:%d/bridge-client/1", NatssyncClientName, m.Namespace, NatssyncClientPort)
	natsSyncClientAboutURL := fmt.Sprintf("%s/about", natsSyncClientURL)
	return natsSyncClientAboutURL
}

// getNatssyncClientRegistrationStatus returns the locationID string
func (r *AstraAgentReconciler) getNatssyncClientRegistrationStatus(natsSyncClientRegisterURL string) (string, error) {
	resp, err := http.Get(natsSyncClientRegisterURL)
	if err != nil {
		return "", err
	}

	type registrationResponse struct {
		LocationID string `json:"locationID"`
	}
	var registrationResp registrationResponse
	all, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(all, &registrationResp)
	if err != nil {
		return "", err
	}

	return registrationResp.LocationID, nil
}

// getNatssyncClientVersion returns the NatssyncClient Version
func (r *AstraAgentReconciler) getNatssyncClientVersion(natsSyncClientAboutURL string) (string, error) {
	resp, err := http.Get(natsSyncClientAboutURL)
	if err != nil {
		return "", err
	}

	type aboutResponse struct {
		AppVersion string `json:"appVersion,omitempty"`
	}
	var aboutResp aboutResponse
	all, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(all, &aboutResp)
	if err != nil {
		return "", err
	}
	return aboutResp.AppVersion, nil
}
