package main

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"bou.ke/monkey"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/chart"
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
			args: []string{"ketch", "foo-bar", "-d", "/tmp/app"},
			appExport: func(ctx context.Context, cfg config, chartNew chartNewFn, options appExportOptions, out io.Writer) error {
				require.Equal(t, "foo-bar", options.appName)
				require.Equal(t, "/tmp/app", options.directory)
				return nil
			},
		},
		{
			name:    "missing directory arg",
			args:    []string{"ketch", "foo-bar"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Args = tt.args
			cmd := newAppExportCmd(nil, nil, tt.appExport)
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
	directory1, err := ioutil.TempDir("", "ketch-app-export")
	require.Nil(t, err)

	dashboard := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dashboard",
		},
		Spec: ketchv1.AppSpec{
			Pool: "gke",
			Ingress: ketchv1.IngressSpec{
				GenerateDefaultCname: true,
			},
		},
	}

	gke := &ketchv1.Pool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gke",
		},
		Spec: ketchv1.PoolSpec{
			NamespaceName: "ketch-gke",
			IngressController: ketchv1.IngressControllerSpec{
				IngressType: ketchv1.IstioIngressControllerType,
			},
		},
	}

	tests := []struct {
		name     string
		cfg      config
		options  appExportOptions
		chartNew chartNewFn
		wantOut  string
		wantErr  string
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
				appName:   "dashboard",
				directory: directory1,
			},
			chartNew: func(application *ketchv1.App, pool *ketchv1.Pool, opts ...chart.Option) (*chart.ApplicationChart, error) {
				require.Equal(t, "dashboard", application.Name)
				require.Equal(t, "gke", pool.Name)
				return &chart.ApplicationChart{}, nil
			},
			wantOut: "Successfully exported!\n",
		},
		{
			name: "no pool",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{dashboard},
			},
			options: appExportOptions{
				appName:   "dashboard",
				directory: directory1,
			},
			wantErr: `failed to get pool: pools.theketch.io "gke" not found`,
		},
		{
			name: "no app",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{},
			},
			options: appExportOptions{
				appName:   "dashboard",
				directory: directory1,
			},
			wantErr: `failed to get app: apps.theketch.io "dashboard" not found`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// safely patch time.Now for tests
			patch := monkey.Patch(time.Now, func() time.Time { return time.Date(2020, 12, 11, 20, 34, 58, 651387237, time.UTC) })
			defer patch.Unpatch()
			out := &bytes.Buffer{}
			err := appExport(context.Background(), tt.cfg, tt.chartNew, tt.options, out)
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err, "appExport() error = %v, wantErr %v", err, tt.wantErr)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.wantOut, out.String())
			files, err := ioutil.ReadDir(tt.options.directory + "/" + tt.options.appName + "_11_Dec_20_20_34_UTC")
			require.Nil(t, err)

			directoryContent := make(map[string]struct{})
			for _, f := range files {
				directoryContent[f.Name()] = struct{}{}
			}
			expected := map[string]struct{}{
				"Chart.yaml":  {},
				"templates":   {},
				"values.yaml": {},
			}
			require.Equal(t, expected, directoryContent)
		})
	}
}
