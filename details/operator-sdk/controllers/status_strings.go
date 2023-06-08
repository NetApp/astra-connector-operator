package controllers

const (
	CreateStatefulSet        = "Creating StatefulSet %s/%s"
	CreateRoleBinding        = "Creating RoleBinding %s/%s"
	CreateServiceAccount     = "Creating ServiceAccount %s/%s"
	CreateConfigMap          = "Creating ConfigMap %s/%s"
	CreateDeployment         = "Creating Deployment %s/%s"
	UpdateDeployment         = "Updating Deployment %s/%s"
	CreateService            = "Creating Service %s/%s"
	CreateRole               = "Creating Role %s/%s"
	CreateClusterRole        = "Creating ClusterRole %s/%s"
	CreateClusterRoleBinding = "Creating ClusterRoleBinding %s/%s"

	ErrorCreateStatefulSets        = "Error creating StatefulSets %s/%s"
	ErrorCreateRoleBindings        = "Error creating RoleBindings %s/%s"
	ErrorCreateClusterRoleBindings = "Error creating ClusterRoleBindings %s/%s"
	ErrorCreateServiceAccounts     = "Error creating ServiceAccounts %s/%s"
	ErrorCreateConfigMaps          = "Error creating ConfigMaps %s/%s"
	ErrorCreateDeployments         = "Error creating Deployments %s/%s"
	ErrorCreateService             = "Error creating Services  %s/%s"
	ErrorCreateRoles               = "Error creating Roles  %s/%s"
	ErrorCreateClusterRoles        = "Error creating ClusterRoles %s/%s"

	FailedFinalizerAdd      = "Failed to add finalizer"
	FailedFinalizerRemove   = "Failed to remove finalizer"
	FailedAstraConnectorGet = "Failed to get AstraConnector"

	FailedLocationIDGet = "Failed to get the locationID from ConfigMap"
	EmptyLocationIDGet  = "Got an empty location ID from ConfigMap"

	RegisterNSClient         = "Registered natssync-client"
	FailedRegisterNSClient   = "Failed to register natssync-client"
	UnregisterNSClient       = "Unregistered natssync-client"
	FailedUnRegisterNSClient = "Failed to unregister natssync-client"

	UnregisterFromAstra = "Unregistered the cluster with Astra"

	FailedConnectorIDAdd    = "Failed to add ConnectorID to Astra"
	FailedConnectorIDRemove = "Failed to remove the ConnectorID from Astra"
)
