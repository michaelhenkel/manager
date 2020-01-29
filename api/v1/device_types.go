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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DeviceSpec defines the desired state of Device
type DeviceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// Vendor defines the device vendor
	Vendor string `json:"vendor,omitempty"`
	//InterfacesSelector defines a list of Interfaces to be used by the device
	InterfaceSelector []map[string]string `json:"interfaceSelector,omitempty"`
}

//CommitStatus defines the state of the configuration
type CommitStatus string

const (
	//COMMITSUCCESS means config is correctly applied
	COMMITSUCCESS CommitStatus = "successCommit"
	//FAILED means config failed
	FAILED CommitStatus = "failed"
	//PENDINGCREATE means config commit is in progress to create a new resource
	PENDINGCREATE CommitStatus = "pendingCreation"
	//PENDINGDELETE means config commit is in progress to delete a resource
	PENDINGDELETE CommitStatus = "pendingDeletion"
	//PENDINGUPDATE means config commit is in progress to change a resource
	PENDINGUPDATE CommitStatus = "pendingUpdate"
	//UPDATESUCCESS means config is correctly applied
	UPDATESUCCESS CommitStatus = "successUpdate"
	//CREATESUCCESS means config is correctly applied
	CREATESUCCESS CommitStatus = "successCreate"
	//DELETESUCCESS means config is correctly applied
	DELETESUCCESS CommitStatus = "successDelete"
	//UPDATEFAIL means config is correctly applied
	UPDATEFAIL CommitStatus = "failUpdate"
	//CREATEFAIL means config is correctly applied
	CREATEFAIL CommitStatus = "failCreate"
	//DELETEFAIL means config is correctly applied
	DELETEFAIL CommitStatus = "failDelete"
)

// DeviceStatus defines the observed state of Device
type DeviceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Interfaces map[string]*DeviceInterfaceStatus `json:"interfaces,omitempty"`
}

// DeviceInterfaceStatus defines the status of an interface on the device
type DeviceInterfaceStatus struct {
	InterfaceRef *InterfaceReference `json:"interfaceRef,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Device is the Schema for the devices API
type Device struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeviceSpec   `json:"spec,omitempty"`
	Status DeviceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DeviceList contains a list of Device
type DeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Device `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Device{}, &DeviceList{})
}
