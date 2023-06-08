/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Astra struct {
	Unregister  bool   `json:"unregister,omitempty"`
	Token       string `json:"token,omitempty"`
	ClusterName string `json:"clusterName,omitempty"`
	AccountID   string `json:"accountId"`
	// +kubebuilder:validation:Optional
	CloudID string `json:"cloudID"`
	// +kubebuilder:validation:Optional
	ClusterID string `json:"clusterID"`
	// +kubebuilder:validation:Optional
	StorageClassName  string `json:"storageClassName"`
	SkipTLSValidation bool   `json:"skipTLSValidation,omitempty"`
}

// +kubebuilder:validation:Optional
type NatssyncClient struct {
	Image          string `json:"image,omitempty"`
	Size           int32  `json:"size,omitempty"`
	CloudBridgeURL string `json:"cloud-bridge-url,omitempty"`
	HostAlias      bool   `json:"hostalias,omitempty"`
	HostAliasIP    string `json:"hostaliasIP,omitempty"`
}

// +kubebuilder:validation:Optional
type Nats struct {
	Size  int32  `json:"size,omitempty"`
	Image string `json:"image,omitempty"`
}

// +kubebuilder:validation:Optional
type AstraConnect struct {
	Size  int32  `json:"size,omitempty"`
	Image string `json:"image,omitempty"`
}

// +kubebuilder:validation:Optional
type Neptune struct {
	Size  int32  `json:"size,omitempty"`
	Image string `json:"image,omitempty"`
}

// +kubebuilder:validation:Optional
type ConnectorSpec struct {
	Astra          Astra          `json:"astra"`
	NatssyncClient NatssyncClient `json:"natssync-client,omitempty"`
	Nats           Nats           `json:"nats,omitempty"`
	AstraConnect   AstraConnect   `json:"astra-connect,omitempty"`
}

// AstraConnectorSpec defines the desired state of AstraConnector
type AstraConnectorSpec struct {
	ConnectorSpec ConnectorSpec `json:"connector"`
	Neptune       Neptune       `json:"neptune"`
	ImageRegistry ImageRegistry `json:"imageRegistry,omitempty"`
}

// AstraConnectorStatus defines the observed state of AstraConnector
type AstraConnectorStatus struct {
	Nodes          []string             `json:"nodes"`
	NatssyncClient NatssyncClientStatus `json:"natssync-client"`
}

// NatssyncClientStatus defines the observed state of NatssyncClient
type NatssyncClientStatus struct {
	Registered       string `json:"registered"` //todo cluster vs connector registered
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
//+kubebuilder:printcolumn:name="Registered",type=string,JSONPath=`.status.natssync-client.registered`
//+kubebuilder:printcolumn:name="AstraConnectorID",type=string,JSONPath=`.status.natssync-client.astraConnectorID`
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.natssync-client.status`

// AstraConnector is the Schema for the astraconnectors API
type AstraConnector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AstraConnectorSpec   `json:"spec,omitempty"`
	Status AstraConnectorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AstraConnectorList contains a list of AstraConnector
type AstraConnectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AstraConnector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AstraConnector{}, &AstraConnectorList{})
}
