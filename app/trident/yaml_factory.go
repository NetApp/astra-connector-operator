// Copyright 2020 NetApp, Inc. All Rights Reserved.

package trident

import (
	"fmt"
	"github.com/NetApp-Polaris/astra-connector-operator/app/trident/kubernetes"
	"strconv"
	"strings"

	tridentconfig "github.com/netapp/trident/config"
	tridentversion "github.com/netapp/trident/utils/version"
)

func GetNamespaceYAML(name string) string {
	return strings.ReplaceAll(namespaceYAMLTemplate, "{NAMESPACE}", name)
}

const namespaceYAMLTemplate = `---
apiVersion: v1
kind: Namespace
metadata:
  name: {NAMESPACE}
`

func GetServiceAccountYAML(label, namespace string) string {

	serviceAccountYAML := serviceAccountYAML

	serviceAccountYAML = strings.ReplaceAll(serviceAccountYAML, "{NAME}", TridentOperatorServiceAccount)
	serviceAccountYAML = strings.ReplaceAll(serviceAccountYAML, "{NAMESPACE}", namespace)
	serviceAccountYAML = strings.ReplaceAll(serviceAccountYAML, "{LABEL}", label)

	return serviceAccountYAML
}

const serviceAccountYAML = `---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {NAME}
  namespace: {NAMESPACE}
  labels:
    app: {LABEL}
`

func GetClusterRoleYAML(k8sClient kubernetes.Interface, label string) string {

	var clusterRoleYAML string
	var yamlTemplate string
	flavor := k8sClient.Flavor()

	pspRemovedVersion := tridentversion.MustParseMajorMinorVersion(
		tridentconfig.PodSecurityPoliciesRemovedKubernetesVersion)
	if !k8sClient.ServerVersion().LessThan(pspRemovedVersion) {
		yamlTemplate = clusterRolePost125YAMLTemplate
	} else {
		yamlTemplate = clusterRolePre125YAMLTemplate
	}

	switch flavor {
	case kubernetes.FlavorOpenShift:
		clusterRoleYAML = strings.ReplaceAll(yamlTemplate, "{API_VERSION}", "authorization.openshift.io/v1")
	default:
		fallthrough
	case kubernetes.FlavorKubernetes:
		clusterRoleYAML = strings.ReplaceAll(yamlTemplate, "{API_VERSION}", "rbac.authorization.k8s.io/v1")
	}

	clusterRoleYAML = strings.ReplaceAll(clusterRoleYAML, "{NAME}", TridentOperatorClusterRoleName)
	clusterRoleYAML = strings.ReplaceAll(clusterRoleYAML, "{LABEL}", label)
	clusterRoleYAML = strings.ReplaceAll(clusterRoleYAML, "{PSP_NAME}", TridentOperatorPodSecurityPolicy)
	clusterRoleYAML = strings.ReplaceAll(clusterRoleYAML, "{APP_NAME}", TridentOperatorDeploymentName)

	return clusterRoleYAML
}

const clusterRolePre125YAMLTemplate = `---
apiVersion: {API_VERSION}
kind: ClusterRole
metadata:
  name: {NAME}
  labels:
    app: {LABEL}
rules:
  # Permissions same as Trident
  - apiGroups:
      - ""
    resources:
      - namespaces
    verbs:
      - get
      - list
  - apiGroups:
      - ""
    resources:
      - persistentvolumes
      - persistentvolumeclaims
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - update
      - patch
  - apiGroups:
      - ""
    resources:
      - persistentvolumeclaims/status
    verbs:
      - update
      - patch
  - apiGroups:
      - storage.k8s.io
    resources:
      - storageclasses
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - update
      - patch
  - apiGroups:
      - ""
    resources:
      - events
    verbs:
      - get
      - list
      - watch
      - create
      - update
      - patch
  - apiGroups:
      - ""
    resources:
      - secrets
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - update
      - patch
  - apiGroups:
      - ""
    resources:
      - resourcequotas
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - update
      - patch
  - apiGroups:
      - ""
    resources:
      - pods
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - update
      - patch
  - apiGroups:
      - ""
    resources:
      - pods/log
    verbs:
      - get
      - list
      - watch
  - apiGroups:
      - ""
    resources:
      - nodes
    verbs:
      - get
      - list
      - watch
      - update
  - apiGroups:
      - storage.k8s.io
    resources:
      - volumeattachments
    verbs:
      - get
      - list
      - watch
      - update
      - patch
  - apiGroups:
      - storage.k8s.io
    resources:
      - volumeattachments/status
    verbs:
      - update
      - patch
  - apiGroups:
      - snapshot.storage.k8s.io
    resources:
      - volumesnapshots
      - volumesnapshotclasses
    verbs:
      - get
      - list
      - watch
      - update
      - patch
  - apiGroups:
      - snapshot.storage.k8s.io
    resources:
      - volumesnapshots/status
      - volumesnapshotcontents/status
    verbs:
      - update
      - patch
  - apiGroups:
      - snapshot.storage.k8s.io
    resources:
      - volumesnapshotcontents
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - update
      - patch
  - apiGroups:
      - csi.storage.k8s.io
    resources:
      - csidrivers
      - csinodeinfos
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - update
      - patch
  - apiGroups:
      - storage.k8s.io
    resources:
      - csidrivers
      - csinodes
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - update
      - patch
  - apiGroups:
      - apiextensions.k8s.io
    resources:
      - customresourcedefinitions
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - update
      - patch
  - apiGroups:
      - trident.netapp.io
    resources:
      - tridentversions
      - tridentbackends
      - tridentstorageclasses
      - tridentvolumes
      - tridentvolumepublications
      - tridentvolumereferences
      - tridentnodes
      - tridenttransactions
      - tridentsnapshots
      - tridentbackendconfigs
      - tridentbackendconfigs/status
      - tridentmirrorrelationships
      - tridentmirrorrelationships/status
      - tridentactionmirrorupdates
      - tridentactionmirrorupdates/status
      - tridentsnapshotinfos
      - tridentsnapshotinfos/status
      - tridentactionsnapshotrestores
      - tridentactionsnapshotrestores/status
      - tridentprovisioners # Required for Tprov
      - tridentprovisioners/status # Required to update Tprov's status section
      - tridentorchestrators # Required for Torc
      - tridentorchestrators/status # Required to update Torc's status section
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - update
      - patch
  - apiGroups:
      - policy
    resources:
      - podsecuritypolicies
    verbs:
      - use
    resourceNames:
      - tridentpods
      - trident-controller
      - trident-node-linux
      - trident-node-windows
  # Now Operator specific permissions
  - apiGroups:
      - ""
    resources:
      - namespaces
    verbs:
      - create
      - patch
  - apiGroups:
      - apps
    resources:
      - deployments
      - daemonsets
      - statefulsets
    verbs:
      - get
      - list
      - watch
      - create
  - apiGroups:
      - apps
    resources:
      - deployments
      - statefulsets
    verbs:
      - delete
      - update
      - patch
    resourceNames:
      - trident
      - trident-csi
      - trident-controller
  - apiGroups:
      - apps
    resources:
      - daemonsets
    verbs:
      - delete
      - update
      - patch
    resourceNames:
      - trident
      - trident-csi
      - trident-csi-windows
      - trident-node-linux
      - trident-node-windows
  - apiGroups:
      - ""
    resources:
      - pods/exec
      - services
      - serviceaccounts
    verbs:
      - get
      - list
      - create
  - apiGroups:
      - ""
    resources:
      - pods/exec
      - services
    verbs:
      - delete
      - update
      - patch
    resourceNames:
      - trident-csi
      - trident
  - apiGroups:
      - ""
    resources:
      - serviceaccounts
    verbs:
      - delete
      - update
      - patch
    resourceNames:
      - trident-controller
      - trident-node-linux
      - trident-node-windows
      - trident-csi
      - trident
  - apiGroups:
      - authorization.openshift.io
      - rbac.authorization.k8s.io
    resources:
      - roles
      - rolebindings
      - clusterroles
      - clusterrolebindings
    verbs:
      - list
      - create
  - apiGroups:
      - authorization.openshift.io
      - rbac.authorization.k8s.io
    resources:
      - roles
      - rolebindings
      - clusterroles
      - clusterrolebindings
    verbs:
      - delete
      - update
      - patch
    resourceNames:
      - trident-controller
      - trident-node-linux
      - trident-node-windows
      - trident-csi
      - trident
  - apiGroups:
      - policy
    resources:
      - podsecuritypolicies
    verbs:
      - list
      - create
  - apiGroups:
      - policy
    resources:
      - podsecuritypolicies
    resourceNames:
      - tridentpods
      - trident-controller
      - trident-node-linux
      - trident-node-windows
    verbs:
      - delete
      - update
      - patch
  - apiGroups:
      - security.openshift.io
    resources:
      - securitycontextconstraints
    verbs:
      - get
      - list
      - create
  - apiGroups:
      - security.openshift.io
    resources:
      - securitycontextconstraints
    resourceNames:
      - trident-controller
      - trident-node-linux
      - trident-node-windows
      - trident
    verbs:
      - delete
      - update
      - patch
  - apiGroups:
      - policy
    resources:
      - podsecuritypolicies
    verbs:
      - use
    resourceNames:
      - tridentoperatorpods
`

const clusterRolePost125YAMLTemplate = `---
apiVersion: {API_VERSION}
kind: ClusterRole
metadata:
  name: {NAME}
  labels:
    app: {LABEL}
rules:
  # Permissions same as Trident
  - apiGroups:
      - ""
    resources:
      - namespaces
    verbs:
      - get
      - list
  - apiGroups:
      - ""
    resources:
      - persistentvolumes
      - persistentvolumeclaims
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - update
      - patch
  - apiGroups:
      - ""
    resources:
      - persistentvolumeclaims/status
    verbs:
      - update
      - patch
  - apiGroups:
      - storage.k8s.io
    resources:
      - storageclasses
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - update
      - patch
  - apiGroups:
      - ""
    resources:
      - events
    verbs:
      - get
      - list
      - watch
      - create
      - update
      - patch
  - apiGroups:
      - ""
    resources:
      - secrets
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - update
      - patch
  - apiGroups:
      - ""
    resources:
      - resourcequotas
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - update
      - patch
  - apiGroups:
      - ""
    resources:
      - pods
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - update
      - patch
  - apiGroups:
      - ""
    resources:
      - pods/log
    verbs:
      - get
      - list
      - watch
  - apiGroups:
      - ""
    resources:
      - nodes
    verbs:
      - get
      - list
      - watch
      - update
  - apiGroups:
      - storage.k8s.io
    resources:
      - volumeattachments
    verbs:
      - get
      - list
      - watch
      - update
      - patch
  - apiGroups:
      - storage.k8s.io
    resources:
      - volumeattachments/status
    verbs:
      - update
      - patch
  - apiGroups:
      - snapshot.storage.k8s.io
    resources:
      - volumesnapshots
      - volumesnapshotclasses
    verbs:
      - get
      - list
      - watch
      - update
      - patch
  - apiGroups:
      - snapshot.storage.k8s.io
    resources:
      - volumesnapshots/status
      - volumesnapshotcontents/status
    verbs:
      - update
      - patch
  - apiGroups:
      - snapshot.storage.k8s.io
    resources:
      - volumesnapshotcontents
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - update
      - patch
  - apiGroups:
      - csi.storage.k8s.io
    resources:
      - csidrivers
      - csinodeinfos
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - update
      - patch
  - apiGroups:
      - storage.k8s.io
    resources:
      - csidrivers
      - csinodes
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - update
      - patch
  - apiGroups:
      - apiextensions.k8s.io
    resources:
      - customresourcedefinitions
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - update
      - patch
  - apiGroups:
      - trident.netapp.io
    resources:
      - tridentversions
      - tridentbackends
      - tridentstorageclasses
      - tridentvolumes
      - tridentvolumepublications
      - tridentvolumereferences
      - tridentnodes
      - tridenttransactions
      - tridentsnapshots
      - tridentbackendconfigs
      - tridentbackendconfigs/status
      - tridentmirrorrelationships
      - tridentmirrorrelationships/status
      - tridentactionmirrorupdates
      - tridentactionmirrorupdates/status
      - tridentsnapshotinfos
      - tridentsnapshotinfos/status
      - tridentactionsnapshotrestores
      - tridentactionsnapshotrestores/status
      - tridentprovisioners # Required for Tprov
      - tridentprovisioners/status # Required to update Tprov's status section
      - tridentorchestrators # Required for Torc
      - tridentorchestrators/status # Required to update Torc's status section
    verbs:
      - get
      - list
      - watch
      - create
      - delete
      - update
      - patch
  - apiGroups:
      - policy
    resources:
      - podsecuritypolicies
    verbs:
      - use
    resourceNames:
      - tridentpods
  # Now Operator specific permissions
  - apiGroups:
      - ""
    resources:
      - namespaces
    verbs:
      - create
      - patch
  - apiGroups:
      - apps
    resources:
      - deployments
      - daemonsets
      - statefulsets
    verbs:
      - get
      - list
      - watch
      - create
  - apiGroups:
      - apps
    resources:
      - deployments
      - statefulsets
    verbs:
      - delete
      - update
      - patch
    resourceNames:
      - trident
      - trident-csi
      - trident-controller
  - apiGroups:
      - apps
    resources:
      - daemonsets
    verbs:
      - delete
      - update
      - patch
    resourceNames:
      - trident
      - trident-csi
      - trident-csi-windows
      - trident-node-linux
      - trident-node-windows
  - apiGroups:
      - ""
    resources:
      - pods/exec
      - services
      - serviceaccounts
    verbs:
      - get
      - list
      - create
  - apiGroups:
      - ""
    resources:
      - pods/exec
      - services
    verbs:
      - delete
      - update
      - patch
    resourceNames:
      - trident-csi
      - trident
  - apiGroups:
      - ""
    resources:
      - serviceaccounts
    verbs:
      - delete
      - update
      - patch
    resourceNames:
      - trident-controller
      - trident-node-linux
      - trident-node-windows
      - trident-csi
      - trident
  - apiGroups:
      - authorization.openshift.io
      - rbac.authorization.k8s.io
    resources:
      - roles
      - rolebindings
      - clusterroles
      - clusterrolebindings
    verbs:
      - list
      - create
  - apiGroups:
      - authorization.openshift.io
      - rbac.authorization.k8s.io
    resources:
      - roles
      - rolebindings
      - clusterroles
      - clusterrolebindings
    verbs:
      - delete
      - update
      - patch
    resourceNames:
      - trident-controller
      - trident-node-linux
      - trident-node-windows
      - trident-csi
      - trident
  - apiGroups:
      - policy
    resources:
      - podsecuritypolicies
    verbs:
      - list
      - create
  - apiGroups:
      - policy
    resources:
      - podsecuritypolicies
    resourceNames:
      - tridentpods
    verbs:
      - delete
      - update
      - patch
  - apiGroups:
      - security.openshift.io
    resources:
      - securitycontextconstraints
    verbs:
      - get
      - list
      - create
  - apiGroups:
      - security.openshift.io
    resources:
      - securitycontextconstraints
    resourceNames:
      - trident-controller
      - trident-node-linux
      - trident-node-windows
      - trident
    verbs:
      - delete
      - update
      - patch
  - apiGroups:
      - policy
    resources:
      - podsecuritypolicies
    verbs:
      - use
    resourceNames:
      - trident-controller
      - trident-node-linux
      - trident-node-windows
      - tridentoperatorpods
`

func GetClusterRoleBindingYAML(flavor kubernetes.OrchestratorFlavor, label, namespace string) string {

	var crbYAML string

	switch flavor {
	case kubernetes.FlavorOpenShift:
		crbYAML = clusterRoleBindingOpenShiftYAMLTemplate
	default:
		fallthrough
	case kubernetes.FlavorKubernetes:
		crbYAML = clusterRoleBindingKubernetesYAMLTemplate
	}

	crbYAML = strings.ReplaceAll(crbYAML, "{NAME}", TridentOperatorClusterRoleBindingName)
	crbYAML = strings.ReplaceAll(crbYAML, "{LABEL}", label)
	crbYAML = strings.ReplaceAll(crbYAML, "{SA_NAME}", TridentOperatorServiceAccount)
	crbYAML = strings.ReplaceAll(crbYAML, "{NAMESPACE}", namespace)
	crbYAML = strings.ReplaceAll(crbYAML, "{CR_NAME}", TridentOperatorClusterRoleName)

	return crbYAML
}

const clusterRoleBindingOpenShiftYAMLTemplate = `---
kind: ClusterRoleBinding
apiVersion: authorization.openshift.io/v1
metadata:
  name: {NAME}
  labels:
    app: {LABEL}
subjects:
  - kind: ServiceAccount
    name: {SA_NAME}
    namespace: {NAMESPACE}
roleRef:
  kind: ClusterRole
  name: {CR_NAME}
`

const clusterRoleBindingKubernetesYAMLTemplate = `---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: {NAME}
  labels:
    app: {LABEL}
subjects:
  - kind: ServiceAccount
    name: {SA_NAME}
    namespace: {NAMESPACE}
roleRef:
  kind: ClusterRole
  name: {CR_NAME}
  apiGroup: rbac.authorization.k8s.io
`

func GetImagePullSecretYAML(name, namespace, label string) string {

	secretYAML := imagePullSecretTemplate

	secretYAML = strings.ReplaceAll(secretYAML, "{NAME}", name)
	secretYAML = strings.ReplaceAll(secretYAML, "{NAMESPACE}", namespace)
	secretYAML = strings.ReplaceAll(secretYAML, "{LABEL}", label)
	secretYAML = strings.ReplaceAll(secretYAML, "{CREDS}", ArtifactoryImagePullCredsBase64)

	return secretYAML
}

const imagePullSecretTemplate = `---
apiVersion: v1
kind: Secret
type: kubernetes.io/dockerconfigjson
metadata:
  name: {NAME}
  namespace: {NAMESPACE}
  labels:
    app: {LABEL}
data:
  .dockerconfigjson: {CREDS}
`

func GetDeploymentYAML(namespace, operatorImage, label, logFormat string, debug bool) string {

	var debugLine string
	if debug {
		debugLine = "- --debug"
	} else {
		debugLine = "#- --debug"
	}

	deploymentYAML := deploymentYAMLTemplate

	deploymentYAML = strings.ReplaceAll(deploymentYAML, "{OPERATOR_IMAGE}", operatorImage)
	deploymentYAML = strings.ReplaceAll(deploymentYAML, "{NAME}", TridentOperatorDeploymentName)
	deploymentYAML = strings.ReplaceAll(deploymentYAML, "{NAMESPACE}", namespace)
	deploymentYAML = strings.ReplaceAll(deploymentYAML, "{SA_NAME}", TridentOperatorServiceAccount)
	deploymentYAML = strings.ReplaceAll(deploymentYAML, "{DEBUG}", debugLine)
	deploymentYAML = strings.ReplaceAll(deploymentYAML, "{LABEL}", label)
	deploymentYAML = strings.ReplaceAll(deploymentYAML, "{LOG_FORMAT}", logFormat)
	deploymentYAML = strings.ReplaceAll(deploymentYAML, "{IMAGE_PULL_SECRET}", TridentOperatorImagePullSecretName)

	return deploymentYAML
}

const deploymentYAMLTemplate = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {NAME}
  namespace: {NAMESPACE}
  labels:
    app: {LABEL}
spec:
  replicas: 1
  selector:
    matchLabels:
      name: {NAME}
      app: {LABEL}
  template:
    metadata:
      labels:
        name: {NAME}
        app: {LABEL}
    spec:
      serviceAccountName: {SA_NAME}
      containers:
      - name: {NAME}
        image: {OPERATOR_IMAGE}
        command:
        - "/trident-operator"
        args:
        - "--log-format={LOG_FORMAT}"
        {DEBUG}
        imagePullPolicy: IfNotPresent
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: OPERATOR_NAME
          value: {NAME}
      imagePullSecrets:
      - name: {IMAGE_PULL_SECRET}
      nodeSelector:
        kubernetes.io/os: linux
        kubernetes.io/arch: amd64
`

func GetPodSecurityPolicyYAML(label string) string {

	pspYAML := podSecurityPolicyYAMLTemplate

	pspYAML = strings.ReplaceAll(pspYAML, "{NAME}", TridentOperatorPodSecurityPolicy)
	pspYAML = strings.ReplaceAll(pspYAML, "{LABEL}", label)

	return pspYAML
}

const podSecurityPolicyYAMLTemplate = `
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: {NAME}
  labels:
    app: {LABEL}
spec:
  privileged: false
  seLinux:
    rule: RunAsAny
  supplementalGroups:
    rule: RunAsAny
  runAsUser:
    rule: RunAsAny
  fsGroup:
    rule: RunAsAny
  volumes:
    - '*'
`

func GetTridentOrchestratorCRDYAML() string {
	return tridentOrchestratorCRDTemplate
}

const tridentOrchestratorCRDTemplate = `---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: tridentorchestrators.trident.netapp.io
spec:
  group: trident.netapp.io
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          x-kubernetes-preserve-unknown-fields: true
      subresources:
        status: {}
  names:
    kind: TridentOrchestrator
    listKind: TridentOrchestratorList
    plural: tridentorchestrators
    singular: tridentorchestrator
    shortNames:
    - torc
    - torchestrator
  scope: Cluster`

func GetTridentOrchestratorCRYAML(debug, useIPv6, windows bool, namespace, serialNumber, hostname, logFormat,
	tridentImage, tridentAutosupportImage, proxyURL, imageRegistry, acpImage string) string {

	crYAML := tridentOrchestratorCRTemplate

	crYAML = strings.ReplaceAll(crYAML, "{NAMESPACE}", namespace)
	crYAML = strings.ReplaceAll(crYAML, "{DEBUG}", strconv.FormatBool(debug))
	crYAML = strings.ReplaceAll(crYAML, "{ACP_IMAGE}", acpImage)
	crYAML = strings.ReplaceAll(crYAML, "{IPv6}", strconv.FormatBool(useIPv6))
	crYAML = strings.ReplaceAll(crYAML, "{WINDOWS}", strconv.FormatBool(windows))
	crYAML = strings.ReplaceAll(crYAML, "{TRIDENT_AUTOSUPPORT_IMAGE}", tridentAutosupportImage)
	crYAML = strings.ReplaceAll(crYAML, "{SERIAL_NUMBER}", fmt.Sprintf(`"%s"`, serialNumber))
	crYAML = strings.ReplaceAll(crYAML, "{HOSTNAME}", hostname)
	crYAML = strings.ReplaceAll(crYAML, "{LOG_FORMAT}", logFormat)
	crYAML = strings.ReplaceAll(crYAML, "{TRIDENT_IMAGE}", tridentImage)
	crYAML = strings.ReplaceAll(crYAML, "{IMAGE_REGISTRY}", imageRegistry)
	crYAML = strings.ReplaceAll(crYAML, "{PROXY_URL}", proxyURL)
	crYAML = strings.ReplaceAll(crYAML, "{IMAGE_PULL_SECRET}", TridentOperatorImagePullSecretName)

	return crYAML
}

const tridentOrchestratorCRTemplate = `---
apiVersion: trident.netapp.io/v1
kind: TridentOrchestrator
metadata:
  name: trident
spec:
  namespace: {NAMESPACE}
  debug: {DEBUG}
  enableACP: true
  acpImage: {ACP_IMAGE}
  IPv6: {IPv6}
  autosupportImage: {TRIDENT_AUTOSUPPORT_IMAGE}
  autosupportProxy: {PROXY_URL}
  autosupportSerialNumber: {SERIAL_NUMBER}
  autosupportHostname: {HOSTNAME}
  logFormat: {LOG_FORMAT}
  tridentImage: {TRIDENT_IMAGE}
  imageRegistry: {IMAGE_REGISTRY}
  imagePullSecrets:
  - {IMAGE_PULL_SECRET}
  windows: {WINDOWS}
`

func GetGCPStorageClassYAML(name, backendType, serviceLevel string, isDefault, isHardware bool) string {

	var storageClass, volumeBindingMode string

	if isHardware {
		storageClass = CVSStorageClassHardware
		volumeBindingMode = "Immediate"
	} else {
		storageClass = CVSStorageClassSoftware
		volumeBindingMode = "WaitForFirstConsumer"
	}

	scYAML := gcpStorageClassTemplate
	scYAML = strings.ReplaceAll(scYAML, "{NAME}", name)
	scYAML = strings.ReplaceAll(scYAML, "{DEFAULT}", strconv.FormatBool(isDefault))
	scYAML = strings.ReplaceAll(scYAML, "{BACKEND_TYPE}", backendType)
	scYAML = strings.ReplaceAll(scYAML, "{SERVICE_LEVEL}", serviceLevel)
	scYAML = strings.ReplaceAll(scYAML, "{STORAGE_CLASS}", storageClass)
	scYAML = strings.ReplaceAll(scYAML, "{VOLUME_BINDING_MODE}", volumeBindingMode)

	return scYAML
}

const gcpStorageClassTemplate = `---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: {NAME}
  annotations:
    storageclass.kubernetes.io/is-default-class: "{DEFAULT}"
provisioner: csi.trident.netapp.io
parameters:
  backendType: {BACKEND_TYPE}
  selector: serviceLevel={SERVICE_LEVEL};storageClass={STORAGE_CLASS}
volumeBindingMode: {VOLUME_BINDING_MODE}
allowVolumeExpansion: true
`

func GetANFStorageClassYAML(name, backendType, serviceLevel string, isDefault bool) string {

	scYAML := anfStorageClassTemplate
	scYAML = strings.ReplaceAll(scYAML, "{NAME}", name)
	scYAML = strings.ReplaceAll(scYAML, "{DEFAULT}", strconv.FormatBool(isDefault))
	scYAML = strings.ReplaceAll(scYAML, "{BACKEND_TYPE}", backendType)
	scYAML = strings.ReplaceAll(scYAML, "{SERVICE_LEVEL}", serviceLevel)

	return scYAML
}

const anfStorageClassTemplate = `---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: {NAME}
  annotations:
    storageclass.kubernetes.io/is-default-class: "{DEFAULT}"
provisioner: csi.trident.netapp.io
parameters:
  backendType: {BACKEND_TYPE}
  selector: serviceLevel={SERVICE_LEVEL}
volumeBindingMode: Immediate
allowVolumeExpansion: true
`

func GetVolumeSnapshotClassYAML(name string) string {

	vscYAML := volumeSnapshotClassTemplate

	vscYAML = strings.ReplaceAll(vscYAML, "{NAME}", name)

	return vscYAML
}

const volumeSnapshotClassTemplate = `---
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: {NAME}
driver: csi.trident.netapp.io
deletionPolicy: Delete
`

func GetVolumeImportPVCYAML(name, namespace, storageClass, accessModes string) string {
	volImportPvcYAML := volumeImportPvcTemplate
	volImportPvcYAML = strings.ReplaceAll(volImportPvcYAML, "{NAME}", name)
	volImportPvcYAML = strings.ReplaceAll(volImportPvcYAML, "{NAMESPACE}", namespace)
	volImportPvcYAML = strings.ReplaceAll(volImportPvcYAML, "{STORAGECLASS}", storageClass)
	volImportPvcYAML = strings.ReplaceAll(volImportPvcYAML, "{ACCESSMODES}", accessModes)

	return volImportPvcYAML
}

const volumeImportPvcTemplate = `---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: {NAME}
  namespace: "{NAMESPACE}"
spec:
  storageClassName: {STORAGECLASS}
  accessModes:
    - {ACCESSMODES}
`
