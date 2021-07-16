package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/shipa-corp/ketch/internal/utils/conversions"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/mocks"
	"github.com/shipa-corp/ketch/internal/templates"
)

func Test_newAppExportCmd(t *testing.T) {
	pflag.CommandLine = pflag.NewFlagSet("ketch", pflag.ExitOnError)
	tests := []struct {
		name      string
		args      []string
		appExport appExportFn
		wantErr   bool
	}{
		{
			name: "happy path",
			args: []string{"foo-bar"},
			appExport: func(ctx context.Context, cfg config, options appExportOptions) error {
				require.Equal(t, "foo-bar", options.appName)
				return nil
			},
		},
		{
			name:    "missing arg",
			args:    []string{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newAppExportCmd(nil, tt.appExport)
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if tt.wantErr {
				require.NotNil(t, err)
				return
			}
			require.Nil(t, err)
		})
	}
}

type mockStorage struct {
	OnGet func(name string) (*templates.Templates, error)
}

func (m mockStorage) Get(name string) (*templates.Templates, error) {
	return m.OnGet(name)
}

func (m mockStorage) Update(name string, templates templates.Templates) error {
	panic("implement me")
}

var _ templates.Client = &mockStorage{}

func Test_appExport(t *testing.T) {
	dashboard := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dashboard",
		},
		Spec: ketchv1.AppSpec{
			Framework: "gke",
			Ingress: ketchv1.IngressSpec{
				GenerateDefaultCname: true,
			},
			Version: conversions.StrPtr("v1"),
		},
	}

	gke := &ketchv1.Framework{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gke",
		},
		Spec: ketchv1.FrameworkSpec{
			NamespaceName: "ketch-gke",
			IngressController: ketchv1.IngressControllerSpec{
				IngressType: ketchv1.IstioIngressControllerType,
			},
		},
	}

	tests := []struct {
		name    string
		cfg     config
		options appExportOptions
		wantOut string
		wantErr string
	}{
		{
			name: "happy path",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{dashboard, gke},
				StorageInstance: &mockStorage{
					OnGet: func(name string) (*templates.Templates, error) {
						require.Equal(t, templates.IngressConfigMapName(ketchv1.IstioIngressControllerType.String()), name)
						return &templates.Templates{}, nil
					},
				},
			},
			options: appExportOptions{
				appName: "dashboard",
			},
			wantOut: `framework: gke
name: dashboard
type: Application
version: v1
`,
		},
		{
			name: "no app",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{},
			},
			options: appExportOptions{
				appName: "dashboard",
			},
			wantErr: `failed to get app: apps.theketch.io "dashboard" not found`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.options.filename = filepath.Join(t.TempDir(), "app.yaml")
			defer os.Remove(tt.options.filename)

			err := exportApp(context.Background(), tt.cfg, tt.options)
			if tt.wantErr != "" {
				require.Equal(t, err.Error(), tt.wantErr)
			} else {
				require.Nil(t, err)
				b, err := os.ReadFile(tt.options.filename)
				require.Nil(t, err)
				require.Equal(t, tt.wantOut, string(b))
			}
		})
	}
}
