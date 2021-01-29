package build

import (
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"text/template"

	"github.com/shipa-corp/ketch/internal/archive"
	"github.com/shipa-corp/ketch/internal/errors"
)

const (
	tempArchiveDirPrefix = "ketch-build-*"
)

type buildContext struct {
	ephemeralBuildDir string
}

func newBuildContext() (*buildContext, error) {
	buildDir, err := ioutil.TempDir(os.TempDir(), tempArchiveDirPrefix)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate temp dir for build")
	}
	return &buildContext{ephemeralBuildDir: buildDir}, nil
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
func (bc *buildContext) prepare(platformImage string, workingDir string, sourcePaths, hooks []string) error {
	// TODO: Replace with go:embed feature once Go 1.16 is released.
	const sourceDockerfileTemplate = `FROM {{ .PlatformImage }}
USER root
COPY . /home/application
WORKDIR /home/application/current
RUN chown ubuntu:ubuntu -R /home/application
USER ubuntu
RUN /var/lib/shipa/deploy archive file://{{ .ArchiveFileLocation }}
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

	templateParams := struct {
		PlatformImage       string
		ArchiveFileLocation string
		Hooks               []string
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
