/*

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
	"crypto/md5"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// InterfaceSpec defines the desired state of Interface
type InterfaceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Address is an example field of Interface. Edit Interface_types.go to remove/update
	Units []Unit `json:"units,omitempty"`
	// InterfaceIdentifier specifies the interface name string
	InterfaceIdentifier string `json:"interfaceIdentifier,omitempty"`
	// UsedBy is a list of devices using this interface
	UsedBy []string `json:"usedBy,omitempty"`
}

// Unit is a logical entitiy on an Interface
type Unit struct {
	// Id is the identifier of the logical Unit
	ID int `json:"id,omitempty"`
	// Addresses is a list of addresses
	Addresses []string `json:"addresses,omitempty"`
}

// Address is a representation of an IPv4/v6 address in CIDR format
type Address struct {
	string `json:"addresses,omitempty"`
}

// InterfaceTemplateSpec describes the data a Interface should have when created from a template
type InterfaceTemplateSpec struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Specification of the desired behavior of the pod.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Spec InterfaceSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InterfaceTemplate describes a template for creating copies of a predefined interfaces.
type InterfaceTemplate struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Template defines the pods that will be created from this Interface template.
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Template InterfaceTemplateSpec `json:"template,omitempty" protobuf:"bytes,2,opt,name=template"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InterfaceTemplateList is a list of InterfaceTemplateList.
type InterfaceTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// List of interface templates
	Items []InterfaceTemplate `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// InterfaceFieldSelector represents interface object
type InterfaceFieldSelector struct {
	// Interface name: required
	// +optional
	InterfaceName string `json:"containerName,omitempty" protobuf:"bytes,1,opt,name=interfaceName"`
}

// InterfaceStatus defines the observed state of Interface
type InterfaceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// Interface is the Schema for the interfaces API
type Interface struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InterfaceSpec   `json:"spec,omitempty"`
	Status InterfaceStatus `json:"status,omitempty"`
}

// InterfaceReference holds a reference to an interace object
type InterfaceReference struct {
	Namespace           string        `json:"namespace,omitempty"`
	Name                string        `json:"name,omitempty"`
	InterfaceIdentifier string        `json:"interfaceIdentifier,omitempty"`
	UID                 types.UID     `json:"uid,omitempty"`
	CommitStatus        *CommitStatus `json:"commitStatus,omitempty"`
	ConfigHash          string        `json:"configHash,omitempty"`
}

func (i *Interface) GetReference() *InterfaceReference {
	return &InterfaceReference{
		Name:                i.GetName(),
		Namespace:           i.GetNamespace(),
		InterfaceIdentifier: i.Spec.InterfaceIdentifier,
		UID:                 i.GetUID(),
		ConfigHash:          i.Hash(),
	}
}

// +kubebuilder:object:root=true

// InterfaceList contains a list of Interface
type InterfaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Interface `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Interface{}, &InterfaceList{}, &InterfaceTemplate{}, &InterfaceTemplateList{})
}

func (i *Interface) Hash() string {
	arrBytes := []byte{}
	jsonBytes, _ := json.Marshal(i.Spec)
	arrBytes = append(arrBytes, jsonBytes...)
	return fmt.Sprintf("%x", md5.Sum(arrBytes))
}
