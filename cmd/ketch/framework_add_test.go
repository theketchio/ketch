package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/mocks"
)

func Test_addFramework(t *testing.T) {
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
	tests := []struct {
		name    string
		cfg     config
		options frameworkAddOptions

		wantFrameworkSpec ketchv1.FrameworkSpec
		wantOut           string
		wantErr           string
	}{
		{
			name: "default class name for istio is istio",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{},
				DynamicClientObjects: []runtime.Object{clusterIssuerLe},
			},
			options: frameworkAddOptions{
				name:                   "hello",
				appQuotaLimit:          5,
				namespace:              "gke",
				ingressServiceEndpoint: "10.10.20.30",
				ingressType:            istio,
				ingressClusterIssuer:   "le-production",
			},

			wantFrameworkSpec: ketchv1.FrameworkSpec{
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
				CtrlClientObjects:    []runtime.Object{},
				DynamicClientObjects: []runtime.Object{clusterIssuerLe},
			},
			options: frameworkAddOptions{
				name:                   "hello",
				appQuotaLimit:          5,
				namespace:              "gke",
				ingressClassNameSet:    true,
				ingressClassName:       "custom-class-name",
				ingressServiceEndpoint: "10.10.20.30",
				ingressType:            istio,
				ingressClusterIssuer:   "le-production",
			},

			wantFrameworkSpec: ketchv1.FrameworkSpec{
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
			options: frameworkAddOptions{
				name:                   "aws",
				appQuotaLimit:          5,
				ingressClassName:       "traefik",
				ingressServiceEndpoint: "10.10.10.10",
				ingressType:            traefik,
			},

			wantFrameworkSpec: ketchv1.FrameworkSpec{
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
		{
			name: "error - no cluster issuer",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{},
				DynamicClientObjects: []runtime.Object{},
			},
			options: frameworkAddOptions{
				name:                 "hello",
				ingressClusterIssuer: "le-production",
			},

			wantErr: ErrClusterIssuerNotFound.Error(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			err := addFramework(context.Background(), tt.cfg, tt.options, out)
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.Equal(t, out.String(), tt.wantOut)

			gotFramework := ketchv1.Framework{}
			err = tt.cfg.Client().Get(context.Background(), types.NamespacedName{Name: tt.options.name}, &gotFramework)
			require.Nil(t, err)
			require.Equal(t, tt.wantFrameworkSpec, gotFramework.Spec)
		})
	}
}

func Test_newFrameworkAddCmd(t *testing.T) {
	pflag.CommandLine = pflag.NewFlagSet("ketch", pflag.ExitOnError)

	tests := []struct {
		name         string
		args         []string
		addFramework addFrameworkFn
		wantErr      bool
	}{
		{
			name: "class name is not set",
			args: []string{"ketch", "gke", "--ingress-type", "istio"},
			addFramework: func(ctx context.Context, cfg config, options frameworkAddOptions, out io.Writer) error {
				require.False(t, options.ingressClassNameSet)
				require.Equal(t, "gke", options.name)
				return nil
			},
		},
		{
			name: "class name is set",
			args: []string{"ketch", "gke", "--ingress-type", "istio", "--ingress-class-name", "custom-istio"},
			addFramework: func(ctx context.Context, cfg config, options frameworkAddOptions, out io.Writer) error {
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
			cmd := newFrameworkAddCmd(nil, nil, tt.addFramework)
			err := cmd.Execute()
			if tt.wantErr {
				require.NotNil(t, err)
				return
			}
			require.Nil(t, err)
		})
	}
}
