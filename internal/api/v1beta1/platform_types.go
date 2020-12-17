package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func init() {
	SchemeBuilder.Register(&Platform{}, &PlatformList{})
}

// PlatformSpec contains the specification for the platform object
type PlatformSpec struct {

	// Image is the name of the image that implements the platform
	Image string `json:"image,omitempty"`

	// Description human readable information about the platform.
	// +kubebuilder:validation:MaxLength=140
	Description string `json:"description,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Name",type=string,JSONPath=`.metadata.name`
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.image`
// +kubebuilder:printcolumn:name="Description",type=string,JSONPath=`.spec.description`

// Platform an image that is used to build an app. See https://learn.shipa.io/docs/platforms-1
type Platform struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PlatformSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

type PlatformList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Platform `json:"items"`
}
