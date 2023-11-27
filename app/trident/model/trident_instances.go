package model

import (
	"errors"
	"fmt"
	"net/http"

	tridentdrivers "github.com/netapp/trident/storage_drivers"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ResourceState string

const (
	StateUnknown      ResourceState = "Unknown"
	StateProvisioning               = "Provisioning"
	StateAvailable                  = "Available"
	StateNotAvailable               = "Not Available"
	StateFailed                     = "Failed"
	StateNotUsed                    = "Not Used"
)

// TridentInstance contains the BSON structure of a trident deployment in a target K8S cluster
type TridentInstance struct {
	DBID primitive.ObjectID `json:"-" bson:"_id,omitempty"`

	Metadata  Metadata `json:"metadata" bson:"metadata"`
	Type      string   `json:"type" bson:"type"`
	Version   string   `json:"version" bson:"version"`
	ID        string   `json:"id" bson:"id"` // This is the Astra cluster ID assigned by composite-compute
	AccountID string   `json:"-" bson:"accountID"`

	Spec  TridentInstanceSpec  `json:"spec,omitempty" bson:"spec,omitempty"`
	State TridentInstanceState `json:"state,omitempty" bson:"state,omitempty"`
}

func (t *TridentInstance) ResourceType() string {
	return "application/astra-tridentInstance"
}

func (t *TridentInstance) MIMEType() (string, bool) {
	return "application/astra-tridentInstance+json", false
}

type TridentInstanceSpec struct {
	SerialNumber            string        `json:"serialNumber" bson:"serialNumber"`
	ClusterName             string        `json:"clusterName" bson:"clusterName"`
	ClusterCredentialsID    string        `json:"clusterCredentialsID" bson:"clusterCredentialsID"`
	ClusterCredentials      []byte        `json:"-" bson:"-"` // Kube config bytes
	TridentOperatorImage    string        `json:"tridentOperatorImage" bson:"tridentOperatorImage"`
	TridentImage            string        `json:"tridentImage" bson:"tridentImage"`
	TridentAutosupportImage string        `json:"tridentAutosupportImage" bson:"tridentAutosupportImage"`
	ACPImage                string        `json:"acpImage" bson:"acpImage"`
	TridentDesiredState     ResourceState `json:"tridentDesiredState" bson:"tridentDesiredState"`
	ProxyURL                string        `json:"proxyURL" bson:"proxyURL"`
	NFSMountOptions         string        `json:"nfsMountOptions" bson:"nfsMountOptions"`
	NoManage                *bool         `json:"noManage" bson:"noManage"`
	DefaultStorageClass     *string       `json:"defaultStorageClass" bson:"defaultStorageClass"`
	GCP                     *GCP          `json:"gcp,omitempty" bson:"gcp,omitempty"`
	ANF                     *ANF          `json:"anf,omitempty" bson:"anf,omitempty"`
}

type TridentInstanceState struct {
	ClusterVersion          string        `json:"clusterVersion" bson:"clusterVersion"`
	ClusterState            ResourceState `json:"clusterState" bson:"clusterState"`
	TridentOperatorState    ResourceState `json:"tridentOperatorState" bson:"tridentOperatorState"`
	TridentOperatorImage    string        `json:"tridentOperatorImage" bson:"tridentOperatorImage"`
	TridentVersion          string        `json:"tridentVersion" bson:"tridentVersion"`
	TridentState            ResourceState `json:"tridentState" bson:"tridentState"`
	TridentImage            string        `json:"tridentImage" bson:"tridentImage"`
	TridentAutosupportImage string        `json:"tridentAutosupportImage" bson:"tridentAutosupportImage"`
	ACPImage                string        `json:"acpImage,omitempty" bson:"acpImage,omitempty"`
}

type GCP struct {
	CredentialsID       string                        `json:"credentialsID" bson:"credentialsID"`
	Credentials         *tridentdrivers.GCPPrivateKey `json:"-" bson:"-"` // GCP service account JSON
	ProjectNumber       string                        `json:"projectNumber" bson:"projectNumber"`
	HostProjectNumber   string                        `json:"hostProjectNumber" bson:"hostProjectNumber"`
	APIRegion           string                        `json:"apiRegion" bson:"apiRegion"`
	Network             string                        `json:"network" bson:"network"`
	DefaultStorageClass string                        `json:"defaultStorageClass" bson:"defaultStorageClass"`
	APIURL              string                        `json:"apiURL" bson:"apiURL"`
	APIAudienceURL      string                        `json:"apiAudienceURL" bson:"apiAudienceURL"`
}

type ANF struct {
	CredentialsID       string          `json:"credentialsID" bson:"credentialsID"`
	Credentials         *ANFCredentials `json:"-" bson:"-"`
	Location            string          `json:"location" bson:"location"`
	Network             string          `json:"network" bson:"network"`
	Subnet              string          `json:"subnet" bson:"subnet"`
	DefaultStorageClass string          `json:"defaultStorageClass" bson:"defaultStorageClass"`
}

type ANFCredentials struct {
	DisplayName  string `json:"displayName" bson:"displayName"`
	TenantID     string `json:"tenant" bson:"tenant"`
	ClientID     string `json:"appId" bson:"appId"`
	ClientSecret string `json:"password" bson:"password"`

	// This may eventually move out
	SubscriptionID string `json:"subscriptionId" bson:"subscriptionId"`
}

// NoManage returns true if the reconciler should not attempt to install or configure Trident
// on the target Kubernetes cluster.
func (t *TridentInstance) NoManage() bool {
	return t.Spec.NoManage != nil && *(t.Spec.NoManage)
}

func (t *TridentInstance) DefaultStorageClass() string {
	if t.Spec.DefaultStorageClass == nil {
		return ""
	} else {
		return *(t.Spec.DefaultStorageClass)
	}
}

type TridentInstances struct {
	Items []TridentInstance `json:"items"`
}

func (t *TridentInstances) MIMEType() (string, bool) {
	return "application/astra-tridentInstance+json", true
}

// Version contains the version of this service
type Version struct {
	Version string `json:"version"`
}

func (t *Version) MIMEType() (string, bool) {
	return "application/astra-trident-svc-version+json", false
}

type MIMETyper interface {
	MIMEType() (string, bool)
}

func ResponseContentType(r *http.Request, t MIMETyper) (string, error) {

	// Handle bad input
	if t == nil {
		return "", errors.New("object may not be nil")
	}
	if r == nil {
		return "", errors.New("http request may not be nil")
	}

	requestAcceptMIMEType := r.Header.Get("Accept")
	preferredMIMEType, isCollection := t.MIMEType()

	switch requestAcceptMIMEType {
	case "", "*/*", "application/json":
		return "application/json", nil
	case preferredMIMEType:
		if isCollection {
			return "application/json", nil
		} else {
			return preferredMIMEType, nil
		}
	default:
		return "", fmt.Errorf("invalid Accept header (%s)", requestAcceptMIMEType)
	}
}
