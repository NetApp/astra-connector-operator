//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright 2022 NetApp, Inc. All Rights Reserved.
*/

// Code generated by controller-gen. DO NOT EDIT.

package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Astra) DeepCopyInto(out *Astra) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Astra.
func (in *Astra) DeepCopy() *Astra {
	if in == nil {
		return nil
	}
	out := new(Astra)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AstraConnect) DeepCopyInto(out *AstraConnect) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AstraConnect.
func (in *AstraConnect) DeepCopy() *AstraConnect {
	if in == nil {
		return nil
	}
	out := new(AstraConnect)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AstraConnector) DeepCopyInto(out *AstraConnector) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AstraConnector.
func (in *AstraConnector) DeepCopy() *AstraConnector {
	if in == nil {
		return nil
	}
	out := new(AstraConnector)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AstraConnector) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AstraConnectorList) DeepCopyInto(out *AstraConnectorList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AstraConnector, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AstraConnectorList.
func (in *AstraConnectorList) DeepCopy() *AstraConnectorList {
	if in == nil {
		return nil
	}
	out := new(AstraConnectorList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AstraConnectorList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AstraConnectorSpec) DeepCopyInto(out *AstraConnectorSpec) {
	*out = *in
	out.Astra = in.Astra
	out.NatsSyncClient = in.NatsSyncClient
	out.Nats = in.Nats
	out.AstraConnect = in.AstraConnect
	out.ImageRegistry = in.ImageRegistry
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AstraConnectorSpec.
func (in *AstraConnectorSpec) DeepCopy() *AstraConnectorSpec {
	if in == nil {
		return nil
	}
	out := new(AstraConnectorSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AstraConnectorStatus) DeepCopyInto(out *AstraConnectorStatus) {
	*out = *in
	if in.Nodes != nil {
		in, out := &in.Nodes, &out.Nodes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	out.NatsSyncClient = in.NatsSyncClient
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AstraConnectorStatus.
func (in *AstraConnectorStatus) DeepCopy() *AstraConnectorStatus {
	if in == nil {
		return nil
	}
	out := new(AstraConnectorStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ImageRegistry) DeepCopyInto(out *ImageRegistry) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ImageRegistry.
func (in *ImageRegistry) DeepCopy() *ImageRegistry {
	if in == nil {
		return nil
	}
	out := new(ImageRegistry)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Nats) DeepCopyInto(out *Nats) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Nats.
func (in *Nats) DeepCopy() *Nats {
	if in == nil {
		return nil
	}
	out := new(Nats)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NatsSyncClient) DeepCopyInto(out *NatsSyncClient) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NatsSyncClient.
func (in *NatsSyncClient) DeepCopy() *NatsSyncClient {
	if in == nil {
		return nil
	}
	out := new(NatsSyncClient)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NatsSyncClientStatus) DeepCopyInto(out *NatsSyncClientStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NatsSyncClientStatus.
func (in *NatsSyncClientStatus) DeepCopy() *NatsSyncClientStatus {
	if in == nil {
		return nil
	}
	out := new(NatsSyncClientStatus)
	in.DeepCopyInto(out)
	return out
}
