// Package pack wraps a pack client and is used for building and pushing images
package pack

import (
	"context"
	"io"

	"github.com/buildpacks/pack/pkg/client"
	packConfig "github.com/buildpacks/pack/pkg/image"
	"github.com/buildpacks/pack/pkg/logging"
)

const (
	defaultProcessType = "web"
)

type packService interface {
	Build(ctx context.Context, opts client.BuildOptions) error
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
	buildLogger := logging.NewSimpleLogger(out)
	builder, err := client.NewClient(client.WithLogger(buildLogger))
	if err != nil {
		return nil, err
	}

	return &Client{
		builder: builder,
	}, nil
}

// BuildAndPushImage builds and pushes an image via pack with the specified parameters in BuildRequest
func (c *Client) BuildAndPushImage(ctx context.Context, req BuildRequest) error {
	buildOptions := client.BuildOptions{
		Image:             req.Image,
		Builder:           req.Builder,
		Registry:          "",
		AppPath:           req.WorkingDir,
		RunImage:          "",
		AdditionalMirrors: nil,
		Env:               nil,
		Publish:           true,
		ClearCache:        false,
		TrustBuilder: func(s string) bool {
			return true
		},
		Buildpacks:         req.BuildPacks,
		ProxyConfig:        nil,
		ContainerConfig:    client.ContainerConfig{},
		DefaultProcessType: defaultProcessType,
		PullPolicy:         packConfig.PullIfNotPresent,
	}
	return c.builder.Build(ctx, buildOptions)
}
