package main

import (
	"bytes"
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/mocks"
)

func Test_poolList(t *testing.T) {
	poolA := &ketchv1.Pool{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "pool-a",
		},
		Spec: ketchv1.PoolSpec{
			NamespaceName: "a",
			AppQuotaLimit: 30,
			IngressController: ketchv1.IngressControllerSpec{
				ClassName:       "istio",
				ServiceEndpoint: "192.168.1.17",
				ClusterIssuer:   "letsencrypt",
				IngressType:     ketchv1.IstioIngressControllerType,
			},
		},
	}
	poolB := &ketchv1.Pool{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "pool-b",
		},
		Spec: ketchv1.PoolSpec{
			NamespaceName: "b",
			AppQuotaLimit: 30,
			IngressController: ketchv1.IngressControllerSpec{
				ClassName:       "classname-b",
				ServiceEndpoint: "192.168.1.17",
				ClusterIssuer:   "letsencrypt",
				IngressType:     ketchv1.TraefikIngressControllerType,
			},
		},
	}
	tests := []struct {
		name string
		cfg  config

		wantOut string
		wantErr bool
	}{
		{
			name: "update service endpoint",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{poolA, poolB},
			},
			wantOut: `NAME      STATUS    NAMESPACE    INGRESS TYPE    INGRESS CLASS NAME    CLUSTER ISSUER    APPS
pool-a              a            istio           istio                 letsencrypt       0/30
pool-b              b            traefik         classname-b           letsencrypt       0/30
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			err := poolList(context.Background(), tt.cfg, out)
			if (err != nil) != tt.wantErr {
				t.Errorf("poolList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotOut := out.String(); gotOut != tt.wantOut {
				t.Errorf("poolList() gotOut = %v, want %v", gotOut, tt.wantOut)
			}
		})
	}
}
