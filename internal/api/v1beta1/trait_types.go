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
	//runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// TraitSpec defines the desired state of Trait
type TraitSpec struct {
	AppliesToWorkloads []string      `json:"appliesToWorkloads"`
	ConflictsWith      []string      `json:"conflictsWith"'`
	DefinitionRef      DefinitionRef `json:"definitionRef"`
}

type DefinitionRef struct {
	Name       string `json:"name,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
}

// TraitStatus defines the observed state of Trait
type TraitStatus struct {
	// ConditionedStatus reflects the observed status of a resource
	//runtimev1.ConditionedStatus `json:",inline"`
	// ConfigMapRef refer to a ConfigMap which contains OpenAPI V3 JSON schema of Component parameters.
	ConfigMapRef string `json:"configMapRef,omitempty"`
	// LatestRevision of the component definition
	// +optional
	LatestRevision *common.Revision `json:"latestRevision,omitempty"`
}

// TOOD - move to common types
// Revision has name and revision number
type Revision struct {
	Name     string `json:"name"`
	Revision int64  `json:"revision"`

	// RevisionHash record the hash value of the spec of ApplicationRevision object.
	RevisionHash string `json:"revisionHash,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Trait is the Schema for the traits API
type Trait struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TraitSpec   `json:"spec,omitempty"`
	Status TraitStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TraitList contains a list of Trait
type TraitList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Trait `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Trait{}, &TraitList{})
}
