package main

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/shipa-corp/ketch/cmd/ketch/configuration"
	"github.com/shipa-corp/ketch/cmd/ketch/output"
)

const builderListHelp = `
List CNCF registered builders, along with any additional builders defined by the user in config.toml (default path: $HOME/.ketch)
`

var builderList = []configuration.AdditionalBuilder{
	{
		Vendor:      "Google",
		Image:       "gcr.io/buildpacks/builder:v1",
		Description: "GCP Builder for all runtimes",
	},
	{
		Vendor:      "Heroku",
		Image:       "heroku/buildpacks:18",
		Description: "heroku-18 base image with buildpacks for Ruby, Java, Node.js, Python, Golang, & PHP",
	},
	{
		Vendor:      "Heroku",
		Image:       "heroku/buildpacks:20",
		Description: "heroku-20 base image with buildpacks for Ruby, Java, Node.js, Python, Golang, & PHP",
	},
	{
		Vendor:      "Paketo Buildpacks",
		Image:       "paketobuildpacks/builder:base",
		Description: "Small base image with buildpacks for Java, Node.js, Golang, & .NET Core",
	},
	{
		Vendor:      "Paketo Buildpacks",
		Image:       "paketobuildpacks/builder:full",
		Description: "Larger base image with buildpacks for Java, Node.js, Golang, .NET Core, & PHP",
	},
	{
		Vendor:      "Paketo Buildpacks",
		Image:       "paketobuildpacks/builder:tiny",
		Description: "Tiny base image (bionic build image, distroless run image) with buildpacks for Golang",
	},
}

func newBuilderListCmd(ketchConfig configuration.KetchConfig, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list builders",
		Long:  builderListHelp,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			outputFlag, _ := cmd.Flags().GetString("output")
			return output.Write(append(builderList, ketchConfig.AdditionalBuilders...), out, outputFlag)
		},
	}
	cmd.Flags().StringP("output", "o", "", "used to specify output, e.g. --output format=json")
	return cmd
}
