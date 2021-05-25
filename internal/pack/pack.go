// Package pack wraps a pack client and is used for building and pushing images
package pack

import (
	"context"
	"github.com/buildpacks/pack"
	packConfig "github.com/buildpacks/pack/config"
	"github.com/buildpacks/pack/logging"
	"io"
)

const (
	defaultProcessType = "web"
)

type packService interface {
	Build(ctx context.Context, opts pack.BuildOptions) error
}

// BuildRequest contains parameters for the Build command
type BuildRequest struct {
	Image      string
	Builder    string
	WorkingDir string
	BuildPacks []string
}

// Client wrapper around the pack client
type Client struct {
	builder packService
}

func New(out io.Writer) (*Client, error) {
	buildLogger := logging.New(out)
	builder, err := pack.NewClient(pack.WithLogger(buildLogger))
	if err != nil {
		return nil, err
	}

	return &Client{
		builder: builder,
	}, nil
}

// BuildAndPushImage builds and pushes an image via pack with the specified parameters in BuildRequest
func (c *Client) BuildAndPushImage(ctx context.Context, req BuildRequest) error {
	buildOptions := pack.BuildOptions{
		Image:              req.Image,
		Builder:            req.Builder,
		Registry:           "",
		AppPath:            req.WorkingDir,
		RunImage:           "",
		AdditionalMirrors:  nil,
		Env:                nil,
		Publish:            true,
		ClearCache:         false,
		TrustBuilder:       true,
		Buildpacks:         req.BuildPacks,
		ProxyConfig:        nil,
		ContainerConfig:    pack.ContainerConfig{},
		DefaultProcessType: defaultProcessType,
		FileFilter:         nil,
		PullPolicy:         packConfig.PullIfNotPresent,
	}
	return c.builder.Build(ctx, buildOptions)
}
