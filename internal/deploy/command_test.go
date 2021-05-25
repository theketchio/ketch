package deploy

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path"
	"testing"

	registryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/build"
	packService "github.com/shipa-corp/ketch/internal/pack"
)

type getterCreatorMockFn func(m *mockClient, obj runtime.Object) error
type funcMap map[int]getterCreatorMockFn

type mockClient struct {
	get    funcMap
	create funcMap
	update funcMap

	app      *ketchv1.App
	platform *ketchv1.Platform
	pool     *ketchv1.Pool

	getCounter    int
	createCounter int
	updateCounter int
}

func newMockClient() *mockClient {
	return &mockClient{
		get:    make(funcMap),
		update: make(funcMap),
		create: make(funcMap),
		app: &ketchv1.App{
			Spec: ketchv1.AppSpec{
				Description: "foo",
				Pool:        "initialpool",
				Platform:    "initialplatform",
			},
		},
		pool: &ketchv1.Pool{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       ketchv1.PoolSpec{},
			Status:     ketchv1.PoolStatus{},
		},
		platform: &ketchv1.Platform{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       ketchv1.PlatformSpec{},
		},
	}
}

func (m *mockClient) Get(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
	m.getCounter++

	if f, ok := m.get[m.getCounter]; ok {
		return f(m, obj)
	}

	switch v := obj.(type) {
	case *ketchv1.App:
		*v = *m.app
		return nil
	case *ketchv1.Platform:
		*v = *m.platform
		return nil
	case *ketchv1.Pool:
		*v = *m.pool
		return nil
	}
	panic("unhandled type")
}

func (m *mockClient) Create(_ context.Context, obj runtime.Object, _ ...client.CreateOption) error {
	m.createCounter++

	if f, ok := m.create[m.createCounter]; ok {
		return f(m, obj)
	}

	switch v := obj.(type) {
	case *ketchv1.App:
		m.app = v
		return nil
	case *ketchv1.Platform:
		m.platform = v
		return nil
	case *ketchv1.Pool:
		m.pool = v
		return nil
	}
	panic("unhandled type")
}

func (m *mockClient) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	m.updateCounter++

	if f, ok := m.update[m.updateCounter]; ok {
		return f(m, obj)
	}

	switch v := obj.(type) {
	case *ketchv1.App:
		m.app = v
		return nil
	case *ketchv1.Platform:
		m.platform = v
		return nil
	case *ketchv1.Pool:
		m.pool = v
		return nil
	}
	panic("unhandled type")
}

type packMocker struct{}

func (packMocker) BuildAndPushImage(ctx context.Context, req packService.BuildRequest) error {
	return nil
}

func getImageConfig(ctx context.Context, args imageConfigRequest) (*registryv1.ConfigFile, error) {
	return &registryv1.ConfigFile{
		Config: registryv1.Config{
			Cmd: []string{"/bin/eatme"},
		},
	}, nil
}

var ketchYaml string = `
kubernetes:
  processes:
    web:
      ports:
        - name: apache-http # an optional name for the port
          protocol: TCP 
          port: 80 # The port that is going to be exposed on the router.
          target_port: 9999 # The port on which the application listens on.
    worker:
      ports:
        - name: http 
          protocol: TCP
          port: 80
    worker-2:
      ports: []
`

func TestNewCommand(t *testing.T) {

	tt := []struct {
		name      string
		params    *Params
		arguments []string
		setup     func(t *testing.T)
		validate  func(t *testing.T, m getterCreator)
		wantError bool
	}{
		// build from source, creates app
		{
			name: "happy path build from source",
			arguments: []string{
				"myapp",
				"src",
				"--pool", "mypool",
				"--image", "shipa/go-sample:latest",
				"--env", "foo=bar,zip=zap",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
				require.Nil(t, ioutil.WriteFile("src/ketch.yaml", []byte(ketchYaml), 0600))
			},
			validate: func(t *testing.T, m getterCreator) {
				mock, ok := m.(*mockClient)
				require.True(t, ok)
				require.Equal(t, "mypool", mock.app.Spec.Pool)
				require.Len(t, mock.app.Spec.Deployments, 1)
				require.Len(t, mock.app.Spec.Deployments[0].KetchYaml.Kubernetes.Processes, 3)
				require.Len(t, mock.app.Spec.Env, 2)
			},
			params: &Params{
				Client: func() *mockClient {
					m := newMockClient()
					m.get[1] = func(_ *mockClient, _ runtime.Object) error {
						return errors.NewNotFound(v1.Resource(""), "")
					}
					return m
				}(),
				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		// build from source, updates app
		{
			name: "with custom yaml path and includes",
			arguments: []string{
				"myapp",
				"src",
				"--pool", "mypool",
				"--image", "shipa/go-sample:latest",
				"--env", "foo=bar,zip=zap",
				"--ketch-yaml", "config/ketch.yaml",
				"--registry-secret", "supersecret",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()

				require.Nil(t, os.Mkdir(path.Join(dir, "config"), 0700))
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.MkdirAll(path.Join(dir, "src/include1"), 0700))
				require.Nil(t, os.MkdirAll(path.Join(dir, "src/include2"), 0700))
				require.Nil(t, os.Chdir(dir))
				require.Nil(t, ioutil.WriteFile("config/ketch.yaml", []byte(ketchYaml), 0600))
			},
			validate: func(t *testing.T, m getterCreator) {
				mock, ok := m.(*mockClient)
				require.True(t, ok)
				require.Equal(t, "mypool", mock.app.Spec.Pool)
				require.Len(t, mock.app.Spec.Deployments, 1)
				require.Len(t, mock.app.Spec.Deployments[0].KetchYaml.Kubernetes.Processes, 3)
				require.Len(t, mock.app.Spec.Env, 2)
				require.Equal(t, "supersecret", mock.app.Spec.DockerRegistry.SecretName)
			},
			params: &Params{
				Client: func() *mockClient {
					m := newMockClient()

					return m
				}(),
				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		{
			name: "happy path with canary deploy build from source",
			arguments: []string{
				"myapp",
				"src",
				"--steps", "4",
				"--step-interval", "1h",
				"--pool", "mypool",
				"--image", "shipa/go-sample:latest",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
			},
			validate: func(t *testing.T, m getterCreator) {
				mock, ok := m.(*mockClient)
				require.True(t, ok)
				require.Equal(t, mock.app.Spec.Pool, "mypool")

			},
			params: &Params{
				Client: func() *mockClient {
					m := newMockClient()
					m.app.Spec.Deployments = []ketchv1.AppDeploymentSpec{
						{
							Image:           "shipa/go-sample:latest",
							Version:         1,
							Processes:       nil,
							KetchYaml:       nil,
							Labels:          nil,
							RoutingSettings: ketchv1.RoutingSettings{},
							ExposedPorts:    nil,
						},
					}
					return m
				}(),

				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		{
			name: "happy path build from image",
			arguments: []string{
				"myapp",
				"--pool", "mypool",
				"--image", "shipa/go-sample:latest",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
			},
			params: &Params{
				Client: func() *mockClient {
					m := newMockClient()
					m.get[1] = func(_ *mockClient, _ runtime.Object) error {
						return errors.NewNotFound(v1.Resource(""), "")
					}
					return m
				}(),

				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		{
			name:      "missing source path",
			wantError: true,
			arguments: []string{
				"myapp",
				"src",
				"--pool", "mypool",
				"--image", "shipa/go-sample:latest",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Chdir(dir))
			},
			params: &Params{
				Client: func() *mockClient {
					m := newMockClient()
					m.get[1] = func(_ *mockClient, _ runtime.Object) error {
						return errors.NewNotFound(v1.Resource(""), "")
					}
					return m
				}(),

				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		{
			name: "with environment variables",
			arguments: []string{
				"myapp",
				"src",
				"--pool", "mypool",
				"--image", "shipa/go-sample:latest",
				"--env", "foo=bar,bobb=dobbs",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
			},
			params: &Params{
				Client: func() *mockClient {
					m := newMockClient()
					m.get[1] = func(_ *mockClient, _ runtime.Object) error {
						return errors.NewNotFound(v1.Resource(""), "")
					}
					return m
				}(),

				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
		{
			name:      "with messed up environment variables",
			wantError: true,
			arguments: []string{
				"myapp",
				"src",
				"--pool", "mypool",
				"--image", "shipa/go-sample:latest",
				"--env", "foo=bar,bobbdobbs",
			},
			setup: func(t *testing.T) {
				dir := t.TempDir()
				require.Nil(t, os.Mkdir(path.Join(dir, "src"), 0700))
				require.Nil(t, os.Chdir(dir))
			},
			params: &Params{
				Client: func() *mockClient {
					m := newMockClient()
					m.get[1] = func(_ *mockClient, _ runtime.Object) error {
						return errors.NewNotFound(v1.Resource(""), "")
					}
					return m
				}(),
				KubeClient:     fake.NewSimpleClientset(),
				Builder:        build.GetSourceHandler(&packMocker{}),
				GetImageConfig: getImageConfig,
				Wait:           nil,
				Writer:         &bytes.Buffer{},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(t)
			}
			cmd := NewCommand(tc.params)
			cmd.SetArgs(tc.arguments)
			err := cmd.Execute()
			if tc.wantError {
				t.Logf("got error %s", err)
				require.NotNil(t, err)
				return
			}

			require.Nil(t, err)
			if tc.validate != nil {
				tc.validate(t, tc.params.Client)
			}
		})
	}
}
