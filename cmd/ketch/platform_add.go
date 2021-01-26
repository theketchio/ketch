package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

const (
	platformAddHelp = `
Adds a new platform.  The platform image can be inferred if you're using an
official platform.

Examples:
  ketch platform add java  # Uses official Shipa image from docker hub
  ketch platform add java -i gcr.io/somerepo/custom-platform:latest  # Custom image from an image repo
  ketch platform add java -d path/to/Dockerfile
  ketch platform add java -d http://some.com/path/Dockerfile   #Reference remote dockerfile 

`
	imageFlag            = "image"
	imageShortFlag       = "i"
	dockerfileFlag       = "dockerfile"
	dockerfileShortFlag  = "d"
	descriptionFlag      = "description"
	descriptionShortFlag = "D"
)

func newPlatformAddCmd(creator resourceCreator, out io.Writer) *cobra.Command {
	var options platformAddOptions
	cmd := &cobra.Command{
		Use:   "add PLATFORM",
		Short: "Add a platform",
		Long:  platformAddHelp,
		Args:  cobra.ExactArgs(1),
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return options.validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			options.name = args[0]
			return platformAdd(cmd.Context(), creator, options, out)
		},
	}

	cmd.Flags().StringVarP(&options.imageReference, imageFlag, imageShortFlag, "", "Image reference for platform")
	cmd.Flags().StringVarP(&options.dockerFileLocation, dockerfileFlag, dockerfileShortFlag, "", "Path or URL to a Dockerfile")
	cmd.Flags().StringVarP(&options.description, descriptionFlag, descriptionShortFlag, "", "A description of the platform")
	return cmd
}

type platformAddOptions struct {
	dockerFileLocation string
	imageReference     string
	description        string
	name               string
}

func (opts platformAddOptions) validate() error {
	if opts.dockerFileLocation != "" && opts.imageReference != "" {
		return fmt.Errorf("%q and %q are mutually exclusive, only one maybe defined", imageFlag, dockerfileFlag)
	}
	return nil
}

// TODO: Implement see https://shipaio.atlassian.net/browse/SHIPA-831?atlOrigin=eyJpIjoiYjJjMzc4NzJhOWZmNGNmMGIyMDk3YzhlODk1NTUzZjgiLCJwIjoiaiJ9
func platformFromDockerfile(ctx context.Context, dockerFileReference string) (string, error) {
	return "", errors.New("build platform from dockerfile not implemented")
}

func platformAdd(ctx context.Context, creator resourceCreator, options platformAddOptions, out io.Writer) error {
	var imageRef string
	var err error
	if options.dockerFileLocation != "" {
		if imageRef, err = platformFromDockerfile(ctx, options.dockerFileLocation); err != nil {
			return err
		}
	}
	if options.imageReference != "" {
		imageRef = options.imageReference
	}
	if options.imageReference == "" && options.dockerFileLocation == "" {
		ref, ok := officialPlatforms()(options.name)
		if !ok {
			return fmt.Errorf("platform image not found for %q", options.name)
		}
		imageRef = ref
	}

	platform := platformSpec{
		name:        options.name,
		image:       imageRef,
		description: options.description,
	}
	if err = platformCreate(ctx, creator, platform); err != nil {
		return fmt.Errorf("could not create platform: %w", err)
	}
	fmt.Fprintf(out, "Added platform %q\n", options.name)
	return nil
}

func officialPlatforms() func(name string) (string, bool) {
	p := map[string]string{
		"cordova": "shipasoftware/cordova:v1.2",
		"dotnet":  "shipasoftware/dotnet:v1.2",
		"elixir":  "shipasoftware/elixir:v1.2",
		"go":      "shipasoftware/go:v1.2",
		"java":    "shipasoftware/java:v1.2",
		"lua":     "shipasoftware/lua:v1.2",
		"nodejs":  "shipasoftware/nodejs:v1.2",
		"perl":    "shipasoftware/perl:v1.2",
		"php":     "shipasoftware/php:v1.2",
		"play":    "shipasoftware/play:v1.2",
		"pypy":    "shipasoftware/pypy:v1.2",
		"python":  "shipasoftware/python:v1.2",
		"ruby":    "shipasoftware/ruby:v1.2",
		"static":  "shipasoftware/static:v1.2",
	}

	return func(name string) (string, bool) {
		imageRef, ok := p[name]
		return imageRef, ok
	}
}
