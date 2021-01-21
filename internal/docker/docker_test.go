package docker

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/stretchr/testify/require"
)

type buildFnT func(tarRdr io.Reader, opts types.ImageBuildOptions) (types.ImageBuildResponse, error)
type pushFnT func(image string, options types.ImagePushOptions) (io.ReadCloser, error)

type mockImageManager struct {
	pushFn      pushFnT
	pushCalled  int
	buildFn     buildFnT
	buildCalled int
}

func (m *mockImageManager) ImageBuild(ctx context.Context, buildContext io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error) {
	m.buildCalled += 1
	return m.buildFn(buildContext, options)
}

func (m *mockImageManager) ImagePush(ctx context.Context, image string, options types.ImagePushOptions) (io.ReadCloser, error) {
	m.pushCalled += 1
	return m.pushFn(image, options)
}

func (m *mockImageManager) Close() error {
	panic("implement me")
}

type mockReadCloser struct {
	bytes.Buffer
}

func (mr *mockReadCloser) Close() error {
	return nil
}

func authEncoder(_ *BuildRequest) (string, error) {
	return "123456abcdef7890", nil
}

const (
	normalBuildAuxLine = `{"aux":{"ID":"sha256:8741ab561337f6795e5ae0206279dae98bc4292c746e7728ffb9967a5d5eb1e8"}}
`
	normalPushAuxLine = `{"progressDetail":{},"aux":{"Tag":"v0.1","Digest":"sha256:e9ba0580ada07d66fb46f581d524327d3f63933474bc35ff3f1943e5a9099630","Size":3042}}
`
	errorLine = `{"error": "something bad happened", "errorDetail":{"message":"antimatter explosion"}}
`
)

func TestBuild(t *testing.T) {
	tmpDir, err := ioutils.TempDir("", "test-build-*")
	require.Nil(t, err)
	myDocker := `FROM shipasoftware/go:v1.2
	USER root
	COPY . /home/application
	WORKDIR /home/application/current
	RUN /var/lib/shipa/deploy archive file://archive.tar.gz
`
	err = ioutil.WriteFile(path.Join(tmpDir, "Dockerfile"), []byte(myDocker), 0644)
	require.Nil(t, err)

	tt := []struct {
		name      string
		buildFn   buildFnT
		pushFn    pushFnT
		request   *BuildRequest
		expected  *BuildResponse
		wantError bool
	}{
		{
			name: "happy path",
			request: &BuildRequest{
				Image:          "some/app:v0.1",
				BuildDirectory: tmpDir,
				Out:            os.Stdout,
			},
			expected: &BuildResponse{
				ImageURI: "docker.io/some/app:v0.1",
			},
			buildFn: func(_ io.Reader, opts types.ImageBuildOptions) (types.ImageBuildResponse, error) {
				var resp types.ImageBuildResponse
				var body mockReadCloser
				body.WriteString(normalBuildAuxLine)
				resp.Body = &body
				return resp, nil
			},
			pushFn: func(_ string, _ types.ImagePushOptions) (io.ReadCloser, error) {
				var resp mockReadCloser
				resp.WriteString(normalPushAuxLine)
				return &resp, nil
			},
		},
		{
			name: "build error",
			request: &BuildRequest{
				Image:          "some/app:v0.1",
				BuildDirectory: tmpDir,
				Out:            os.Stdout,
			},
			expected: &BuildResponse{
				ImageURI: "docker.io/some/app:v0.1",
			},
			buildFn: func(_ io.Reader, opts types.ImageBuildOptions) (types.ImageBuildResponse, error) {
				var resp types.ImageBuildResponse
				var body mockReadCloser
				body.WriteString(errorLine)
				resp.Body = &body
				return resp, nil
			},
			pushFn: func(_ string, _ types.ImagePushOptions) (io.ReadCloser, error) {
				t.Fail()
				return nil, nil
			},
			wantError: true,
		},
		{
			name: "push error",
			request: &BuildRequest{
				Image:          "some/app:v0.1",
				BuildDirectory: tmpDir,
				Out:            os.Stdout,
			},
			expected: &BuildResponse{
				ImageURI: "docker.io/some/app:v0.1",
			},
			buildFn: func(_ io.Reader, opts types.ImageBuildOptions) (types.ImageBuildResponse, error) {
				var resp types.ImageBuildResponse
				var body mockReadCloser
				body.WriteString(normalBuildAuxLine)
				resp.Body = &body
				return resp, nil
			},
			pushFn: func(_ string, _ types.ImagePushOptions) (io.ReadCloser, error) {
				var resp mockReadCloser
				resp.WriteString(errorLine)
				return &resp, nil
			},
			wantError: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			cli := Client{
				manager:      &mockImageManager{pushFn: tc.pushFn, buildFn: tc.buildFn},
				authEncodeFn: authEncoder,
			}

			actual, err := cli.Build(context.Background(), tc.request)
			if tc.wantError {
				require.NotNil(t, err)
				return
			}
			require.Nil(t, err)
			require.Equal(t, tc.expected.ImageURI, actual.ImageURI)
		})
	}

}
