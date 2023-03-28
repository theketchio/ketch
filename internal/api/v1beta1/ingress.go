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
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IngressControllerType is a type of an ingress controller.
type IngressControllerType string

func (t IngressControllerType) String() string { return string(t) }

const (
	TraefikIngressControllerType IngressControllerType = "traefik"
	IstioIngressControllerType   IngressControllerType = "istio"
	NginxIngressControllerType   IngressControllerType = "nginx"

	IngressConfigmapNamespace = "default"
	IngressConfigmapName      = "ketch-ingress"
)

// IngressControllerSpec contains configuration for an ingress controller.
type IngressControllerSpec struct {
	ClassName       string                `json:"className,omitempty"`
	ServiceEndpoint string                `json:"serviceEndpoint,omitempty"`
	IngressType     IngressControllerType `json:"type,omitempty"`
	ClusterIssuer   string                `json:"clusterIssuer,omitempty"`
}

// GetIngressControllerSpec gets the ketch-ingress configmap and returns an IngressControllerSpec from the configmap's data
func GetIngressControllerSpec(ctx context.Context, client client.Client) (*IngressControllerSpec, error) {
	var configmap v1.ConfigMap
	if err := client.Get(ctx, types.NamespacedName{Name: IngressConfigmapName, Namespace: IngressConfigmapNamespace}, &configmap); err != nil {
		return nil, err
	}
	if configmap.Data == nil {
		return nil, fmt.Errorf("ingress configmap data is nil")
	}
	return NewIngressControllerSpec(configmap), nil
}

func NewIngressControllerSpec(configmap v1.ConfigMap) *IngressControllerSpec {
	controllerType := IngressControllerType(configmap.Data["type"])
	if len(controllerType) == 0 {
		controllerType = IngressControllerType(configmap.Data["ingressType"])
	}
	return &IngressControllerSpec{
		ClassName:       configmap.Data["className"],
		ServiceEndpoint: configmap.Data["serviceEndpoint"],
		IngressType:     controllerType,
		ClusterIssuer:   configmap.Data["clusterIssuer"],
	}
}
