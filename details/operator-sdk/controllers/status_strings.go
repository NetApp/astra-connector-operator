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

	WaitForClusterManagedState = "Waiting for cluster state 'managed'"

	DeleteInProgress = "AstraConnector deletion in progress"
	DeletionComplete = "AstraConnector deletion complete"

	ErrorCreateStatefulSets        = "Error creating StatefulSets %s/%s"
	ErrorCreateRoleBindings        = "Error creating RoleBindings %s/%s"
	ErrorCreateClusterRoleBindings = "Error creating ClusterRoleBindings %s/%s"
	ErrorCreateServiceAccounts     = "Error creating ServiceAccounts %s/%s"
	ErrorCreateConfigMaps          = "Error creating ConfigMaps %s/%s"
	ErrorCreateDeployments         = "Error creating Deployments %s/%s"
	ErrorCreateService             = "Error creating Services  %s/%s"
	ErrorCreateRoles               = "Error creating Roles  %s/%s"
	ErrorCreateClusterRoles        = "Error creating ClusterRoles %s/%s"
	ErrorClusterUnmanaged          = "Timed out waiting for cluster to become managed"

	FailedFinalizerAdd             = "Failed to add finalizer"
	FailedFinalizerRemove          = "Failed to remove finalizer"
	FailedAstraConnectorGet        = "Failed to get AstraConnector"
	FailedAstraConnectorValidation = "Failed to validate AstraConnector"

	FailedASUPCreation = "Failed to create ASUP CR"

	DeployedComponents  = "Deployed all the connector components"
	RegisteredWithAstra = "Registered with Astra"
)
