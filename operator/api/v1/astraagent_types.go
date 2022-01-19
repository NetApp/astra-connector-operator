/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AstraAgentSpec defines the desired state of AstraAgent
type AstraAgentSpec struct {
	// Size is the size of the AstraAgent deployment
	Size int32 `json:"size"`
}

// AstraAgentStatus defines the observed state of AstraAgent
type AstraAgentStatus struct {
	// Nodes are the names of the AstraAgent pods
	Nodes []string `json:"nodes"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

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
