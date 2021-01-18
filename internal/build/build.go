// Package build exposes functions to build images from source code.
package build

import (
	"bytes"
	"context"
	"github.com/docker/distribution/reference"
	"github.com/moby/buildkit/client/llb"
	"io"
	"io/ioutil"
	"os"
	"path"
	"text/template"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/errors"
)

const (
	archiveFileName         = "archive.tar.gz"
	archiveFileLocation     = "/home/application/" + archiveFileName
	defaultWorkingDirectory = "/home/application/current"
	tempArchiveDirPrefix    = "ketch-build-*"
)

//type imageService interface {
//	GetCredentials(image string) (*image.Credentials, error)
//}

type resourceGetter interface {
	Get(ctx context.Context, name types.NamespacedName, object runtime.Object) error
}

type solver interface {
	Solve(ctx context.Context, def *llb.Definition, opt client.SolveOpt, statusChan chan *client.SolveStatus) (*client.SolveResponse, error)
}

type CreateImageFromSourceRequest struct {
	ImageURI string
	// source code paths, defaults to the current working directory
	sourcePaths  []string
	// AppName is the name of the application we will deploy to.  It maps to a CRD that contains information
	// pertaining to our build.
	AppName      string
	// defaults to current working directory, use WithWorkingDirectory to override
	workingDir string
	// defaults to stdout, override use WithOutput
	out io.Writer
}

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

func GetSourceHandler(buildClient solver, k8sClient resourceGetter) func(context.Context, *CreateImageFromSourceRequest, ...Option)(*CreateImageFromSourceResponse,error) {
	return func(ctx context.Context, req *CreateImageFromSourceRequest, opts ...Option)(*CreateImageFromSourceResponse,error) {
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

		normalizedResultImageURI, err := normalizeImage(req.ImageURI)
		if err != nil {
			return nil, err
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

		// Set up the build artifacts in a temp file and return the path to the dockerfile we will use for build
		dockerFile, err := buildCtx.getDockerfile(platformImageURI, req.workingDir, req.sourcePaths)



		// call out to buildkit to perform the build and push the result image to a registry
		if err := build(ctx, buildClient, &solveOpts, req.out); err != nil {
			return nil, errors.Wrap(err, "build failed")
		}

		response := &CreateImageFromSourceResponse{
			ImageURI: normalizedResultImageURI,
		}
		return response, nil
	}
}



func normalizeImage(imageURI string)(string,error){
	named, err := reference.ParseNormalizedNamed(imageURI)
	if err != nil {
		return "", errors.Wrap(err, "could not parse image url %q", imageURI)
	}
	return reference.TagNameOnly(named).String(), nil
}

// Retrieve a normalized platform image URI associated with the App.
func getPlatformImageURI(ctx context.Context, platformName string, client resourceGetter) (string, error) {
	var platform ketchv1.Platform
	if err := client.Get(ctx, types.NamespacedName{Name: platformName}, &platform); err != nil {
		return "", errors.Wrap(err, "could not get platform crd %q", platformName)
	}

	return normalizeImage(platform.Spec.Image)
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

func(bc *buildContext) BuildDir() string {
	return bc.ephemeralBuildDir
}

func (bc *buildContext) getDockerfile(platformImage string, workingDir string, sourcePaths []string) (string, error) {
	const sourceDockerfileTemplate = `FROM {{ .PlatformImage }}
COPY . /home/application
WORKDIR /home/application/current
RUN /var/lib/shipa/deploy archive file://{{ .ArchivePath }}
{{- range .Hooks }}
RUN /bin/sh -lc "{{ . }}"
{{- end }}`
	archivePath := path.Join(bc.ephemeralBuildDir, archiveFileName)
	if err := createArchive(archivePath, withWorkingDirectory(workingDir), includeDirs(sourcePaths...)); err != nil {
		return "", err
	}
	hooks, err := getHooks(workingDir, sourcePaths)
	if err != nil {
		return "", errors.Wrap(err, "failed to get hooks")
	}
	templateParams := struct {
		PlatformImage string
		ArchivePath   string
		Hooks         []string
	}{platformImage, archiveFileLocation, hooks}

	tmpl, err := template.New("").Parse(sourceDockerfileTemplate)
	if err != nil {
		return "", err
	}
	var buff bytes.Buffer
	if err = tmpl.Execute(&buff, &templateParams); err != nil {
		return "", err
	}

	dockerFilePath := path.Join(bc.ephemeralBuildDir, "Dockerfile")
	if err = ioutil.WriteFile(dockerFilePath, buff.Bytes(), 0644); err != nil {
		return "", err
	}
	return dockerFilePath, nil
}
