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

func Test_poolUpdate(t *testing.T) {
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
	frontendPool := &ketchv1.Pool{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "frontend-pool",
		},
		Spec: ketchv1.PoolSpec{
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
		options poolUpdateOptions

		wantPoolSpec ketchv1.PoolSpec
		wantOut      string
		wantErr      string
	}{
		{
			name: "update service endpoint",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{frontendPool},
			},
			options: poolUpdateOptions{
				name:                      "frontend-pool",
				ingressServiceEndpointSet: true,
				ingressServiceEndpoint:    "192.168.1.18",
			},
			wantOut: "Successfully updated!\n",
			wantPoolSpec: ketchv1.PoolSpec{
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
				CtrlClientObjects: []runtime.Object{frontendPool},
			},
			options: poolUpdateOptions{
				name:                "frontend-pool",
				ingressClassNameSet: true,
				ingressClassName:    "traefik",
			},
			wantOut: "Successfully updated!\n",
			wantPoolSpec: ketchv1.PoolSpec{
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
				CtrlClientObjects: []runtime.Object{frontendPool},
			},
			options: poolUpdateOptions{
				name:         "frontend-pool",
				namespaceSet: true,
				namespace:    "new-namespace",
			},
			wantOut: "Successfully updated!\n",
			wantPoolSpec: ketchv1.PoolSpec{
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
				CtrlClientObjects: []runtime.Object{frontendPool},
			},
			options: poolUpdateOptions{
				name:             "frontend-pool",
				appQuotaLimitSet: true,
				appQuotaLimit:    50,
			},
			wantOut: "Successfully updated!\n",
			wantPoolSpec: ketchv1.PoolSpec{
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
				CtrlClientObjects: []runtime.Object{frontendPool},
			},
			options: poolUpdateOptions{
				name:           "frontend-pool",
				ingressTypeSet: true,
				ingressType:    traefik,
			},
			wantOut: "Successfully updated!\n",
			wantPoolSpec: ketchv1.PoolSpec{
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
				CtrlClientObjects:    []runtime.Object{frontendPool},
				DynamicClientObjects: []runtime.Object{clusterIssuerLe},
			},
			options: poolUpdateOptions{
				name:                    "frontend-pool",
				ingressClusterIssuerSet: true,
				ingressClusterIssuer:    "le-production",
			},
			wantOut: "Successfully updated!\n",
			wantPoolSpec: ketchv1.PoolSpec{
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
			name: "err - no pool",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{},
			},
			options: poolUpdateOptions{
				name:             "frontend-pool",
				appQuotaLimitSet: true,
				appQuotaLimit:    50,
			},
			wantErr: `failed to get the pool: pools.theketch.io "frontend-pool" not found`,
		},
		{
			name: "error - no cluster issuer",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{frontendPool},
			},
			options: poolUpdateOptions{
				name:                    "frontend-pool",
				ingressClusterIssuerSet: true,
				ingressClusterIssuer:    "le-production",
			},
			wantErr: "cluster issuer not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			err := poolUpdate(context.Background(), tt.cfg, tt.options, out)
			wantErr := len(tt.wantErr) > 0
			if wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.Equal(t, tt.wantOut, out.String())
			gotPool := ketchv1.Pool{}
			err = tt.cfg.Client().Get(context.Background(), types.NamespacedName{Name: tt.options.name}, &gotPool)
			require.Nil(t, err)
			require.Equal(t, tt.wantPoolSpec, gotPool.Spec)
		})
	}
}
