// Package build exposes functions to build images from source code.
package build

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path"
	"text/template"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/archive"
	"github.com/shipa-corp/ketch/internal/docker"
	"github.com/shipa-corp/ketch/internal/errors"
)

const (
	archiveFileName      = "archive.tar.gz"
	archiveFileLocation  = "/home/application/" + archiveFileName
	tempArchiveDirPrefix = "ketch-build-*"
)

type resourceGetter interface {
	Get(ctx context.Context, name types.NamespacedName, object runtime.Object) error
}

type builder interface {
	Build(ctx context.Context, request *docker.BuildRequest) (*docker.BuildResponse, error)
}

// CreateImageFromSource request contains fields used to build an image from source code.
type CreateImageFromSourceRequest struct {
	// AppName is the name of the application we will deploy to.  It maps to a CRD that contains information
	// pertaining to our build.
	AppName string
	// Image is the name of the image that will be built from source code
	Image string

	// source code paths, relative to the working directory. Use WithSourcePaths to override.
	sourcePaths []string
	// defaults to current working directory, use WithWorkingDirectory to override. Typically the
	// working directory would be the root of the source code that will be built
	workingDir string
	// defaults to stdout, override use WithOutput
	out io.Writer
}

// CreateImageFromSourceResponse is returned from the build handler function and contains the
// fully qualified image name that was built.
type CreateImageFromSourceResponse struct {
	ImageURI string
}

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

// GetSourceHandler returns a build function. It takes a docker client, and a k8s client as arguments.
func GetSourceHandler(dockerCli builder, k8sClient resourceGetter) func(context.Context, *CreateImageFromSourceRequest, ...Option) (*CreateImageFromSourceResponse, error) {
	return func(ctx context.Context, req *CreateImageFromSourceRequest, opts ...Option) (*CreateImageFromSourceResponse, error) {
		// default to current working directory
		wd, err := os.Getwd()
		if err != nil {
			return nil, errors.Wrap(err, "could not get working directory")
		}

		req.workingDir = wd
		// build output default to stdout
		req.out = os.Stdout

		// source code default directory is current dir
		req.sourcePaths = []string{"."}

		for _, opt := range opts {
			opt(req)
		}

		// get the application CRD containing platform and other stuff we need to build
		var app ketchv1.App
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: req.AppName}, &app); err != nil {
			return nil, errors.Wrap(err, "could not get app")
		}

		// get the image for the platform we will use to build
		platformImageURI, err := getPlatformImageURI(ctx, app.Spec.Platform, k8sClient)
		if err != nil {
			return nil, err
		}

		buildCtx, err := newBuildContext()
		if err != nil {
			return nil, err
		}
		defer buildCtx.close()

		// prepare the build directory with an archive containing sources and a docker file
		if err := buildCtx.prepare(platformImageURI, req.workingDir, req.sourcePaths); err != nil {
			return nil, err
		}

		// create an image that contains our built source and push image to target registry
		resp, err := dockerCli.Build(
			ctx,
			&docker.BuildRequest{
				Image:          req.Image,
				BuildDirectory: buildCtx.BuildDir(),
				Out:            req.out,
			},
		)
		if err != nil {
			return nil, err
		}

		response := &CreateImageFromSourceResponse{
			ImageURI: resp.ImageURI,
		}
		return response, nil
	}
}

// Retrieve a normalized platform image URI associated with the App.
func getPlatformImageURI(ctx context.Context, platformName string, client resourceGetter) (string, error) {
	var platform ketchv1.Platform
	if err := client.Get(ctx, types.NamespacedName{Name: platformName}, &platform); err != nil {
		return "", errors.Wrap(err, "could not get platform crd %q", platformName)
	}

	return docker.NormalizeImage(platform.Spec.Image)
}

type buildContext struct {
	ephemeralBuildDir string
}

func newBuildContext() (*buildContext, error) {
	var ctx buildContext
	buildDir, err := ioutil.TempDir(os.TempDir(), tempArchiveDirPrefix)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate temp dir for build")
	}
	ctx.ephemeralBuildDir = buildDir
	return &ctx, nil
}

// Call close when done with build context to clean up file system resources used for build.
func (bc *buildContext) close() {
	_ = os.RemoveAll(bc.ephemeralBuildDir)
}

// BuildDir contains directory where generated Dockerfile and source archive is located
func (bc *buildContext) BuildDir() string {
	return bc.ephemeralBuildDir
}

// Prepare the build. Create a temp directory containing a docker file, and an archive containing source codes.
func (bc *buildContext) prepare(platformImage string, workingDir string, sourcePaths []string) error {
	const sourceDockerfileTemplate = `FROM {{ .PlatformImage }}
USER root
COPY . /home/application
WORKDIR /home/application/current
RUN /var/lib/shipa/deploy archive file://{{ .ArchivePath }}
{{- range .Hooks }}
RUN /bin/sh -lc "{{ . }}"
{{- end }}`
	archivePath := path.Join(bc.ephemeralBuildDir, archiveFileName)
	err := archive.Create(
		archivePath,
		archive.WithWorkingDirectory(workingDir),
		archive.IncludeDirs(sourcePaths...),
	)
	if err != nil {
		return errors.Wrap(err, "could not create archive %q", archivePath)
	}
	hooks, err := getHooks(workingDir, sourcePaths)
	if err != nil {
		return errors.Wrap(err, "failed to get hooks")
	}
	templateParams := struct {
		PlatformImage string
		ArchivePath   string
		Hooks         []string
	}{platformImage, archiveFileLocation, hooks}

	tmpl, err := template.New("").Parse(sourceDockerfileTemplate)
	if err != nil {
		return errors.Wrap(err, "could not generate dockerfile")
	}
	var buff bytes.Buffer
	if err = tmpl.Execute(&buff, &templateParams); err != nil {
		return errors.Wrap(err, "could not generate dockerfile")
	}

	dockerFilePath := path.Join(bc.ephemeralBuildDir, "Dockerfile")
	if err = ioutil.WriteFile(dockerFilePath, buff.Bytes(), 0644); err != nil {
		return errors.Wrap(err, "could not write docker file to %q", dockerFilePath)
	}
	return nil
}
