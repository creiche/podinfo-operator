/*
Copyright 2023.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MyAppResourceSpec defines the desired state of MyAppResource
type MyAppResourceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	ReplicaCount int32                      `json:"replicaCount"`
	Resources    MyAppResourceSpecResources `json:"resources"`
	Image        MyAppResourceSpecImage     `json:"image"`
	Ui           MyAppResourceSpecUi        `json:"ui"`
	Redis        MyAppResourceSpecRedis     `json:"redis"`
}

type MyAppResourceSpecResources struct {
	CpuRequest  string `json:"cpuRequest"`
	MemoryLimit string `json:"memoryLimit"`
}

type MyAppResourceSpecImage struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
}

type MyAppResourceSpecUi struct {
	Color   string `json:"color"`
	Message string `json:"message"`
}

type MyAppResourceSpecRedis struct {
	Enabled bool `json:"enabled"`
	//+kubebuilder:default:=latest
	Tag string `json:"tag,omitempty"`
}

// MyAppResourceStatus defines the observed state of MyAppResource
type MyAppResourceStatus struct {
	Conditions []metav1.Condition `json:"conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MyAppResource is the Schema for the myappresources API
type MyAppResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MyAppResourceSpec   `json:"spec,omitempty"`
	Status MyAppResourceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MyAppResourceList contains a list of MyAppResource
type MyAppResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MyAppResource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MyAppResource{}, &MyAppResourceList{})
}
