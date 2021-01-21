package build

import (
	"context"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/src-d/go-git.v4"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/docker"
	"github.com/shipa-corp/ketch/internal/errors"
)

type mockGetter struct {
	calls  int
	testFn func(name types.NamespacedName, object runtime.Object) error
}

func (m *mockGetter) Get(ctx context.Context, name types.NamespacedName, object runtime.Object) error {
	m.calls += 1
	return m.testFn(name, object)
}

type mockBuilder struct {
	calls  int
	testFn func(req *docker.BuildRequest) (*docker.BuildResponse, error)
}

func (mb *mockBuilder) Build(ctx context.Context, req *docker.BuildRequest) (*docker.BuildResponse, error) {
	mb.calls += 1
	return mb.testFn(req)
}

func cloneSource(tempDir, repositoryURL string) (string, error) {
	targetDir := path.Join(tempDir, "source")

	_, err := git.PlainClone(targetDir, true, &git.CloneOptions{
		URL:      repositoryURL,
		Progress: os.Stdout,
	})
	if err != nil {
		return "", err
	}

	return targetDir, nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func TestGetSourceHandler(t *testing.T) {
	workingDir, err := cloneSource(t.TempDir(), "https://github.com/shipa-corp/go-sample")
	require.Nil(t, err)

	tt := []struct {
		name      string
		wantErr   bool
		builderFn func(req *docker.BuildRequest) (*docker.BuildResponse, error)
		crdGetFn  func(name types.NamespacedName, object runtime.Object) error
		request   *CreateImageFromSourceRequest
		expected  *CreateImageFromSourceResponse
	}{
		{
			name: "happy path",
			builderFn: func(req *docker.BuildRequest) (*docker.BuildResponse, error) {
				assert.True(t, fileExists(path.Join(req.BuildDirectory, "Dockerfile")))
				assert.True(t, fileExists(path.Join(req.BuildDirectory, archiveFileName)))
				assert.Equal(t, req.Image, "acme/superimage")
				return &docker.BuildResponse{
					ImageURI: "docker.io/acme/superimage:latest",
				}, nil
			},
			crdGetFn: func(_ types.NamespacedName, object runtime.Object) error {
				switch v := object.(type) {
				case *v1beta1.App:
					v.Spec.Platform = "go"
					return nil
				case *v1beta1.Platform:
					v.Spec.Image = "shipasoftware/go:v1.2"
					return nil
				}
				t.Fail()
				return errors.New("unknown type")
			},
			request: &CreateImageFromSourceRequest{
				Image:   "acme/superimage",
				AppName: "acmeapp",
			},
			expected: &CreateImageFromSourceResponse{
				ImageURI: "docker.io/acme/superimage:latest",
			},
		},
		{
			name: "missing app",
			crdGetFn: func(_ types.NamespacedName, object runtime.Object) error {
				return errors.New("missing app")
			},
			builderFn: func(_ *docker.BuildRequest) (*docker.BuildResponse, error) {
				t.Fail()
				return nil, nil
			},
			request: &CreateImageFromSourceRequest{
				Image:   "acme/superimage",
				AppName: "acmeapp",
			},
			wantErr: true,
		},
		{
			name: "missing platform",
			crdGetFn: func(_ types.NamespacedName, object runtime.Object) error {
				switch v := object.(type) {
				case *v1beta1.App:
					v.Spec.Platform = "go"
					return nil
				case *v1beta1.Platform:
					return errors.New("missing platform")
				}
				t.Fail()
				return errors.New("unknown type")
			},
			builderFn: func(_ *docker.BuildRequest) (*docker.BuildResponse, error) {
				t.Fail()
				return nil, nil
			},
			request: &CreateImageFromSourceRequest{
				Image:   "acme/superimage",
				AppName: "acmeapp",
			},
			wantErr: true,
		},
		{
			name: "failed build",
			builderFn: func(req *docker.BuildRequest) (*docker.BuildResponse, error) {
				return nil, errors.New("failed build")
			},
			crdGetFn: func(_ types.NamespacedName, object runtime.Object) error {
				switch v := object.(type) {
				case *v1beta1.App:
					v.Spec.Platform = "go"
					return nil
				case *v1beta1.Platform:
					v.Spec.Image = "shipasoftware/go:v1.2"
					return nil
				}
				t.Fail()
				return errors.New("unknown type")
			},
			request: &CreateImageFromSourceRequest{
				Image:   "acme/superimage",
				AppName: "acmeapp",
			},
			wantErr: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			builder := &mockBuilder{
				testFn: tc.builderFn,
			}
			k8sClient := &mockGetter{
				testFn: tc.crdGetFn,
			}
			actual, err := GetSourceHandler(builder, k8sClient)(
				context.Background(),
				tc.request,
				WithWorkingDirectory(workingDir),
			)
			if tc.wantErr {
				require.NotNil(t, err)
				return
			}
			require.Nil(t, err)
			require.Equal(t, tc.expected.ImageURI, actual.ImageURI)
			require.Equal(t, 1, builder.calls)
			require.Equal(t, 2, k8sClient.calls)

		})
	}
}
