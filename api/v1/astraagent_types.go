/*
 * Copyright (c) 2022. NetApp, Inc. All Rights Reserved.
 */

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//+kubebuilder:validation:Optional
type HttpProxyClient struct {
	Image string `json:"image,omitempty"`
	Size  int32  `json:"size,omitempty"`
}

//+kubebuilder:validation:Optional
type EchoClient struct {
	Image string `json:"image,omitempty"`
	Size  int32  `json:"size,omitempty"`
}

type Astra struct {
	Unregister  bool   `json:"unregister,omitempty"`
	Token       string `json:"token,omitempty"`
	ClusterName string `json:"clusterName"`
	AccountID   string `json:"accountId"`
	AcceptEULA  bool   `json:"acceptEULA"`
	OldAuth     bool   `json:"oldAuth,omitempty"`
}

//+kubebuilder:validation:Optional
type NatssyncClient struct {
	Image             string `json:"image,omitempty"`
	Size              int32  `json:"size,omitempty"`
	CloudBridgeURL    string `json:"cloud-bridge-url,omitempty"`
	SkipTLSValidation bool   `json:"skipTLSValidation,omitempty"`
	HostAlias         bool   `json:"hostalias,omitempty"`
	HostAliasIP       string `json:"hostaliasIP,omitempty"`
}

//+kubebuilder:validation:Optional
type Nats struct {
	Size  int32  `json:"size,omitempty"`
	Image string `json:"image,omitempty"`
}

//+kubebuilder:validation:Optional
type ImageRegistry struct {
	Name   string `json:"name,omitempty"`
	Secret string `json:"secret,omitempty"`
}

// AstraAgentSpec defines the desired state of AstraAgent
type AstraAgentSpec struct {
	NatssyncClient  NatssyncClient  `json:"natssync-client,omitempty"`
	HttpProxyClient HttpProxyClient `json:"httpproxy-client,omitempty"`
	EchoClient      EchoClient      `json:"echo-client,omitempty"`
	Nats            Nats            `json:"nats,omitempty"`
	Astra           Astra           `json:"astra"`
	ImageRegistry   ImageRegistry   `json:"imageRegistry,omitempty"`
}

// AstraAgentStatus defines the observed state of AstraAgent
type AstraAgentStatus struct {
	Nodes          []string             `json:"nodes"`
	NatssyncClient NatssyncClientStatus `json:"natssync-client"`
}

// NatssyncClientStatus defines the observed state of NatssyncClient
type NatssyncClientStatus struct {
	Registered string `json:"registered"`
	LocationID string `json:"locationID"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Registered",type=string,JSONPath=`.status.natssync-client.registered`
//+kubebuilder:printcolumn:name="LocationID",type=string,JSONPath=`.status.natssync-client.locationID`

// AstraAgent is the Schema for the astraagents API
type AstraAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AstraAgentSpec   `json:"spec,omitempty"`
	Status AstraAgentStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AstraAgentList contains a list of AstraAgent
type AstraAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AstraAgent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AstraAgent{}, &AstraAgentList{})
}
