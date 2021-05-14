package main

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

const builderListHelp = `
List CNCF registered builders
`

type SuggestedBuilder struct {
	Vendor             string
	Image              string
	DefaultDescription string
}

var suggestedBuilders = []SuggestedBuilder{
	{
		Vendor:             "Google",
		Image:              "gcr.io/buildpacks/builder:v1",
		DefaultDescription: "GCP Builder for all runtimes",
	},
	{
		Vendor:             "Heroku",
		Image:              "heroku/buildpacks:18",
		DefaultDescription: "heroku-18 base image with buildpacks for Ruby, Java, Node.js, Python, Golang, & PHP",
	},
	{
		Vendor:             "Heroku",
		Image:              "heroku/buildpacks:20",
		DefaultDescription: "heroku-20 base image with buildpacks for Ruby, Java, Node.js, Python, Golang, & PHP",
	},
	{
		Vendor:             "Paketo Buildpacks",
		Image:              "paketobuildpacks/builder:base",
		DefaultDescription: "Small base image with buildpacks for Java, Node.js, Golang, & .NET Core",
	},
	{
		Vendor:             "Paketo Buildpacks",
		Image:              "paketobuildpacks/builder:full",
		DefaultDescription: "Larger base image with buildpacks for Java, Node.js, Golang, .NET Core, & PHP",
	},
	{
		Vendor:             "Paketo Buildpacks",
		Image:              "paketobuildpacks/builder:tiny",
		DefaultDescription: "Tiny base image (bionic build image, distroless run image) with buildpacks for Golang",
	},
}

func newBuilderListCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List CNCF registered builders",
		Long:  builderListHelp,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			writeBuilders(out)
		},
	}
	return cmd
}

func writeBuilders(out io.Writer) {
	tw := tabwriter.NewWriter(out, 10, 10, 5, ' ', tabwriter.TabIndent)
	for _, builder := range suggestedBuilders {
		fmt.Fprintf(tw, "\t%s:\t%s\t%s\t\n", builder.Vendor, builder.Image, builder.DefaultDescription)
	}

	tw.Flush()
}
