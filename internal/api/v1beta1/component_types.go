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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ComponentSpec defines the desired state of Component
type ComponentSpec struct {
	// Workload represents an identifier to a workload type
	Workload WorkloadTypeDescriptor `json:"workload"`

	// Schematic
	// +optional
	Schematic Schematic `json:"schematic"`
}

// WorkloadTypeDescriptor defines a workload
type WorkloadTypeDescriptor struct {
	// Default is whether the component is applied by default when creating an Application
	Default bool `json:"default"`

	// Definition is a reference to Workload definition via Group, Version, Kind
	Definition WorkloadGVK `json:"workloadGVK"`
}

// WorkloadGVK defines the version and kind for a workload
type WorkloadGVK struct {
	// Group defines the object's group
	Group string `json:"group,omitempty"`

	// APIVersion defines the versioned schema of this representation of an object.
	APIVersion string `json:"apiVersion,omitempty"`

	// Kind is a string value representing the REST resource this object represents.
	Kind string `json:"kind,omitempty"`
}

type Parameter struct {
	Name         string   `json:"name"`
	TemplateName string   `json:"templateName"`
	Required     bool     `json:"required"`
	Type         string   `json:"type"`
	Paths        []string `json:"fieldPaths"`
}

type Schematic struct {
	// Kube is a template with parameter list for a certain Kubernetes workload resource as a component.
	Kube *Kube `json:"kube"`
}

type Kube struct {
	// Templates
	Templates  []Template  `json:"templates"`
	Parameters []Parameter `json:"parameters,omitempty"`
}

type Template struct {
	Name                 string `json:"name"`
	runtime.RawExtension `json:"object"`
}

// ComponentStatus defines the observed state of Component
type ComponentStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Component is the Schema for the components API
type Component struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentSpec   `json:"spec,omitempty"`
	Status ComponentStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ComponentList contains a list of Component
type ComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Component `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Component{}, &ComponentList{})
}
