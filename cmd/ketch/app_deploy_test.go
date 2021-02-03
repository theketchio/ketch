package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	registryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/fake"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	kubeFake "k8s.io/client-go/kubernetes/fake"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/build"
	"github.com/shipa-corp/ketch/internal/chart"
	"github.com/shipa-corp/ketch/internal/controllers"
	"github.com/shipa-corp/ketch/internal/mocks"
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
				&corev1.Secret{
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
			sa := &corev1.ServiceAccount{
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

func metav1TimeRef(t metav1.Time) *metav1.Time {
	return &t
}

func Test_changeAppCRD(t *testing.T) {

	configFile := &registryv1.ConfigFile{
		Config: registryv1.Config{
			ExposedPorts: map[string]struct{}{
				"999/tcp": {},
			},
		},
	}
	tests := []struct {
		name      string
		args      deploymentArguments
		sourceApp *ketchv1.App

		wantAppSpec ketchv1.AppSpec
		wantErr     string
	}{
		{
			name: "canary deployment",
			sourceApp: &ketchv1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-1",
				},
				Spec: ketchv1.AppSpec{
					Pool: "pool-1",
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
				},
			},
			args: deploymentArguments{
				image: "ketch:v2",
				procfile: chart.Procfile{
					Processes: map[string][]string{
						"web":    {"/app/web"},
						"worker": {"/app/worker"},
					},
					RoutableProcessName: "web",
				},
				steps:             3,
				stepWeight:        33,
				stepTimeInterval:  5 * time.Hour,
				nextScheduledTime: time.Date(2017, 11, 11, 10, 30, 30, 0, time.UTC),
				configFile:        configFile,
				started:           time.Date(2017, 11, 11, 10, 30, 30, 0, time.UTC),
			},
			wantAppSpec: ketchv1.AppSpec{
				Canary: ketchv1.CanarySpec{
					Steps:             3,
					StepWeight:        33,
					StepTimeInteval:   5 * time.Hour,
					NextScheduledTime: metav1TimeRef(metav1.NewTime(time.Date(2017, 11, 11, 10, 30, 30, 0, time.UTC))),
					CurrentStep:       1,
					Active:            true,
					Started:           metav1TimeRef(metav1.NewTime(time.Date(2017, 11, 11, 10, 30, 30, 0, time.UTC))),
				},
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
					{
						Image:   "ketch:v2",
						Version: 1,
						Processes: []ketchv1.ProcessSpec{
							{Name: "web", Cmd: []string{"/app/web"}},
							{Name: "worker", Cmd: []string{"/app/worker"}},
						},
						RoutingSettings: ketchv1.RoutingSettings{Weight: 0},
						ExposedPorts: []ketchv1.ExposedPort{
							{Port: 999, Protocol: "TCP"},
						},
					},
				},
				DeploymentsCount: 1,
				Pool:             "pool-1",
			},
		},
		{
			name: "app deploy with entrypoint and cmd without ketch.yaml",
			args: deploymentArguments{
				image: "ketch:v1",
				procfile: chart.Procfile{
					Processes: map[string][]string{
						"web": {"cmd"},
					},
					RoutableProcessName: "web",
				},
				configFile: configFile,
			},
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
		},
		{
			name: "app deploy with entrypoint and cmd + ketch.yaml",
			args: deploymentArguments{
				image: "ketch:v1",
				ketchYaml: &ketchv1.KetchYamlData{
					Hooks: &ketchv1.KetchYamlHooks{Restart: ketchv1.KetchYamlRestartHooks{Before: []string{`echo "before"`}}},
				},
				procfile: chart.Procfile{
					Processes: map[string][]string{
						"web": {"/app/app"},
					},
					RoutableProcessName: "web",
				},
				configFile: configFile,
			},
			wantAppSpec: ketchv1.AppSpec{
				Deployments: []ketchv1.AppDeploymentSpec{
					{
						Image:           "ketch:v1",
						Version:         1,
						Processes:       []ketchv1.ProcessSpec{{Name: "web", Cmd: []string{"/app/app"}}},
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
		},
		{
			name: "app deploy with Procfile + ketch.yaml",
			args: deploymentArguments{
				image: "ketch:v1",
				ketchYaml: &ketchv1.KetchYamlData{
					Hooks: &ketchv1.KetchYamlHooks{Restart: ketchv1.KetchYamlRestartHooks{Before: []string{`echo "before"`}}},
				},
				procfile: chart.Procfile{
					Processes: map[string][]string{
						"web":    {"/app/app :$PORT web"},
						"worker": {"/app/app :$PORT worker"},
					},
					RoutableProcessName: "web",
				},
				configFile: configFile,
			},
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
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := tt.sourceApp
			if app == nil {
				app = &ketchv1.App{
					ObjectMeta: metav1.ObjectMeta{
						Name: "app-1",
					},
					Spec: ketchv1.AppSpec{
						Pool: "pool-1",
					},
				}
			}
			err := changeAppCRD(app, tt.args)
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.True(t, strings.Contains(err.Error(), tt.wantErr))
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.wantAppSpec, app.Spec)
		})
	}
}

func Test_appDeploy(t *testing.T) {

	dashboard := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dashboard",
		},
		Spec: ketchv1.AppSpec{
			Pool: "pool-1",
		},
	}
	goapp := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "go-app",
		},
		Spec: ketchv1.AppSpec{
			Pool:     "pool-1",
			Platform: "golang",
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
	platform := &ketchv1.Platform{
		ObjectMeta: metav1.ObjectMeta{
			Name: "golang",
		},
		Spec: ketchv1.PlatformSpec{
			Image: "shipasoftware/golang:latest",
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
		name              string
		cfg               config
		options           appDeployOptions
		imageConfigFn     getImageConfigFileFn
		waitFn            waitFn
		changeAppCRDFn    changeAppCRDFn
		buildFromSourceFn buildFromSourceFn

		wantOut string
		wantErr string
	}{
		{
			name: "error - changeAppFn failed",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{dashboard, pool1},
			},
			options: appDeployOptions{
				appName: "dashboard",
				image:   "ketch:v1",
				timeout: "20s",
				steps:   1,
			},
			imageConfigFn: validExtractFn.get,
			changeAppCRDFn: func(app *ketchv1.App, args deploymentArguments) error {
				return errors.New("changeAppFn error")
			},
			wantErr: "changeAppFn error",
		},
		{
			name: "error - buildFromSource failed: application without platform",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{dashboard, pool1, platform},
			},
			options: appDeployOptions{
				appName:       "dashboard",
				image:         "ketch:v1",
				timeout:       "20s",
				steps:         1,
				appSourcePath: "/home/shipa/application",
			},
			wantErr: "can't build an application without platform",
		},
		{
			name: "error - buildFromSource failed: platform not found",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{goapp, pool1},
			},
			options: appDeployOptions{
				appName:       "go-app",
				image:         "ketch:v1",
				timeout:       "20s",
				steps:         1,
				appSourcePath: "/home/shipa/application",
			},
			wantErr: "failed to get platform",
		},
		{
			name: "error - buildFromSource failed",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{goapp, pool1, platform},
			},
			options: appDeployOptions{
				appName:       "go-app",
				image:         "ketch:v1",
				timeout:       "20s",
				steps:         1,
				appSourcePath: "/home/shipa/application",
			},
			imageConfigFn: validExtractFn.get,
			buildFromSourceFn: func(ctx context.Context, request *build.CreateImageFromSourceRequest, opts ...build.Option) (*build.CreateImageFromSourceResponse, error) {
				require.Equal(t, "shipasoftware/golang:latest", request.PlatformImage)
				require.Equal(t, "go-app", request.AppName)
				require.Equal(t, "ketch:v1", request.Image)
				return nil, errors.New("buildFn error")
			},
			wantErr: "buildFn error",
		},
		{
			name: "error - wait failed",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{dashboard, pool1},
			},
			options: appDeployOptions{
				appName: "dashboard",
				image:   "ketch:v1",
				timeout: "20s",
				steps:   1,
				wait:    true,
			},
			imageConfigFn: validExtractFn.get,
			changeAppCRDFn: func(app *ketchv1.App, args deploymentArguments) error {
				return nil
			},
			waitFn: func(ctx context.Context, cfg config, app ketchv1.App, timeout time.Duration, out io.Writer) error {
				return errors.New("wait error")
			},
			wantErr: "wait error",
		},
		{
			name: "happy path - wait is not called",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{dashboard, pool1},
			},
			options: appDeployOptions{
				appName: "dashboard",
				image:   "ketch:v1",
				timeout: "20s",
				steps:   1,
			},
			imageConfigFn: validExtractFn.get,
			changeAppCRDFn: func(app *ketchv1.App, args deploymentArguments) error {
				return nil
			},
			wantOut: "App dashboard deployed successfully. Run `ketch app info dashboard` to check status of the deployment\n",
		},
		{
			name: "error - no pool",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{dashboard},
			},
			options: appDeployOptions{
				appName: "dashboard",
				image:   "ketch:v1",
				steps:   1,
				timeout: "25s",
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
				appName: "dashboard",
				image:   "ketch:v1",
				steps:   1,
				timeout: "25s",
			},
			imageConfigFn: validExtractFn.get,
			wantErr:       `failed to get app instance: apps.theketch.io "dashboard" not found`,
		},
		{
			name: "error - ketch.yaml not found",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{dashboard, pool1},
			},
			options: appDeployOptions{
				appName:           "dashboard",
				image:             "ketch:v1",
				ketchYamlFileName: "no-ketch.yaml",
				steps:             1,
				timeout:           "25s",
			},
			imageConfigFn: validExtractFn.get,
			wantErr:       "ketch.yaml could not be processed",
		},
		{
			name: "error - ketch.yaml not found",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{dashboard, pool1},
			},
			options: appDeployOptions{
				appName: "dashboard",
				image:   "ketch:v1",
				steps:   1,
				timeout: "25s",
			},
			imageConfigFn: (&mockGetImageConfig{returnErr: errors.New("extract issue")}).get,
			wantErr:       "can't use the image: extract issue",
		},
		{
			name: "error - invalid ketch.yaml with strict option",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{dashboard, pool1},
			},
			options: appDeployOptions{
				appName:                 "dashboard",
				image:                   "ketch:v1",
				ketchYamlFileName:       "./testdata/invalid-ketch.yaml",
				strictKetchYamlDecoding: true,
				steps:                   1,
				timeout:                 "25s",
			},
			imageConfigFn: validExtractFn.get,
			wantErr:       `unknown field "invalidField"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			out := &bytes.Buffer{}
			err := appDeploy(context.Background(), tt.cfg, tt.imageConfigFn, tt.waitFn, tt.buildFromSourceFn, tt.changeAppCRDFn, tt.options, out)

			wantErr := len(tt.wantErr) > 0
			if wantErr {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.wantOut, out.String())

			gotApp := ketchv1.App{}
			err = tt.cfg.Client().Get(context.Background(), types.NamespacedName{Name: tt.options.appName}, &gotApp)
			require.Nil(t, err)

		})
	}
}

func Test_waitHandler(t *testing.T) {

	tests := []struct {
		description     string
		deploymentCount int
		appName         string
		eventType       string
		controller      func(watcher *fakeAppReconcileWatcher, deploymentCount int, appName string, message string)
		timeout         time.Duration
		wantOut         string
		wantErr         string
	}{
		{
			description: "happy path",
			controller: func(watcher *fakeAppReconcileWatcher, deploymentCount int, appName string, message string) {
				watcher.Push(deploymentCount, appName, corev1.EventTypeNormal, message)
			},
			timeout:         10 * time.Second,
			eventType:       corev1.EventTypeNormal,
			deploymentCount: 3,
			appName:         "dashboard",
			wantOut:         "successfully deployed!\n",
		},
		{
			description: "no events for this deployment",
			controller: func(watcher *fakeAppReconcileWatcher, deploymentCount int, appName string, message string) {
				watcher.Push(deploymentCount, appName, corev1.EventTypeNormal, message)
			},
			timeout:         3 * time.Second,
			deploymentCount: 10,
			appName:         "dashboard",
			wantErr:         "maximum execution time exceeded",
		},
		{
			description: "deployment fails",
			controller: func(watcher *fakeAppReconcileWatcher, deploymentCount int, appName string, message string) {
				watcher.Push(deploymentCount, appName, corev1.EventTypeWarning, message)
			},
			timeout:         3 * time.Second,
			deploymentCount: 3,
			appName:         "dashboard",
			wantErr:         "deployed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {

			watcher := &fakeAppReconcileWatcher{ch: make(chan watch.Event)}
			fn := func(ctx context.Context, kubeClient kubernetes.Interface, app *ketchv1.App) (watch.Interface, error) {
				return watcher, nil
			}
			out := &bytes.Buffer{}
			app := ketchv1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name: "dashboard",
				},
				Spec: ketchv1.AppSpec{
					DeploymentsCount: 3,
				},
			}
			go tt.controller(watcher, tt.deploymentCount, tt.appName, "deployed")
			err := waitHandler(fn)(context.Background(), &mocks.Configuration{}, app, tt.timeout, out)
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.wantOut, out.String())
		})
	}
}

type fakeAppReconcileWatcher struct {
	ch chan watch.Event
}

// ResultChan implements ResultChan method of watch.Interface.
func (f *fakeAppReconcileWatcher) ResultChan() <-chan watch.Event {
	return f.ch
}

// Stop implements Stop method of watch.Interface.
func (f *fakeAppReconcileWatcher) Stop() {
	close(f.ch)
}

func (f *fakeAppReconcileWatcher) Push(deplomentCount int, appName, eventType, msg string) {
	reason := controllers.AppReconcileReason{AppName: appName, DeploymentCount: deplomentCount}
	evt := corev1.Event{
		InvolvedObject: corev1.ObjectReference{
			Kind:       "App",
			Name:       appName,
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
