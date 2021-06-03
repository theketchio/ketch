package build

import (
	"context"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/src-d/go-git.v4"

	"github.com/shipa-corp/ketch/internal/errors"
	"github.com/shipa-corp/ketch/internal/pack"
)

type mockBuilder struct {
	buildAndPushCalls int
	buildAndPushFn    func(ctx context.Context, req pack.BuildRequest) error
}

func (mb *mockBuilder) BuildAndPushImage(ctx context.Context, req pack.BuildRequest) error {
	mb.buildAndPushCalls += 1
	return mb.buildAndPushFn(ctx, req)
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

func TestGetSourceHandler(t *testing.T) {
	workingDir, err := cloneSource(t.TempDir(), "https://github.com/shipa-corp/go-sample")
	require.Nil(t, err)

	tt := []struct {
		name      string
		wantErr   bool
		builderFn func(ctx context.Context, req pack.BuildRequest) error
		request   *CreateImageFromSourceRequest
	}{
		{
			name: "happy path",
			builderFn: func(ctx context.Context, req pack.BuildRequest) error {
				assert.Equal(t, req.Image, "acme/superimage")
				assert.Equal(t, req.WorkingDir, workingDir)
				return nil
			},
			request: &CreateImageFromSourceRequest{
				Image:   "acme/superimage",
				AppName: "acmeapp",
				Builder: "heroku/buildpacks:18",
			},
		},
		{
			name: "failed build",
			builderFn: func(ctx context.Context, req pack.BuildRequest) error {
				return errors.New("failed build")
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
				buildAndPushFn: tc.builderFn,
			}
			err := GetSourceHandler(builder)(
				context.Background(),
				tc.request,
				WithWorkingDirectory(workingDir),
			)
			if tc.wantErr {
				require.NotNil(t, err)
				return
			}
			require.Nil(t, err)
			require.Equal(t, 1, builder.buildAndPushCalls)

		})
	}
}
