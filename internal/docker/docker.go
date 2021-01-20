// Package docker wraps a docker client and is used to build and push images
package docker

import (
	"context"
	"encoding/base64"
	"io"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/shipa-corp/ketch/internal/errors"
)

type imageManager interface {
	ImageBuild(ctx context.Context, buildContext io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error)
	ImagePush(ctx context.Context, image string, options types.ImagePushOptions) (io.ReadCloser, error)
	Close() error
}

type Client struct {
	manager imageManager
	authEncodeFn func(br *BuildRequest)(string,error)
}

// BuildRequest contains parameters for the Build command
type BuildRequest struct {
	// Tagged image name such as myrepo/myimage:v0.1
	Image string
	// Repository i.e. gcr.io
	//Repository string
	// BuildDirectory root directory containing Dockerfile and source file archive
	BuildDirectory string
	// Out
	Out io.Writer
	// DockerConfigDir is the path to where the docker configuration is located.
	DockerConfigDir string
	// AuthConfig optional auth config that could be from a k8s secret or supplied on the
	// command line
	AuthConfig *types.AuthConfig
	// Insecure true if the repository doesn't use TLS
	Insecure bool
}



func Domain(img string) (string, error) {
	named, err := reference.ParseNormalizedNamed(img)
	if err != nil {
		return "", err
	}
	return reference.Domain(named), nil
}

func NormalizeImage(imageURI string) (string, error) {
	named, err := reference.ParseNormalizedNamed(imageURI)
	if err != nil {
		return "", errors.Wrap(err, "could not parse image url %q", imageURI)
	}
	return reference.TagNameOnly(named).String(), nil
}

type BuildResponse struct {
	ImageURI string
}

func New() (*Client, error) {
	var resp Client
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	resp.manager = cli

	resp.authEncodeFn = func(req *BuildRequest)(string, error) {
		if req.AuthConfig != nil {
			jsonAuth, err := json.Marshal(req.AuthConfig)
			if err != nil {
				return "", errors.Wrap(err, "could not marshal auth config")
			}
			return base64.URLEncoding.EncodeToString(jsonAuth), nil
		}

		repo, err := Domain(req.Image)
		if err != nil {
			return "", err
		}
		encodedAuth, err := getEncodedRegistryAuth(req.DockerConfigDir, repo, req.Insecure )
		if err != nil {
			return "", err
		}
		return encodedAuth, nil
	}

	return &resp, nil
}

func (c *Client) Close() error {
	return c.manager.Close()
}

func (c *Client) Build(ctx context.Context, req *BuildRequest) (*BuildResponse, error) {
	buildCtx, err := archive.TarWithOptions(req.BuildDirectory, &archive.TarOptions{})
	if err != nil {
		return nil, err
	}
	resp, err := c.manager.ImageBuild(
		ctx,
		buildCtx,
		types.ImageBuildOptions{
			Dockerfile: "Dockerfile",
			Tags:       []string{req.Image},
			NoCache:    true,
			Remove:     true,
		},
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build image %q", req.Image)
	}

	if err := print(resp.Body, req.Out); err != nil {
		return nil, errors.Wrap(err, "build failed")
	}

	normedImage, err := NormalizeImage(req.Image)
	if err != nil {
		return nil, err
	}

	encodedAuth, err := c.authEncodeFn(req)
	if err != nil {
		return nil, err
	}

	pushResp, err := c.manager.ImagePush(
		ctx,
		normedImage,
		types.ImagePushOptions{
			RegistryAuth: encodedAuth,
		},
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to push image %q", normedImage)
	}

	if err := print(pushResp, req.Out); err != nil {
		return nil, errors.Wrap(err, "push failed")
	}

	return &BuildResponse{ImageURI: normedImage}, nil
}
