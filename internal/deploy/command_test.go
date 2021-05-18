package deploy

import (
	"bytes"
	"context"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/shipa-corp/ketch/internal/build"
	"github.com/shipa-corp/ketch/internal/docker"
	"testing"

	registryv1 "github.com/google/go-containerregistry/pkg/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stretchr/testify/require"
)

type mockClient struct {
	getFn    func(key client.ObjectKey) error
	createFn func(obj runtime.Object) error
	updateFn func(obj runtime.Object) error
}

func (m mockClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	if m.getFn != nil {
		return m.getFn(key)
	}
	return nil
}

func (m mockClient) Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
	if m.createFn != nil {
		return m.createFn(obj)
	}
	return nil
}

func (m mockClient) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	if m.updateFn != nil {
		return m.updateFn(obj)
	}
	return nil
}

type dockerMocker struct {}
func (dockerMocker) Build(ctx context.Context, req docker.BuildRequest)(*docker.BuildResponse,error){
	return &docker.BuildResponse{
		Procfile: ".proc",
		ImageURI: "shipa/someimage:latest",
	}, nil
}
func(dockerMocker) Push(ctx context.Context, req docker.BuildRequest) error {
	return nil
}

type mockConfiger struct  {}

func (c mockConfiger) ConfigFile() (*registryv1.ConfigFile, error) {
	return &registryv1.ConfigFile{}, nil
}


func remoteImgFn(ref name.Reference, options ...remote.Option) (ImageConfiger, error) {
	return &mockConfiger{}, nil
}

func TestNewCommand(t *testing.T) {
	tt := []struct {
		name      string
		params    *Params
		arguments []string
		setup     func(t *testing.T)
		wantError bool
	}{
		{
			name: "happy path build from source",
			arguments: []string{
				"myapp",
				"src",
				"--platform", "go",
				"--pool", "mypool",
				"--image", "shipa/go-sample:latest",
			},
			params: &Params{
				Client:      &mockClient{},
				KubeClient:  fake.NewSimpleClientset(),
				Builder:     build.GetSourceHandler(&dockerMocker{}),
				RemoteImage: remoteImgFn,
				Wait:        nil,
				Writer:      &bytes.Buffer{},
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
				require.NotNil(t, err)
				return
			}

			require.Nil(t, err)
		})
	}
}
