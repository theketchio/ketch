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

// +kubebuilder:validation:Enum=traefik;istio;nginx

// IngressControllerType is a type of an ingress controller.
type IngressControllerType string

func (t IngressControllerType) String() string { return string(t) }

const (
	TraefikIngressControllerType IngressControllerType = "traefik"
	IstioIngressControllerType   IngressControllerType = "istio"
	NginxIngressControllerType   IngressControllerType = "nginx"
)

const IngressConfigmapName = "ketch-ingress"

// IngressControllerSpec contains configuration for an ingress controller.
type IngressControllerSpec struct {
	ClassName       string                `json:"className,omitempty"`
	ServiceEndpoint string                `json:"serviceEndpoint,omitempty"`
	IngressType     IngressControllerType `json:"type"`
	ClusterIssuer   string                `json:"clusterIssuer,omitempty"`
}
