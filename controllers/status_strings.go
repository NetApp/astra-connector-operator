package controllers

const (
	CreateStatefulSet    = "Creating StatefulSet %s/%s"
	CreateRoleBinding    = "Creating RoleBinding %s/%s"
	CreateServiceAccount = "Creating ServiceAccount %s/%s"
	CreateConfigMap      = "Creating ConfigMap %s/%s"
	CreateDeployment     = "Creating Deployment %s/%s"
	UpdateDeployment     = "Updating Deployment %s/%s"
	CreateService        = "Creating Service %s/%s"
	CreateRole           = "Creating Role %s/%s"

	ErrorCreateStatefulSets    = "Error creating StatefulSets"
	ErrorCreateRoleBindings    = "Error creating RoleBindings"
	ErrorCreateServiceAccounts = "Error creating ServiceAccounts"
	ErrorCreateConfigMaps      = "Error creating ConfigMaps"
	ErrorCreateDeployments     = "Error creating Deployments"
	ErrorCreateService         = "Error creating Services"
	ErrorCreateRoles           = "Error creating Roles"

	FailedFinalizerAdd      = "Failed to add finalizer"
	FailedFinalizerRemove   = "Failed to remove finalizer"
	FailedAstraConnectorGet = "Failed to get AstraConnector"
	EULANA                  = "EULA not accepted"

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
