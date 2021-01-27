package main

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	registryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/fake"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/controllers"
	"github.com/shipa-corp/ketch/internal/mocks"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	kubeFake "k8s.io/client-go/kubernetes/fake"
)

func Test_appDeployOptions_KetchYaml(t *testing.T) {
	tests := []struct {
		name           string
		opts           appDeployOptions
		want           *ketchv1.KetchYamlData
		wantErr        bool
		wantErrMessage string
	}{
		{
			name: "valid ketch.yaml",
			opts: appDeployOptions{
				strictKetchYamlDecoding: true,
				ketchYamlFileName:       "./testdata/ketch.yaml",
			},
			want: &ketchv1.KetchYamlData{
				Hooks: &ketchv1.KetchYamlHooks{
					Restart: ketchv1.KetchYamlRestartHooks{
						Before: []string{`echo "before"`},
						After:  []string{`echo "after"`},
					},
				},
				Kubernetes: &ketchv1.KetchYamlKubernetesConfig{
					Processes: map[string]ketchv1.KetchYamlProcessConfig{
						"web": {
							Ports: []ketchv1.KetchYamlProcessPortConfig{
								{Name: "web", Protocol: "TCP", Port: 8080, TargetPort: 5000},
								{Name: "socket-port", Protocol: "TCP", Port: 4000},
							},
						},
						"worker": {Ports: []ketchv1.KetchYamlProcessPortConfig{}},
					},
				},
			},
		},
		{
			name: "ketch.yaml contains invalid fields",
			opts: appDeployOptions{
				strictKetchYamlDecoding: true,
				ketchYamlFileName:       "./testdata/invalid-ketch.yaml",
			},
			wantErr:        true,
			wantErrMessage: `error unmarshaling JSON: while decoding JSON: json: unknown field "invalidField"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.opts.KetchYaml()
			if (err != nil) != tt.wantErr {
				t.Errorf("KetchYaml() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && err.Error() != tt.wantErrMessage {
				t.Errorf("KetchYaml() error = %v, wantErr %v", err.Error(), tt.wantErrMessage)
				return
			}
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("KetchYaml() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

type mockRemoteImage struct {
	options     []remote.Option
	returnImage fake.FakeImage
	returnErr   error
}

func (m *mockRemoteImage) Image(ref name.Reference, options ...remote.Option) (registryv1.Image, error) {
	m.options = options
	return &m.returnImage, m.returnErr
}

func Test_createProcfileFromImageEntrypointAndCmd(t *testing.T) {

	tests := []struct {
		name string

		args           getImageConfigArgs
		mock           *mockRemoteImage
		initialObjects []runtime.Object

		wantErr        string
		wantOptionsLen int
		want           *registryv1.ConfigFile
	}{
		{
			name: "remote.Image error",
			args: getImageConfigArgs{
				imageName: "ketch:latest",
			},
			mock: &mockRemoteImage{
				returnErr: errors.New("image error"),
			},
			wantOptionsLen: 0,
			wantErr:        "image error",
		},
		{
			name: "procfile with a private registry",
			args: getImageConfigArgs{
				imageName:       "ketch:latest",
				secretName:      "top-secret",
				secretNamespace: "secret-namespace",
			},
			initialObjects: []runtime.Object{
				&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "top-secret",
						Namespace: "secret-namespace",
					},
				},
			},
			mock: &mockRemoteImage{
				returnImage: fake.FakeImage{
					ConfigFileStub: func() (*registryv1.ConfigFile, error) {
						return &registryv1.ConfigFile{
							Config: registryv1.Config{
								Cmd:        []string{"cmd"},
								Entrypoint: []string{"entrypoint"},
							},
						}, nil
					},
				},
			},
			wantOptionsLen: 1,
			want: &registryv1.ConfigFile{
				Config: registryv1.Config{
					Cmd:        []string{"cmd"},
					Entrypoint: []string{"entrypoint"},
				},
			},
		},
		{
			name: "secret not found",
			args: getImageConfigArgs{
				imageName:       "ketch:latest",
				secretName:      "top-secret",
				secretNamespace: "secret-namespace",
			},
			initialObjects: []runtime.Object{},
			mock:           &mockRemoteImage{},
			wantErr:        `secrets "top-secret" not found`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "secret-namespace",
				},
			}
			objects := append(tt.initialObjects, sa)
			kubeClient := kubeFake.NewSimpleClientset(objects...)
			got, err := getImageConfigFile(context.Background(), kubeClient, tt.args, tt.mock.Image)
			wantErr := len(tt.wantErr) > 0
			if (err != nil) != wantErr {
				t.Errorf("extractProcfileFromImage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if wantErr && err.Error() != tt.wantErr {
				t.Errorf("extractProcfileFromImage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("extractProcfileFromImage() mismatch (-want +got):\n%s", diff)
			}
			if len(tt.mock.options) != tt.wantOptionsLen {
				t.Errorf("extractProcfileFromImage() got options len = %v, want %v", len(tt.mock.options), tt.wantOptionsLen)
			}
		})
	}
}

type mockGetImageConfig struct {
	returnConfigFile *registryv1.ConfigFile
	returnErr        error
}

func (m *mockGetImageConfig) get(ctx context.Context, kubeClient kubernetes.Interface, args getImageConfigArgs, fn getRemoteImageFn) (*registryv1.ConfigFile, error) {
	return m.returnConfigFile, m.returnErr
}

func Test_appDeploy(t *testing.T) {

	app1 := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app-1",
		},
		Spec: ketchv1.AppSpec{
			Pool: "pool-1",
		},
	}
	pool1 := &ketchv1.Pool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pool-1",
		},
		Spec: ketchv1.PoolSpec{
			NamespaceName: "pool-1-namespace",
		},
	}
	validExtractFn := mockGetImageConfig{
		returnConfigFile: &registryv1.ConfigFile{
			Config: registryv1.Config{
				Cmd: []string{"cmd"},
				ExposedPorts: map[string]struct{}{
					"999/tcp": {},
				},
			},
		},
	}
	tests := []struct {
		name          string
		cfg           config
		options       appDeployOptions
		imageConfigFn getImageConfigFileFn
		watchEventFn  watchReconcileEventFn

		wantAppSpec ketchv1.AppSpec
		wantOut     string
		wantErr     string
	}{
		{
			name: "app deploy with entrypoint and cmd without ketch.yaml",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{app1, pool1},
			},
			options: appDeployOptions{
				appName: "app-1",
				image:   "ketch:v1",
			},
			imageConfigFn: validExtractFn.get,
			wantAppSpec: ketchv1.AppSpec{
				Deployments: []ketchv1.AppDeploymentSpec{
					{
						Image:           "ketch:v1",
						Version:         1,
						Processes:       []ketchv1.ProcessSpec{{Name: "web", Cmd: []string{"cmd"}}},
						RoutingSettings: ketchv1.RoutingSettings{Weight: 100},
						ExposedPorts: []ketchv1.ExposedPort{
							{Port: 999, Protocol: "TCP"},
						},
					},
				},
				DeploymentsCount: 1,
				Pool:             "pool-1",
			},
			wantOut: "app crd updated successfully, check the app’s events to understand results of the deployment\n",
		},
		{
			name: "app deploy with entrypoint and cmd + ketch.yaml",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{app1, pool1},
			},
			options: appDeployOptions{
				appName:           "app-1",
				image:             "ketch:v1",
				ketchYamlFileName: "./testdata/mini-ketch.yaml",
			},
			imageConfigFn: validExtractFn.get,
			wantAppSpec: ketchv1.AppSpec{
				Deployments: []ketchv1.AppDeploymentSpec{
					{
						Image:           "ketch:v1",
						Version:         1,
						Processes:       []ketchv1.ProcessSpec{{Name: "web", Cmd: []string{"cmd"}}},
						RoutingSettings: ketchv1.RoutingSettings{Weight: 100},
						KetchYaml: &ketchv1.KetchYamlData{
							Hooks: &ketchv1.KetchYamlHooks{Restart: ketchv1.KetchYamlRestartHooks{Before: []string{`echo "before"`}}},
						},
						ExposedPorts: []ketchv1.ExposedPort{
							{Port: 999, Protocol: "TCP"},
						},
					},
				},
				DeploymentsCount: 1,
				Pool:             "pool-1",
			},
			wantOut: "app crd updated successfully, check the app’s events to understand results of the deployment\n",
			wantErr: "",
		},
		{
			name: "app deploy with Procfile + ketch.yaml",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{app1, pool1},
			},
			options: appDeployOptions{
				appName:           "app-1",
				image:             "ketch:v1",
				procfileFileName:  "./testdata/Procfile",
				ketchYamlFileName: "./testdata/mini-ketch.yaml",
			},
			imageConfigFn: validExtractFn.get,
			wantAppSpec: ketchv1.AppSpec{
				Deployments: []ketchv1.AppDeploymentSpec{
					{
						Image:   "ketch:v1",
						Version: 1,
						Processes: []ketchv1.ProcessSpec{
							{Name: "web", Cmd: []string{"/app/app :$PORT web"}},
							{Name: "worker", Cmd: []string{"/app/app :$PORT worker"}},
						},
						RoutingSettings: ketchv1.RoutingSettings{Weight: 100},
						KetchYaml: &ketchv1.KetchYamlData{
							Hooks: &ketchv1.KetchYamlHooks{Restart: ketchv1.KetchYamlRestartHooks{Before: []string{`echo "before"`}}},
						},
						ExposedPorts: []ketchv1.ExposedPort{
							{Port: 999, Protocol: "TCP"},
						},
					},
				},
				DeploymentsCount: 1,
				Pool:             "pool-1",
			},
			wantOut: "app crd updated successfully, check the app’s events to understand results of the deployment\n",
		},
		{
			name: "app deploy with entrypoint and cmd with wait flag",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{app1, pool1},
			},
			options: appDeployOptions{
				appName: "app-1",
				image:   "ketch:v1",
				wait:    true,
				timeout: 20,
			},
			imageConfigFn: validExtractFn.get,
			watchEventFn:  fakeAppReconcileFn(1, time.Millisecond*100, "app-1", v1.EventTypeNormal, ""),
			wantAppSpec: ketchv1.AppSpec{
				Deployments: []ketchv1.AppDeploymentSpec{
					{
						Image:           "ketch:v1",
						Version:         1,
						Processes:       []ketchv1.ProcessSpec{{Name: "web", Cmd: []string{"cmd"}}},
						RoutingSettings: ketchv1.RoutingSettings{Weight: 100},
						ExposedPorts: []ketchv1.ExposedPort{
							{Port: 999, Protocol: "TCP"},
						},
					},
				},
				DeploymentsCount: 1,
				Pool:             "pool-1",
			},
			wantOut: "successfully deployed!\n",
		},
		{
			name: "error - reconcile failed",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{app1, pool1},
			},
			options: appDeployOptions{
				appName: "app-1",
				image:   "ketch:v1",
				wait:    true,
				timeout: 20,
			},
			imageConfigFn: validExtractFn.get,
			watchEventFn:  fakeAppReconcileFn(1, time.Millisecond*100, "app-1", v1.EventTypeWarning, "error on reconcile"),
			wantAppSpec: ketchv1.AppSpec{
				Deployments: []ketchv1.AppDeploymentSpec{
					{
						Image:           "ketch:v1",
						Version:         1,
						Processes:       []ketchv1.ProcessSpec{{Name: "web", Cmd: []string{"cmd"}}},
						RoutingSettings: ketchv1.RoutingSettings{Weight: 100},
						ExposedPorts: []ketchv1.ExposedPort{
							{Port: 999, Protocol: "TCP"},
						},
					},
				},
				DeploymentsCount: 1,
				Pool:             "pool-1",
			},
			wantErr: "error on reconcile",
		},
		{
			name: "error - no pool",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{app1},
			},
			options: appDeployOptions{
				appName: "app-1",
				image:   "ketch:v1",
			},
			imageConfigFn: validExtractFn.get,
			wantErr:       `failed to get pool instance: pools.theketch.io "pool-1" not found`,
		},
		{
			name: "error - no app",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{pool1},
			},
			options: appDeployOptions{
				appName: "app-1",
				image:   "ketch:v1",
			},
			imageConfigFn: validExtractFn.get,
			wantErr:       `failed to get app instance: apps.theketch.io "app-1" not found`,
		},
		{
			name: "error - ketch.yaml not found",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{app1, pool1},
			},
			options: appDeployOptions{
				appName:           "app-1",
				image:             "ketch:v1",
				ketchYamlFileName: "no-ketch.yaml",
			},
			imageConfigFn: validExtractFn.get,
			wantErr:       "failed to read ketch.yaml: open no-ketch.yaml: no such file or directory",
		},
		{
			name: "error - ketch.yaml not found",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{app1, pool1},
			},
			options: appDeployOptions{
				appName:           "app-1",
				image:             "ketch:v1",
				ketchYamlFileName: "no-ketch.yaml",
			},
			imageConfigFn: (&mockGetImageConfig{returnErr: errors.New("extract issue")}).get,
			wantErr:       "can't use the image: extract issue",
		},
		{
			name: "error - invalid ketch.yaml with strict option",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{app1, pool1},
			},
			options: appDeployOptions{
				appName:                 "app-1",
				image:                   "ketch:v1",
				ketchYamlFileName:       "./testdata/invalid-ketch.yaml",
				strictKetchYamlDecoding: true,
			},
			imageConfigFn: validExtractFn.get,
			wantErr:       `failed to read ketch.yaml: error unmarshaling JSON: while decoding JSON: json: unknown field "invalidField"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			err := appDeploy(context.Background(), metav1.Now, tt.cfg, tt.imageConfigFn, tt.watchEventFn, tt.options, out)
			wantErr := len(tt.wantErr) > 0
			if (err != nil) != wantErr {
				t.Errorf("appDeploy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if wantErr {
				assert.Equal(t, tt.wantErr, err.Error())
				return
			}
			assert.Equal(t, tt.wantOut, out.String())

			gotApp := ketchv1.App{}
			err = tt.cfg.Client().Get(context.Background(), types.NamespacedName{Name: "app-1"}, &gotApp)
			assert.Nil(t, err)
			if diff := cmp.Diff(gotApp.Spec, tt.wantAppSpec); diff != "" {
				t.Errorf("AppSpec mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_canaryAppDeploy(t *testing.T) {
	// set custom time func for tests
	testTimeNowFn := func() metav1.Time { return metav1.Date(2020, 12, 11, 20, 34, 58, 651387237, time.UTC) }
	testStepInt, _ := time.ParseDuration("1h")
	testNextScheduledTime := metav1.NewTime(testTimeNowFn().Add(testStepInt))

	app1 := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app-1",
		},
		Spec: ketchv1.AppSpec{
			Pool: "pool-1",
		},
	}
	pool1 := &ketchv1.Pool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pool-1",
		},
		Spec: ketchv1.PoolSpec{
			NamespaceName: "pool-1-namespace",
		},
	}
	validExtractFn := mockGetImageConfig{
		returnConfigFile: &registryv1.ConfigFile{
			Config: registryv1.Config{
				Cmd: []string{"cmd"},
				ExposedPorts: map[string]struct{}{
					"999/tcp": {},
				},
			},
		},
	}
	tests := []struct {
		name          string
		cfg           config
		options       appDeployOptions
		imageConfigFn getImageConfigFileFn
		watchEventFn  watchReconcileEventFn

		wantAppSpec               ketchv1.AppSpec
		wantPrimaryDeployment     bool
		wantExtraCanaryDeployment bool
		wantOut                   string
		wantErr                   string
	}{
		{
			name: "app deploy for canary deployment with primary deployment",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{app1, pool1},
			},
			options: appDeployOptions{
				appName:          "app-1",
				image:            "ketch:v2",
				steps:            10,
				stepTimeInterval: "1h",
			},
			imageConfigFn: validExtractFn.get,
			wantAppSpec: ketchv1.AppSpec{
				Deployments: []ketchv1.AppDeploymentSpec{
					{
						Image:           "ketch:v1",
						Version:         1,
						Processes:       []ketchv1.ProcessSpec{{Name: "web", Cmd: []string{"cmd"}}},
						RoutingSettings: ketchv1.RoutingSettings{Weight: 90},
						ExposedPorts: []ketchv1.ExposedPort{
							{Port: 999, Protocol: "TCP"},
						},
					},
					{
						Image:           "ketch:v2",
						Version:         2,
						Processes:       []ketchv1.ProcessSpec{{Name: "web", Cmd: []string{"cmd"}}},
						RoutingSettings: ketchv1.RoutingSettings{Weight: 10},
						ExposedPorts: []ketchv1.ExposedPort{
							{Port: 999, Protocol: "TCP"},
						},
					},
				},
				Canary: ketchv1.CanarySpec{
					Steps:             10,
					StepWeight:        10,
					StepTimeInteval:   testStepInt,
					NextScheduledTime: &testNextScheduledTime,
					CurrentCanaryStep: 1,
					IsActiveCanary:    true,
				},
				DeploymentsCount: 2,
				Pool:             "pool-1",
			},
			wantPrimaryDeployment: true,
			wantOut:               "app crd updated successfully, check the app’s events to understand results of the deployment\n",
		},
		{
			name: "app deploy for canary deployment without primary deployment",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{app1, pool1},
			},
			options: appDeployOptions{
				appName:          "app-1",
				image:            "ketch:v1",
				steps:            10,
				stepTimeInterval: "1h",
			},
			imageConfigFn: validExtractFn.get,
			wantAppSpec: ketchv1.AppSpec{
				Deployments: []ketchv1.AppDeploymentSpec{
					{
						Image:           "ketch:v1",
						Version:         1,
						Processes:       []ketchv1.ProcessSpec{{Name: "web", Cmd: []string{"cmd"}}},
						RoutingSettings: ketchv1.RoutingSettings{Weight: 10},
						ExposedPorts: []ketchv1.ExposedPort{
							{Port: 999, Protocol: "TCP"},
						},
					},
				},
				Canary: ketchv1.CanarySpec{
					Steps:           10,
					StepWeight:      10,
					StepTimeInteval: testStepInt,
				},
				DeploymentsCount: 1,
				Pool:             "pool-1",
			},
			wantErr:               "canary deployment failed. No primary deployment found for the app",
			wantPrimaryDeployment: false,
		},
		{
			name: "app deploy for canary deployment with primary deployment with canary already present",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{app1, pool1},
			},
			options: appDeployOptions{
				appName:          "app-1",
				image:            "ketch:v2",
				steps:            10,
				stepTimeInterval: "1h",
			},
			imageConfigFn:             validExtractFn.get,
			wantErr:                   "canary deployment failed. Maximum number of two deployments are currently supported",
			wantPrimaryDeployment:     true,
			wantExtraCanaryDeployment: true,
		},
		{
			name: "app deploy for canary deployment with invalid step limits",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{app1, pool1},
			},
			options: appDeployOptions{
				appName:          "app-1",
				image:            "ketch:v2",
				steps:            1001,
				stepWeight:       1,
				stepTimeInterval: "1h",
			},
			imageConfigFn: validExtractFn.get,
			wantErr:       "steps must be within the range 1 to 100",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPrimaryDeployment {
				primOpts := appDeployOptions{
					appName: "app-1",
					image:   "ketch:v1",
				}
				err := appDeploy(context.Background(), testTimeNowFn, tt.cfg, tt.imageConfigFn, tt.watchEventFn, primOpts, &bytes.Buffer{})
				require.Nil(t, err)
			}

			if tt.wantExtraCanaryDeployment {
				err := appDeploy(context.Background(), testTimeNowFn, tt.cfg, tt.imageConfigFn, tt.watchEventFn, tt.options, &bytes.Buffer{})
				require.Nil(t, err)
			}

			out := &bytes.Buffer{}
			err := appDeploy(context.Background(), testTimeNowFn, tt.cfg, tt.imageConfigFn, tt.watchEventFn, tt.options, out)

			wantErr := len(tt.wantErr) > 0
			if wantErr {
				require.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.wantOut, out.String())

			gotApp := ketchv1.App{}
			err = tt.cfg.Client().Get(context.Background(), types.NamespacedName{Name: "app-1"}, &gotApp)
			require.Nil(t, err)

			if tt.wantPrimaryDeployment {
				gotApp.Spec.Canary.NextScheduledTime = &testNextScheduledTime
			}

			require.Equal(t, tt.wantAppSpec, gotApp.Spec)
		})
	}
}

type fakeAppReconcileWatcher struct {
	watch.Interface
	ch chan watch.Event
}

func NewFakeAppReconcileWatcher() fakeAppReconcileWatcher {
	return fakeAppReconcileWatcher{
		ch: make(chan watch.Event),
	}
}

func (f *fakeAppReconcileWatcher) ResultChan() <-chan watch.Event {
	return f.ch
}

func (f *fakeAppReconcileWatcher) Stop() {
	close(f.ch)
}

func (f *fakeAppReconcileWatcher) Push(deplomentCount int, name, eventType, msg string) {
	reason := controllers.AppReconcileReason{Name: name, DeploymentCount: deplomentCount}
	evt := v1.Event{
		InvolvedObject: v1.ObjectReference{
			Kind:       "App",
			Name:       name,
			APIVersion: v1betaPrefix,
		},
		Reason:  reason.String(),
		Type:    eventType,
		Message: msg,
	}

	f.ch <- watch.Event{
		Type:   watch.Added,
		Object: &evt,
	}
}

func fakeAppReconcileFn(deplomentCount int, timeout time.Duration, name, eventType, msg string) watchReconcileEventFn {
	return func(ctx context.Context, kubeClient kubernetes.Interface, app *ketchv1.App) (watch.Interface, error) {
		watcher := NewFakeAppReconcileWatcher()
		time.Sleep(timeout)
		go watcher.Push(deplomentCount, name, eventType, msg)
		return &watcher, nil
	}
}
