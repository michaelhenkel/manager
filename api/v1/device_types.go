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
	corev1 "k8s.io/api/core/v1"
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
	//SUCCESS means config is correctly applied
	SUCCESS CommitStatus = "success"
	//FAILED means config failed
	FAILED CommitStatus = "failed"
	//PENDING means config commit is in progress
	PENDING CommitStatus = "pending"
)

// DeviceStatus defines the observed state of Device
type DeviceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Interfaces []*DeviceInterfaceStatus `json:"interfaces,omitempty"`
}

// DeviceInterfaceStatus defines the status of an interface on the device
type DeviceInterfaceStatus struct {
	InterfaceRef *corev1.ObjectReference `json:"interfaceRef,omitempty"`
	CommitStatus *CommitStatus           `json:"commitStatus,omitempty"`
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
