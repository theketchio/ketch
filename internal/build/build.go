// Package build exposes functions to build images from source code.
package build

import (
	"context"
	"os"

	"github.com/theketchio/ketch/internal/errors"
	"github.com/theketchio/ketch/internal/pack"
)

type builder interface {
	BuildAndPushImage(ctx context.Context, request pack.BuildRequest) error
}

// CreateImageFromSourceRequest contains fields used to build an image from source code.
type CreateImageFromSourceRequest struct {
	// ID is the id of te application.
	ID string
	// AppName is the name of the application we will deploy to.  It maps to a CRD that contains information
	// pertaining to our build.
	AppName string
	// Image is the name of the image that will be built from source code.
	Image string
	// Builder is the name of the pack builder used to build code
	Builder string
	// BuildPacks list of build packs to include in the build
	BuildPacks []string
	// defaults to current working directory, use WithWorkingDirectory to override. Typically the
	// working directory would be the root of the source code that will be built.
	workingDir string
}

// Option is the signature of options used in GetSourceHandler
type Option func(o *CreateImageFromSourceRequest)

// WithWorkingDirectory override the current working directory as the root directory for source files.
func WithWorkingDirectory(workingDirectory string) Option {
	return func(o *CreateImageFromSourceRequest) {
		o.workingDir = workingDirectory
	}
}

// GetSourceHandler returns a build function. It takes a pack client as an argument.
func GetSourceHandler(packCLI builder) func(context.Context, *CreateImageFromSourceRequest, ...Option) error {
	return func(ctx context.Context, req *CreateImageFromSourceRequest, opts ...Option) error {
		// default to current working directory
		wd, err := os.Getwd()
		if err != nil {
			return errors.Wrap(err, "could not get working directory")
		}

		req.workingDir = wd

		for _, opt := range opts {
			opt(req)
		}

		packRequest := pack.BuildRequest{
			Image:      req.Image,
			Builder:    req.Builder,
			WorkingDir: req.workingDir,
			BuildPacks: req.BuildPacks,
		}
		if err := packCLI.BuildAndPushImage(ctx, packRequest); err != nil {
			return errors.Wrap(err, "could not build image from source")
		}

		return nil
	}
}
