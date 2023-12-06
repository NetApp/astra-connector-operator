/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package v1

import (
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Astra struct {
	AccountId string `json:"accountId"`
	// +kubebuilder:validation:Optional
	CloudId string `json:"cloudId"`
	// +kubebuilder:validation:Optional
	ClusterId   string `json:"clusterId"`
	ClusterName string `json:"clusterName,omitempty"`
	// +kubebuilder:validation:Optional
	StorageClassName  string `json:"storageClassName"`
	SkipTLSValidation bool   `json:"skipTLSValidation,omitempty"`
	TokenRef          string `json:"tokenRef,omitempty"`
	Unregister        bool   `json:"unregister,omitempty"`
}

// AutoSupport defines how the customer interacts with NetApp ActiveIQ.
type AutoSupport struct {
	// Enrolled determines if you want to send anonymous data to NetApp for support purposes.
	// +kubebuilder:default:=true
	Enrolled bool `json:"enrolled"`

	// URL determines where the anonymous data will be sent
	// +kubebuilder:default:="https://216.240.31.151/put/AsupPut-setenv"
	URL string `json:"url,omitempty"`
}

type NatsSyncClient struct {
	CloudBridgeURL string `json:"cloudBridgeURL,omitempty"`
	// +kubebuilder:validation:Optional
	Image string `json:"image,omitempty"`
	// +kubebuilder:validation:Optional
	HostAliasIP string `json:"hostAliasIP,omitempty"`
	// +kubebuilder:validation:Optional
	Replicas int32 `json:"replicas,omitempty"`
}

// +kubebuilder:validation:Optional

type Nats struct {
	Image    string `json:"image,omitempty"`
	Replicas int32  `json:"replicas,omitempty"`
}

// +kubebuilder:validation:Optional

type AstraConnect struct {
	Image    string `json:"image,omitempty"`
	Replicas int32  `json:"replicas,omitempty"`
}

// +kubebuilder:validation:Optional

type Neptune struct {
	Image string `json:"image,omitempty"`
}

// AstraConnectorSpec defines the desired state of AstraConnector
type AstraConnectorSpec struct {
	Astra          Astra          `json:"astra"`
	NatsSyncClient NatsSyncClient `json:"natsSyncClient,omitempty"`
	Nats           Nats           `json:"nats,omitempty"`
	AstraConnect   AstraConnect   `json:"astraConnect,omitempty"`
	Neptune        Neptune        `json:"neptune"`
	ImageRegistry  ImageRegistry  `json:"imageRegistry,omitempty"`

	// AutoSupport indicates willingness to participate in NetApp's proactive support application, NetApp Active IQ.
	// An internet connection is required (port 442) and all support data is anonymized.
	// The default election is true and indicates support data will be sent to NetApp.
	// An empty or blank election is the same as a default election.
	// Air gapped installations should enter false.
	// +kubebuilder:validation:Required
	AutoSupport AutoSupport `json:"autoSupport"`
}

// AstraConnectorStatus defines the observed state of AstraConnector
type AstraConnectorStatus struct {
	Nodes          []string             `json:"nodes"`
	NatsSyncClient NatsSyncClientStatus `json:"natsSyncClient"`
}

// NatsSyncClientStatus defines the observed state of NatsSyncClient
type NatsSyncClientStatus struct {
	Registered       string `json:"registered"` //todo cluster vs connector registered
	AstraClusterId   string `json:"astraClusterID"`
	AstraConnectorID string `json:"astraConnectorID"`
	Status           string `json:"status"`
}

// +kubebuilder:validation:Optional

type ImageRegistry struct {
	Name   string `json:"name,omitempty"`
	Secret string `json:"secret,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Registered",type=string,JSONPath=`.status.natsSyncClient.registered`
//+kubebuilder:printcolumn:name="AstraClusterID",type=string,JSONPath=`.status.natsSyncClient.astraClusterID`
//+kubebuilder:printcolumn:name="AstraConnectorID",type=string,JSONPath=`.status.natsSyncClient.astraConnectorID`
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.natsSyncClient.status`

// AstraConnector is the Schema for the astraconnectors API
type AstraConnector struct {
	metaV1.TypeMeta   `json:",inline"`
	metaV1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AstraConnectorSpec   `json:"spec,omitempty"`
	Status AstraConnectorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AstraConnectorList contains a list of AstraConnector
type AstraConnectorList struct {
	metaV1.TypeMeta `json:",inline"`
	metaV1.ListMeta `json:"metadata,omitempty"`
	Items           []AstraConnector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AstraConnector{}, &AstraConnectorList{})
}
