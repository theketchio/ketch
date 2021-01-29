package build

import (
	"context"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/src-d/go-git.v4"

	"github.com/shipa-corp/ketch/internal/docker"
	"github.com/shipa-corp/ketch/internal/errors"
)

type mockBuilder struct {
	buildCalls int
	pushCalls  int
	buildFn    func(req docker.BuildRequest) (*docker.BuildResponse, error)
	pushFn     func(req docker.BuildRequest) error
}

func (mb *mockBuilder) Build(ctx context.Context, req docker.BuildRequest) (*docker.BuildResponse, error) {
	mb.buildCalls += 1
	return mb.buildFn(req)
}

func (mb *mockBuilder) Push(ctx context.Context, req docker.BuildRequest) error {
	mb.pushCalls += 1
	return mb.pushFn(req)
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
		builderFn func(req docker.BuildRequest) (*docker.BuildResponse, error)
		pushFn    func(req docker.BuildRequest) error
		request   *CreateImageFromSourceRequest
		expected  *CreateImageFromSourceResponse
	}{
		{
			name: "happy path",
			builderFn: func(req docker.BuildRequest) (*docker.BuildResponse, error) {
				assert.True(t, fileExists(path.Join(req.BuildDirectory, "Dockerfile")))
				assert.True(t, fileExists(path.Join(req.BuildDirectory, archiveFileName)))
				assert.Equal(t, req.Image, "acme/superimage")
				return &docker.BuildResponse{
					ImageURI: "docker.io/acme/superimage:latest",
				}, nil
			},
			pushFn: func(req docker.BuildRequest) error {
				assert.Equal(t, req.Image, "acme/superimage")
				return nil
			},
			request: &CreateImageFromSourceRequest{
				Image:         "acme/superimage",
				AppName:       "acmeapp",
				PlatformImage: "shipasoftware/go:v1.2",
			},
			expected: &CreateImageFromSourceResponse{
				ImageURI: "docker.io/acme/superimage:latest",
			},
		},
		{
			name: "failed build",
			builderFn: func(req docker.BuildRequest) (*docker.BuildResponse, error) {
				return nil, errors.New("failed build")
			},
			request: &CreateImageFromSourceRequest{
				Image:   "acme/superimage",
				AppName: "acmeapp",
			},
			wantErr: true,
		},
		{
			name: "push build",
			builderFn: func(req docker.BuildRequest) (*docker.BuildResponse, error) {
				return &docker.BuildResponse{
					ImageURI: "docker.io/acme/superimage:latest",
				}, nil
			},
			pushFn: func(req docker.BuildRequest) error {
				return errors.New("failed push")
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
				buildFn: tc.builderFn,
				pushFn:  tc.pushFn,
			}
			actual, err := GetSourceHandler(builder)(
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
			require.Equal(t, 1, builder.buildCalls)
			require.Equal(t, 1, builder.pushCalls)

		})
	}
}
