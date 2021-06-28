package main

import (
	"context"
	"os"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"

	"github.com/stretchr/testify/require"

	"github.com/shipa-corp/ketch/internal/mocks"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestExportFramework(t *testing.T) {
	mockFramework := &ketchv1.Framework{ObjectMeta: metav1.ObjectMeta{Name: "myframework"}, Spec: ketchv1.FrameworkSpec{
		NamespaceName: "ketch-myframework",
		Name:          "myframework",
		AppQuotaLimit: 1,
		IngressController: ketchv1.IngressControllerSpec{
			ClassName:       "traefik",
			ServiceEndpoint: "10.10.20.30",
			IngressType:     "traefik",
			ClusterIssuer:   "letsencrypt",
		},
	}}
	tests := []struct {
		name     string
		cfg      config
		options  frameworkExportOptions
		before   func()
		expected string
		err      error
	}{
		{
			name: "success",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{mockFramework},
				DynamicClientObjects: []runtime.Object{},
			},
			options: frameworkExportOptions{filename: "test-framework.yaml", frameworkName: "myframework"},
			expected: `name: myframework
namespace: ketch-myframework
appQuotaLimit: 1
ingressController:
    className: traefik
    serviceEndpoint: 10.10.20.30
    type: traefik
    clusterIssuer: letsencrypt
`,
		},
		{
			name: "error - file exists",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{mockFramework},
				DynamicClientObjects: []runtime.Object{},
			},
			options: frameworkExportOptions{filename: "test-framework.yaml", frameworkName: "myframework"},
			before: func() {
				os.WriteFile("test-framework.yaml", []byte("data"), os.ModePerm)
			},
			err: errFileExists,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer os.Remove(tt.options.filename)
			if tt.before != nil {
				tt.before()
			}
			err := exportFramework(context.Background(), tt.cfg, tt.options)
			if tt.err != nil {
				require.Equal(t, tt.err, err)
				return
			} else {
				require.Nil(t, err)
			}
			data, err := os.ReadFile(tt.options.filename)
			require.Nil(t, err)
			require.Equal(t, tt.expected, string(data))
		})
	}
}
