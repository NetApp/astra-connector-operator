/*
 * Copyright (c) 2024. NetApp, Inc. All Rights Reserved.
 */

package v1

import (
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AutoSupport defines how the customer interacts with NetApp ActiveIQ.
type AutoSupport struct {
	// Enrolled determines if you want to send anonymous data to NetApp for support purposes.
	// +kubebuilder:default:=true
	Enrolled bool `json:"enrolled"`

	// URL determines where the anonymous data will be sent
	// +kubebuilder:default:="https://stagesupport.netapp.com/put/AsupPut"
	URL string `json:"url,omitempty"`
}

// AstraNeptuneSpec defines the desired state of AstraNeptune
type AstraNeptuneSpec struct {
	ImageRegistry ImageRegistry `json:"imageRegistry,omitempty"`
	Image         string        `json:"image,omitempty"`

	// AutoSupport indicates willingness to participate in NetApp's proactive support application, NetApp Active IQ.
	// An internet connection is required (port 442) and all support data is anonymized.
	// The default election is true and indicates support data will be sent to NetApp.
	// An empty or blank election is the same as a default election.
	// Air gapped installations should enter false.
	// +kubebuilder:default={"enrolled":true, "url":"https://stagesupport.netapp.com/put/AsupPut"}
	AutoSupport AutoSupport `json:"autoSupport"`

	// SkipPreCheck determines if you want to skip pre-checks and go ahead with the installation.
	// +kubebuilder:default:=false
	SkipPreCheck bool `json:"skipPreCheck"`

	// Labels any additional labels wanted to be added to resources
	Labels map[string]string `json:"labels"`
}

// AstraNeptuneStatus defines the observed state of AstraNeptune
type AstraNeptuneStatus struct {
	Version string `json:"version"`
	Status  string `json:"status"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="AstraNeptuneVersion",type=string,JSONPath=`.status.version`
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`

// AstraNeptune is the Schema for the astraneptunes API
// +kubebuilder:subresource:status
type AstraNeptune struct {
	metaV1.TypeMeta   `json:",inline"`
	metaV1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AstraNeptuneSpec   `json:"spec,omitempty"`
	Status AstraNeptuneStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AstraNeptuneList contains a list of AstraNeptune
type AstraNeptuneList struct {
	metaV1.TypeMeta `json:",inline"`
	metaV1.ListMeta `json:"metadata,omitempty"`
	Items           []AstraConnector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AstraNeptune{}, &AstraNeptuneList{})
}
