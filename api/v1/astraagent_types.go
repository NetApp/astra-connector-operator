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

type HttpProxyClient struct {
	Image string `json:"image"`
}

type EchoClient struct {
	Image string `json:"image"`
	Size  int32  `json:"size"`
}

type Astra struct {
	Register    bool   `json:"register"`
	Token       string `json:"token"`
	ClusterName string `json:"clusterName"`
	AccountID   string `json:"accountId"`
	CloudType   string `json:"cloudType"`
}

type NatssyncClient struct {
	Image             string `json:"image"`
	CloudBridgeURL    string `json:"cloud-bridge-url"`
	SkipTLSValidation string `json:"skipTLSValidation"`
	HostAlias         bool   `json:"hostalias"`
	HostAliasIP       string `json:"hostaliasIP"`
}

type Nats struct {
	Size  int32  `json:"size"`
	Image string `json:"image"`
}

// AstraAgentSpec defines the desired state of AstraAgent
type AstraAgentSpec struct {
	Namespace       string          `json:"namespace"`
	NatssyncClient  NatssyncClient  `json:"natssync-client"`
	HttpProxyClient HttpProxyClient `json:"httpproxy-client"`
	EchoClient      EchoClient      `json:"echo-client"`
	Nats            Nats            `json:"nats"`
	Astra           Astra           `json:"astra"`
}

// AstraAgentStatus defines the observed state of AstraAgent
type AstraAgentStatus struct {
	Nodes          []string             `json:"nodes"`
	NatssyncClient NatssyncClientStatus `json:"natssync-client"`
}

// NatssyncClientStatus defines the observed state of NatssyncClient
type NatssyncClientStatus struct {
	Version    string `json:"version"`
	State      string `json:"state"`
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
