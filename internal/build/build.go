// Package build exposes functions to build images from source code.
package build

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/buildpacks/pack"
	packConfig "github.com/buildpacks/pack/config"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/archive"
	"github.com/shipa-corp/ketch/internal/chart"
	"github.com/shipa-corp/ketch/internal/docker"
	"github.com/shipa-corp/ketch/internal/errors"
)

const (
	archiveFileName     = "archive.tar.gz"
	archiveFileLocation = "/home/application/" + archiveFileName
	defaultProcessType  = "web"
)

type resourceGetter interface {
	Get(ctx context.Context, name types.NamespacedName, object runtime.Object) error
}

type builder interface {
	Push(ctx context.Context, req docker.BuildRequest) error
	Build(ctx context.Context, request docker.BuildRequest) (*docker.BuildResponse, error)
}

// CreateImageFromSource request contains fields used to build an image from source code.
type CreateImageFromSourceRequest struct {
	// AppName is the name of the application we will deploy to.  It maps to a CRD that contains information
	// pertaining to our build.
	AppName string
	// Image is the name of the image that will be built from source code.
	Image string
	// PlatformImage is the name of the image to be used with FROM statement.
	PlatformImage string
	// source code paths, relative to the working directory. Use WithSourcePaths to override.
	sourcePaths []string
	// defaults to current working directory, use WithWorkingDirectory to override. Typically the
	// working directory would be the root of the source code that will be built.
	workingDir string
	// defaults to stdout, override use WithOutput.
	out io.Writer
	// optional build hooks from ketch.yaml.
	hooks []string
}

type CreateImageFromSourceRequestPack struct {
	// AppName is the name of the application we will deploy to.  It maps to a CRD that contains information
	// pertaining to our build.
	AppName string
	// Image is the name of the image that will be built from source code.
	Image string
	// Builder is the name of the pack builder used to build code
	Builder    string
	BuildPacks []string
	// source code paths, relative to the working directory. Use WithSourcePaths to override.
	sourcePaths []string
	// defaults to current working directory, use WithWorkingDirectory to override. Typically the
	// working directory would be the root of the source code that will be built.
	workingDir string
	// defaults to stdout, override use WithOutput.
	out io.Writer
	// optional build hooks from ketch.yaml.
	hooks []string
}

// CreateImageFromSourceResponse is returned from the build handler function and contains the
// fully qualified image name that was built.
type CreateImageFromSourceResponse struct {
	ImageURI string
	Procfile *chart.Procfile
}

// Option is the signature of options used in GetSourceHandler
type Option func(o *CreateImageFromSourceRequest)

// WithWorkingDirectory override the current working directory as the root directory for source files.
func WithWorkingDirectory(workingDirectory string) Option {
	return func(o *CreateImageFromSourceRequest) {
		o.workingDir = workingDirectory
	}
}

// WithOutput override stdout to receive build messages
func WithOutput(w io.Writer) Option {
	return func(o *CreateImageFromSourceRequest) {
		o.out = w
	}
}

// WithSourcePaths sets the paths that contain build artifacts such as source code and config files that will
// be build or executed on the image.  SourcePaths are relative to the current working directory.  The working
// directory can be overridden using the WithWorkingDirectory option function.
func WithSourcePaths(paths ...string) Option {
	return func(o *CreateImageFromSourceRequest) {
		o.sourcePaths = paths
	}
}

// MaybeWithBuildHooks sets build hooks if they are read from ketch.yaml
func MaybeWithBuildHooks(v *ketchv1.KetchYamlData) Option {
	var hooks []string
	if v != nil {
		if v.Hooks != nil {
			hooks = v.Hooks.Build
		}
	}
	return func(o *CreateImageFromSourceRequest) {
		o.hooks = hooks
	}
}

type packService interface {
	Build(ctx context.Context, opts pack.BuildOptions) error
}

//temporarily hijacking the request and the opts
func GetSourceHandlerPack(packCLI packService) func(context.Context, *CreateImageFromSourceRequest, ...Option) (*CreateImageFromSourceResponse, error) {
	return func(ctx context.Context, req *CreateImageFromSourceRequest, opts ...Option) (*CreateImageFromSourceResponse, error) {
		log.Println("in the builder")
		// default to current working directory
		wd, err := os.Getwd()
		if err != nil {
			return nil, errors.Wrap(err, "could not get working directory")
		}

		req.workingDir = wd
		// build output default to stdout
		req.out = os.Stdout

		req.sourcePaths = archive.DefaultSourcePaths()

		for _, opt := range opts {
			opt(req)
		}

		buildOptions := pack.BuildOptions{
			Image:              req.Image,
			Builder:            req.PlatformImage,
			Registry:           "",
			AppPath:            req.workingDir,
			RunImage:           "",
			AdditionalMirrors:  nil,
			Env:                nil,
			Publish:            true,
			ClearCache:         false,
			TrustBuilder:       true,
			Buildpacks:         nil,
			ProxyConfig:        nil,
			ContainerConfig:    pack.ContainerConfig{},
			DefaultProcessType: defaultProcessType,
			FileFilter:         nil,
			PullPolicy:         packConfig.PullIfNotPresent,
		}
		if err := packCLI.Build(ctx, buildOptions); err != nil {
			return nil, err
		}
		log.Println("success!! image should be built")
		/*buildCtx, err := newBuildContext()
		if err != nil {
			return nil, err
		}
		defer buildCtx.close()

		// prepare the build directory with an archive containing sources and a docker file
		if err := buildCtx.prepare(req.PlatformImage, req.workingDir, req.sourcePaths, req.hooks); err != nil {
			return nil, err
		}

		buildReq := docker.BuildRequest{
			Image:          req.Image,
			BuildDirectory: buildCtx.BuildDir(),
			Out:            req.out,
		}
		// create an image that contains our built source
		buildResponse, err := dockerCli.Build(ctx, buildReq)
		if err != nil {
			return nil, err
		}

		var procfile *chart.Procfile
		if len(buildResponse.Procfile) > 0 {
			procfile, err = chart.ParseProcfile(buildResponse.Procfile)
			if err != nil {
				return nil, err
			}
		}

		// push the image to target registry
		err = dockerCli.Push(ctx, buildReq)
		if err != nil {
			return nil, err
		}*/

		var procfile *chart.Procfile

		response := &CreateImageFromSourceResponse{
			ImageURI: req.Image,
			Procfile: procfile,
		}
		return response, nil
	}
}

// GetSourceHandler returns a build function. It takes a docker client, and a k8s client as arguments.
func GetSourceHandler(dockerCli builder) func(context.Context, *CreateImageFromSourceRequest, ...Option) (*CreateImageFromSourceResponse, error) {
	return func(ctx context.Context, req *CreateImageFromSourceRequest, opts ...Option) (*CreateImageFromSourceResponse, error) {
		// default to current working directory
		wd, err := os.Getwd()
		if err != nil {
			return nil, errors.Wrap(err, "could not get working directory")
		}

		req.workingDir = wd
		// build output default to stdout
		req.out = os.Stdout

		req.sourcePaths = archive.DefaultSourcePaths()

		for _, opt := range opts {
			opt(req)
		}

		buildCtx, err := newBuildContext()
		if err != nil {
			return nil, err
		}
		defer buildCtx.close()

		// prepare the build directory with an archive containing sources and a docker file
		if err := buildCtx.prepare(req.PlatformImage, req.workingDir, req.sourcePaths, req.hooks); err != nil {
			return nil, err
		}

		buildReq := docker.BuildRequest{
			Image:          req.Image,
			BuildDirectory: buildCtx.BuildDir(),
			Out:            req.out,
		}
		// create an image that contains our built source
		buildResponse, err := dockerCli.Build(ctx, buildReq)
		if err != nil {
			return nil, err
		}

		var procfile *chart.Procfile
		if len(buildResponse.Procfile) > 0 {
			procfile, err = chart.ParseProcfile(buildResponse.Procfile)
			if err != nil {
				return nil, err
			}
		}

		// push the image to target registry
		err = dockerCli.Push(ctx, buildReq)
		if err != nil {
			return nil, err
		}

		response := &CreateImageFromSourceResponse{
			ImageURI: buildResponse.ImageURI,
			Procfile: procfile,
		}
		return response, nil
	}
}
