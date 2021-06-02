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

type BuilderListEntry struct {
	Vendor      string
	Image       string
	Description string
}

var builderList = []BuilderListEntry{
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

func newBuilderListCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List CNCF registered builders",
		Long:  builderListHelp,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			writeBuilders(cfg, out)
		},
	}
	return cmd
}

func getUserBuilders(cfg config) []BuilderListEntry {
	var userBuilders []BuilderListEntry
	for _, builder := range cfg.GetKetchConfigObject().AdditionalBuilders {
		entry := BuilderListEntry{
			Vendor:      builder.Vendor,
			Image:       builder.Image,
			Description: builder.Description,
		}
		userBuilders = append(userBuilders, entry)
	}

	return userBuilders
}

func writeBuilders(cfg config, out io.Writer) {
	tw := tabwriter.NewWriter(out, 10, 10, 5, ' ', 0)

	builderList = append(builderList, getUserBuilders(cfg)...)

	fmt.Fprintln(tw, "VENDOR\tIMAGE\tDESCRIPTION")
	for _, builder := range builderList {
		fmt.Fprintf(tw, "%s:\t%s\t%s\t\n", builder.Vendor, builder.Image, builder.Description)
	}

	tw.Flush()
}
