package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/stretchr/testify/require"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/mocks"
)

func Test_poolUpdate(t *testing.T) {
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
				Domain:          "theketch.io",
				ServiceEndpoint: "192.168.1.17",
				IngressType:     ketchv1.IstioIngressControllerType,
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
					Domain:          "theketch.io",
					ServiceEndpoint: "192.168.1.18",
					IngressType:     ketchv1.IstioIngressControllerType,
				},
			},
		},
		{
			name: "update ingress domain name",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{frontendPool},
			},
			options: poolUpdateOptions{
				name:                 "frontend-pool",
				ingressDomainNameSet: true,
				ingressDomainName:    "theketch.cloud",
			},
			wantOut: "Successfully updated!\n",
			wantPoolSpec: ketchv1.PoolSpec{
				NamespaceName: "frontend",
				AppQuotaLimit: 30,
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "default-classname",
					Domain:          "theketch.cloud",
					ServiceEndpoint: "192.168.1.17",
					IngressType:     ketchv1.IstioIngressControllerType,
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
					Domain:          "theketch.io",
					ServiceEndpoint: "192.168.1.17",
					IngressType:     ketchv1.IstioIngressControllerType,
				},
			},
		},
		{
			name: "update namespace name",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{frontendPool},
			},
			options: poolUpdateOptions{
				name:             "frontend-pool",
				kubeNamespaceSet: true,
				kubeNamespace:    "new-namespace",
			},
			wantOut: "Successfully updated!\n",
			wantPoolSpec: ketchv1.PoolSpec{
				NamespaceName: "new-namespace",
				AppQuotaLimit: 30,
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "default-classname",
					Domain:          "theketch.io",
					ServiceEndpoint: "192.168.1.17",
					IngressType:     ketchv1.IstioIngressControllerType,
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
					Domain:          "theketch.io",
					ServiceEndpoint: "192.168.1.17",
					IngressType:     ketchv1.IstioIngressControllerType,
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
				ingressType:    traefik17,
			},
			wantOut: "Successfully updated!\n",
			wantPoolSpec: ketchv1.PoolSpec{
				NamespaceName: "frontend",
				AppQuotaLimit: 30,
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "default-classname",
					Domain:          "theketch.io",
					ServiceEndpoint: "192.168.1.17",
					IngressType:     ketchv1.Traefik17IngressControllerType,
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			err := poolUpdate(context.Background(), tt.cfg, tt.options, out)
			wantErr := len(tt.wantErr) > 0
			if wantErr {
				require.NotNil(t, err, "poolUpdate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if wantErr {
				require.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.Equal(t, tt.wantOut, out.String(), "poolUpdate() gotOut = %v, want %v", out.String(), tt.wantOut)
			gotPool := ketchv1.Pool{}
			err = tt.cfg.Client().Get(context.Background(), types.NamespacedName{Name: tt.options.name}, &gotPool)
			require.Nil(t, err)
			if diff := cmp.Diff(gotPool.Spec, tt.wantPoolSpec); diff != "" {
				t.Errorf("PoolSpec mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
