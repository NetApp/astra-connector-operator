/*
 * Copyright (c) 2024. NetApp, Inc. All Rights Reserved.
 */

package v1

import (
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AstraConnectorSpec defines the desired state of AstraConnector
type AstraConnectorSpec struct {
	AccountId         string `json:"accountId"`
	ApiTokenSecretRef string `json:"apiTokenSecretRef,omitempty"`
	AstraControlUrl   string `json:"astraControlUrl,omitempty"`
	// +kubebuilder:validation:Optional
	CloudId string `json:"cloudId"`
	// +kubebuilder:validation:Optional
	ClusterId string `json:"clusterId"`
	// +kubebuilder:validation:Optional
	SkipTLSValidation bool `json:"skipTLSValidation,omitempty"`

	Image string `json:"image,omitempty"`
	// +kubebuilder:validation:Optional
	HostAliasIP string `json:"hostAliasIP,omitempty"`
	// +kubebuilder:default:=1
	Replicas int32 `json:"replicas,omitempty"`

	ImageRegistry ImageRegistry `json:"imageRegistry,omitempty"`

	// SkipPreCheck determines if you want to skip pre-checks and go ahead with the installation.
	// +kubebuilder:default:=false
	SkipPreCheck bool `json:"skipPreCheck"`

	// Labels any additional labels wanted to be added to resources
	Labels map[string]string `json:"labels"`
}

// AstraConnectorStatus defines the observed state of AstraConnector
type AstraConnectorStatus struct {
	Version string `json:"version"`
	Status  string `json:"status"`
}

// +kubebuilder:validation:Optional

type ImageRegistry struct {
	Name   string `json:"name,omitempty"`
	Secret string `json:"secret,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="AstraConnectorVersion",type=string,JSONPath=`.status.version`
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`

// AstraConnector is the Schema for the astraconnectors API
// +kubebuilder:subresource:status
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
