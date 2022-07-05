package main

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"
	"github.com/theketchio/ketch/internal/mocks"
)

func Test_appInfo(t *testing.T) {
	dashboard := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dashboard",
		},
		Spec: ketchv1.AppSpec{
			Namespace: "gke",
			Ingress: ketchv1.IngressSpec{
				GenerateDefaultCname: true,
			},
		},
	}
	appPython := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app-python",
		},
		Spec: ketchv1.AppSpec{
			Namespace: "gke",
			Ingress: ketchv1.IngressSpec{
				GenerateDefaultCname: true,
			},
			Builder: "python",
		},
	}
	goApp := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "go-app",
		},
		Spec: ketchv1.AppSpec{
			Deployments: []ketchv1.AppDeploymentSpec{
				{
					Version: 1,
					Image:   "shipasoftware/go-app:v1",
					Processes: []ketchv1.ProcessSpec{
						{
							Name: "web",
							Cmd:  []string{"docker-entrypoint.sh", "npm", "start"},
						},
						{
							Name: "worker",
							Cmd:  []string{"docker-entrypoint.sh", "npm", "worker"},
						},
					},
				},
			},
			Env: []ketchv1.Env{
				{Name: "API_KEY", Value: "public_key"},
				{Name: "VAR1", Value: "VALUE"},
			},
			Namespace: "aws",
			Ingress: ketchv1.IngressSpec{
				GenerateDefaultCname: true,
				Controller:           ketchv1.IngressControllerSpec{ServiceEndpoint: "10.10.10.10"},
			},
		},
	}
	goAppWithSecretName := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "go-app",
		},
		Spec: ketchv1.AppSpec{
			Deployments: []ketchv1.AppDeploymentSpec{
				{
					Version: 1,
					Image:   "shipasoftware/go-app:v4",
					Processes: []ketchv1.ProcessSpec{
						{
							Name: "web",
							Cmd:  []string{"docker-entrypoint.sh", "npm", "start"},
						},
					},
				},
			},
			Namespace: "aws",
			Ingress: ketchv1.IngressSpec{
				GenerateDefaultCname: true,
				Cnames:               ketchv1.CnameList{{Name: "theketch.io"}, {Name: "www.theketch.io"}},
				Controller:           ketchv1.IngressControllerSpec{ServiceEndpoint: "10.10.10.10"},
			},
			DockerRegistry: ketchv1.DockerRegistrySpec{
				SecretName: "go-app-pull-credentials",
			},
		},
	}
	tests := []struct {
		name               string
		cfg                config
		options            appInfoOptions
		wantOutputFilename string
		wantErr            bool
	}{
		{
			name: "no cnames, no env variable, no processes",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{dashboard},
			},
			options: appInfoOptions{
				name: "dashboard",
			},
			wantOutputFilename: "./testdata/app-info/dashboard.output",
		},
		{
			name: "no cnames, env variables, processes",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{goApp},
			},
			options: appInfoOptions{
				name: "go-app",
			},
			wantOutputFilename: "./testdata/app-info/go-app.output",
		},
		{
			name: "cnames, env variables, processes + secret name",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{goAppWithSecretName},
			},
			options: appInfoOptions{
				name: "go-app",
			},
			wantOutputFilename: "./testdata/app-info/go-app-secret-name.output",
		},
		{
			name: "app with builder",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{appPython},
			},
			options: appInfoOptions{
				name: "app-python",
			},
			wantOutputFilename: "./testdata/app-info/app-python.output",
		},
		{
			name: "no app",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{},
			},
			options: appInfoOptions{
				name: "dashboard",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			err := appInfo(context.Background(), tt.cfg, tt.options, out)
			if tt.wantErr {
				require.NotNil(t, err, "appInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.Nil(t, err)
			wantOut, err := ioutil.ReadFile(tt.wantOutputFilename)
			require.Nil(t, err)
			require.Equal(t, string(wantOut), out.String())
		})
	}
}
