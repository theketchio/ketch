package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/mocks"
)

func Test_addPool(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config
		options poolAddOptions

		wantPoolSpec ketchv1.PoolSpec
		wantOut      string
		wantErr      bool
	}{
		{
			name: "default class name for istio is istio",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{},
			},
			options: poolAddOptions{
				name:                   "hello",
				appQuotaLimit:          5,
				namespace:              "gke",
				ingressServiceEndpoint: "10.10.20.30",
				ingressType:            istio,
				ingressClusterIssuer:   "le-production",
			},

			wantPoolSpec: ketchv1.PoolSpec{
				NamespaceName: "gke",
				AppQuotaLimit: 5,
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "istio",
					ServiceEndpoint: "10.10.20.30",
					IngressType:     ketchv1.IstioIngressControllerType,
					ClusterIssuer:   "le-production",
				},
			},
			wantOut: "Successfully added!\n",
		},
		{
			name: "successfully added with istio",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{},
			},
			options: poolAddOptions{
				name:                   "hello",
				appQuotaLimit:          5,
				namespace:              "gke",
				ingressClassNameSet:    true,
				ingressClassName:       "custom-class-name",
				ingressServiceEndpoint: "10.10.20.30",
				ingressType:            istio,
				ingressClusterIssuer:   "le-production",
			},

			wantPoolSpec: ketchv1.PoolSpec{
				NamespaceName: "gke",
				AppQuotaLimit: 5,
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "custom-class-name",
					ServiceEndpoint: "10.10.20.30",
					IngressType:     ketchv1.IstioIngressControllerType,
					ClusterIssuer:   "le-production",
				},
			},
			wantOut: "Successfully added!\n",
		},
		{
			name: "traefik + default namespace with ketch- prefix",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{},
			},
			options: poolAddOptions{
				name:                   "aws",
				appQuotaLimit:          5,
				ingressClassName:       "traefik",
				ingressServiceEndpoint: "10.10.10.10",
				ingressType:            traefik,
			},

			wantPoolSpec: ketchv1.PoolSpec{
				NamespaceName: "ketch-aws",
				AppQuotaLimit: 5,
				IngressController: ketchv1.IngressControllerSpec{
					ClassName:       "traefik",
					ServiceEndpoint: "10.10.10.10",
					IngressType:     ketchv1.TraefikIngressControllerType,
				},
			},
			wantOut: "Successfully added!\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			err := addPool(context.Background(), tt.cfg, tt.options, out)
			if tt.wantErr {
				require.NotNil(t, err)
				return
			}
			require.Equal(t, out.String(), tt.wantOut)

			gotPool := ketchv1.Pool{}
			err = tt.cfg.Client().Get(context.Background(), types.NamespacedName{Name: tt.options.name}, &gotPool)
			require.Nil(t, err)
			if diff := cmp.Diff(gotPool.Spec, tt.wantPoolSpec); diff != "" {
				t.Errorf("PoolSpec mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_newPoolAddCmd(t *testing.T) {
	pflag.CommandLine = pflag.NewFlagSet("ketch", pflag.ExitOnError)

	tests := []struct {
		name    string
		args    []string
		addPool addPoolFn
		wantErr bool
	}{
		{
			name: "class name is not set",
			args: []string{"ketch", "gke", "--ingress-type", "istio"},
			addPool: func(ctx context.Context, cfg config, options poolAddOptions, out io.Writer) error {
				require.False(t, options.ingressClassNameSet)
				require.Equal(t, "gke", options.name)
				return nil
			},
		},
		{
			name: "class name is set",
			args: []string{"ketch", "gke", "--ingress-type", "istio", "--ingress-class-name", "custom-istio"},
			addPool: func(ctx context.Context, cfg config, options poolAddOptions, out io.Writer) error {
				require.True(t, options.ingressClassNameSet)
				require.Equal(t, "gke", options.name)
				require.Equal(t, "custom-istio", options.ingressClassName)
				return nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Args = tt.args
			cmd := newPoolAddCmd(nil, nil, tt.addPool)
			err := cmd.Execute()
			if tt.wantErr {
				require.NotNil(t, err)
				return
			}
			require.Nil(t, err)
		})
	}
}
