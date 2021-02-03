// Package docker wraps a docker client and is used to build and push images
package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"

	"github.com/shipa-corp/ketch/internal/errors"
)

const (
	procfileLocation = "home/application/current/Procfile"
)

type imageManager interface {
	ImageBuild(ctx context.Context, buildContext io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error)
	ImagePush(ctx context.Context, image string, options types.ImagePushOptions) (io.ReadCloser, error)
	ImageSave(ctx context.Context, imageIDs []string) (io.ReadCloser, error)
	Close() error
}

// Client maintains state for Docker API.
type Client struct {
	manager      imageManager
	authEncodeFn func(req BuildRequest) (string, error)
	getProcfile  func(imageSaver imageSaver, imageId string) (string, error)
}

// BuildRequest contains parameters for the Build command
type BuildRequest struct {
	// Tagged image name such as myrepo/myimage:v0.1
	Image string
	// BuildDirectory root directory containing Dockerfile and source file archive
	BuildDirectory string
	// Out streams messages sent back from docker daemon.
	Out io.Writer
	// AuthConfig optional auth config that could be from a k8s secret or supplied on the
	// command line if you don't want to use your local docker credentials.
	AuthConfig *types.AuthConfig
}

// BuildResponse is returned from successful invocation of Build. It contains the fully qualified
// ImageURI that we pushed to the destination registry.
type BuildResponse struct {
	ImageURI string
	Procfile string
}

func domain(img string) (string, error) {
	named, err := reference.ParseNormalizedNamed(img)
	if err != nil {
		return "", err
	}
	return reference.Domain(named), nil
}

// NormalizeImage will convert an image into a fully qualified form with the registry host and a tag.
func NormalizeImage(imageURI string) (string, error) {
	named, err := reference.ParseNormalizedNamed(imageURI)
	if err != nil {
		return "", errors.Wrap(err, "could not parse image url %q", imageURI)
	}
	return reference.TagNameOnly(named).String(), nil
}

// New creates a docker client.
func New() (*Client, error) {

	// Docker client version 19.03.09 doesn't automatically downgrade Docker API version correctly
	// https://github.com/docker/cli/issues/2533

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.39"))
	if err != nil {
		return nil, err
	}
	resp := Client{
		manager:     cli,
		getProcfile: getProcfile,
	}

	resp.authEncodeFn = func(req BuildRequest) (string, error) {
		if req.AuthConfig != nil {
			jsonAuth, err := json.Marshal(req.AuthConfig)
			if err != nil {
				return "", errors.Wrap(err, "could not marshal auth config")
			}
			return base64.URLEncoding.EncodeToString(jsonAuth), nil
		}

		repo, err := domain(req.Image)
		if err != nil {
			return "", err
		}
		encodedAuth, err := getEncodedRegistryAuth(repo)
		if err != nil {
			return "", err
		}
		return encodedAuth, nil
	}
	return &resp, nil
}

// Close frees up the connection to the docker daemon and should always be called when we're done with the Client.
func (c *Client) Close() error {
	return c.manager.Close()
}

// Push pushes the given image to a remote repository. It uses the docker credentials
// of the local user.
func (c *Client) Push(ctx context.Context, req BuildRequest) error {
	normedImage, err := NormalizeImage(req.Image)
	if err != nil {
		return err
	}
	encodedAuth, err := c.authEncodeFn(req)
	if err != nil {
		return err
	}

	pushResp, err := c.manager.ImagePush(
		ctx,
		normedImage,
		types.ImagePushOptions{
			RegistryAuth: encodedAuth,
		},
	)
	if err != nil {
		return errors.Wrap(err, "failed to push image %q", normedImage)
	}

	if err := print(pushResp, req.Out); err != nil {
		return errors.Wrap(err, "push failed")
	}

	return nil
}

// Build will create a docker image and push it to a remote repository. It will use the docker credentials
// of the local user.
func (c *Client) Build(ctx context.Context, req BuildRequest) (*BuildResponse, error) {
	buildCtx, err := archive.TarWithOptions(req.BuildDirectory, &archive.TarOptions{})
	if err != nil {
		return nil, err
	}
	// ImageBuild doesn't return an error if there is a problem with the build, we need to
	// capture that in the out stream of the build, see print.
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
	procfile, err := c.getProcfile(c.manager, req.Image)
	if err != nil {
		return nil, err
	}
	normedImage, err := NormalizeImage(req.Image)
	if err != nil {
		return nil, err
	}
	response := &BuildResponse{
		ImageURI: normedImage,
		Procfile: procfile,
	}
	return response, nil
}

type imageSaver interface {
	ImageSave(ctx context.Context, imageIDs []string) (io.ReadCloser, error)
}

// getProcfile exracts content of the given image, looks for Procfile and returns it.
func getProcfile(imageSaver imageSaver, imageId string) (string, error) {
	reader, err := imageSaver.ImageSave(context.TODO(), []string{imageId})
	if err != nil {
		return "", err
	}
	defer reader.Close()

	tarReader := tar.NewReader(reader)
	var procfile string
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if header.FileInfo().Name() == "layer.tar" {
			layerReader := tar.NewReader(tarReader)
			for {
				header, err := layerReader.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					return "", err
				}
				if header.Typeflag == tar.TypeReg && header.Name == procfileLocation {
					buf := &bytes.Buffer{}
					_, err := io.Copy(buf, layerReader)
					if err != nil {
						return "", err
					}
					procfile = buf.String()
				}
			}
		}
	}
	return procfile, nil
}
