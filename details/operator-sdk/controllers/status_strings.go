package controllers

const (
	CreateStatefulSet        = "Creating StatefulSet %s/%s"
	CreateRoleBinding        = "Creating RoleBinding %s/%s"
	CreateServiceAccount     = "Creating ServiceAccount %s/%s"
	CreateConfigMap          = "Creating ConfigMap %s/%s"
	CreateDeployment         = "Creating Deployment %s/%s"
	CreateService            = "Creating Service %s/%s"
	CreateRole               = "Creating Role %s/%s"
	CreateClusterRole        = "Creating ClusterRole %s/%s"
	CreateClusterRoleBinding = "Creating ClusterRoleBinding %s/%s"

	DeleteInProgress = "Delete In Progress"
	DeletionComplete = "Deletion Complete"

	ErrorCreateStatefulSets        = "Error creating StatefulSets %s/%s"
	ErrorCreateRoleBindings        = "Error creating RoleBindings %s/%s"
	ErrorCreateClusterRoleBindings = "Error creating ClusterRoleBindings %s/%s"
	ErrorCreateServiceAccounts     = "Error creating ServiceAccounts %s/%s"
	ErrorCreateConfigMaps          = "Error creating ConfigMaps %s/%s"
	ErrorCreateDeployments         = "Error creating Deployments %s/%s"
	ErrorCreateService             = "Error creating Services  %s/%s"
	ErrorCreateRoles               = "Error creating Roles  %s/%s"
	ErrorCreateClusterRoles        = "Error creating ClusterRoles %s/%s"

	FailedFinalizerAdd             = "Failed to add finalizer"
	FailedFinalizerRemove          = "Failed to remove finalizer"
	FailedAstraConnectorGet        = "Failed to get AstraConnector"
	FailedAstraConnectorValidation = "Failed to validate AstraConnector"

	FailedLocationIDGet = "Failed to get the locationID from ConfigMap"
	EmptyLocationIDGet  = "Got an empty location ID from ConfigMap"

	RegisterNSClient         = "Registered natsSyncClient"
	FailedRegisterNSClient   = "Failed to register natsSyncClient"
	UnregisterNSClient       = "Unregistered natsSyncClient"
	FailedUnRegisterNSClient = "Failed to unregister natsSyncClient"
	FailedASUPCreation       = "Failed to create ASUP CR"

	DeployedComponents  = "Deployed all the connector components"
	RegisteredWithAstra = "Registered with Astra"

	FailedConnectorIDAdd = "Failed to add cluster to Astra"
)
