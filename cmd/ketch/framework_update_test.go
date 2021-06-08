package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/mocks"
)

func Test_frameworkUpdate(t *testing.T) {
	clusterIssuerLe := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cert-manager.io/v1",
			"kind":       "ClusterIssuer",
			"metadata": map[string]interface{}{
				"name": "le-production",
			},
			"spec": map[string]interface{}{
				"acme": "https://acme-v02.api.letsencrypt.org/directory",
			},
		},
	}
	frontendFramework := &ketchv1.Framework{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "frontend-framework",
		},
		Spec: ketchv1.FrameworkSpec{
			NamespaceName: "frontend",
			AppQuotaLimit: 30,
			IngressController: ketchv1.IngressControllerSpec{
				ClassName:       "default-classname",
				ServiceEndpoint: "192.168.1.17",
				IngressType:     ketchv1.IstioIngressControllerType,
				ClusterIssuer:   "le-staging",
			},
		},
	}

	tests := []struct {
		name    string
		cfg     config
		options frameworkUpdateOptions

		wantFrameworkSpec ketchv1.FrameworkSpec
		wantOut           string
		wantErr           string
	}{
		{
			name: "update service endpoint",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{frontendFramework},
			},
			options: frameworkUpdateOptions{
				name:                      "frontend-framework",
				ingressServiceEndpointSet: true,
				ingressServiceEndpoint:    "192.168.1.18",
			},
			wantOut: "Successfully updated!\n",
			wantFrameworkSpec: ketchv1.FrameworkSpec{
				NamespaceName: "frontend",
				AppQuotaLimit: 30,
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "default-classname",
					ServiceEndpoint: "192.168.1.18",
					IngressType:     ketchv1.IstioIngressControllerType,
					ClusterIssuer:   "le-staging",
				},
			},
		},
		{
			name: "update ingress class name",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{frontendFramework},
			},
			options: frameworkUpdateOptions{
				name:                "frontend-framework",
				ingressClassNameSet: true,
				ingressClassName:    "traefik",
			},
			wantOut: "Successfully updated!\n",
			wantFrameworkSpec: ketchv1.FrameworkSpec{
				NamespaceName: "frontend",
				AppQuotaLimit: 30,
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "traefik",
					ServiceEndpoint: "192.168.1.17",
					IngressType:     ketchv1.IstioIngressControllerType,
					ClusterIssuer:   "le-staging",
				},
			},
		},
		{
			name: "update namespace name",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{frontendFramework},
			},
			options: frameworkUpdateOptions{
				name:         "frontend-framework",
				namespaceSet: true,
				namespace:    "new-namespace",
			},
			wantOut: "Successfully updated!\n",
			wantFrameworkSpec: ketchv1.FrameworkSpec{
				NamespaceName: "new-namespace",
				AppQuotaLimit: 30,
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "default-classname",
					ServiceEndpoint: "192.168.1.17",
					IngressType:     ketchv1.IstioIngressControllerType,
					ClusterIssuer:   "le-staging",
				},
			},
		},
		{
			name: "update app quota",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{frontendFramework},
			},
			options: frameworkUpdateOptions{
				name:             "frontend-framework",
				appQuotaLimitSet: true,
				appQuotaLimit:    50,
			},
			wantOut: "Successfully updated!\n",
			wantFrameworkSpec: ketchv1.FrameworkSpec{
				NamespaceName: "frontend",
				AppQuotaLimit: 50,
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "default-classname",
					ServiceEndpoint: "192.168.1.17",
					IngressType:     ketchv1.IstioIngressControllerType,
					ClusterIssuer:   "le-staging",
				},
			},
		},
		{
			name: "update ingress type",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{frontendFramework},
			},
			options: frameworkUpdateOptions{
				name:           "frontend-framework",
				ingressTypeSet: true,
				ingressType:    traefik,
			},
			wantOut: "Successfully updated!\n",
			wantFrameworkSpec: ketchv1.FrameworkSpec{
				NamespaceName: "frontend",
				AppQuotaLimit: 30,
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "default-classname",
					ServiceEndpoint: "192.168.1.17",
					IngressType:     ketchv1.TraefikIngressControllerType,
					ClusterIssuer:   "le-staging",
				},
			},
		},
		{
			name: "update cluster issuer",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{frontendFramework},
				DynamicClientObjects: []runtime.Object{clusterIssuerLe},
			},
			options: frameworkUpdateOptions{
				name:                    "frontend-framework",
				ingressClusterIssuerSet: true,
				ingressClusterIssuer:    "le-production",
			},
			wantOut: "Successfully updated!\n",
			wantFrameworkSpec: ketchv1.FrameworkSpec{
				NamespaceName: "frontend",
				AppQuotaLimit: 30,
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "default-classname",
					ServiceEndpoint: "192.168.1.17",
					IngressType:     ketchv1.IstioIngressControllerType,
					ClusterIssuer:   "le-production",
				},
			},
		},
		{
			name: "err - no framework",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{},
			},
			options: frameworkUpdateOptions{
				name:             "frontend-framework",
				appQuotaLimitSet: true,
				appQuotaLimit:    50,
			},
			wantErr: `failed to get the framework: frameworks.theketch.io "frontend-framework" not found`,
		},
		{
			name: "error - no cluster issuer",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{frontendFramework},
			},
			options: frameworkUpdateOptions{
				name:                    "frontend-framework",
				ingressClusterIssuerSet: true,
				ingressClusterIssuer:    "le-production",
			},
			wantErr: "cluster issuer not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			err := frameworkUpdate(context.Background(), tt.cfg, tt.options, out)
			wantErr := len(tt.wantErr) > 0
			if wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.Equal(t, tt.wantOut, out.String())
			gotFramework := ketchv1.Framework{}
			err = tt.cfg.Client().Get(context.Background(), types.NamespacedName{Name: tt.options.name}, &gotFramework)
			require.Nil(t, err)
			require.Equal(t, tt.wantFrameworkSpec, gotFramework.Spec)
		})
	}
}
