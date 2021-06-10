package main

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v2"

	"github.com/spf13/cobra"

	"github.com/shipa-corp/ketch/cmd/ketch/configuration"
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
			return writeBuilders(ketchConfig, out, cmd.Flags())
		},
	}
	return cmd
}

func writeBuilders(ketchConfig configuration.KetchConfig, out io.Writer, flags *pflag.FlagSet) error {
	tw := tabwriter.NewWriter(out, 10, 10, 5, ' ', 0)

	builderList = append(builderList, ketchConfig.AdditionalBuilders...)
	outputFlag, err := flags.GetString("output")
	if err != nil {
		outputFlag = ""
	}
	switch outputFlag {
	case "json", "JSON":
		j, err := json.MarshalIndent(builderList, "", "\t")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(j))
	case "yaml", "YAML":
		y, err := yaml.Marshal(builderList)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(y))
	default:
		fmt.Fprintln(tw, "VENDOR\tIMAGE\tDESCRIPTION")
		for _, builder := range builderList {
			fmt.Fprintf(tw, "%s:\t%s\t%s\t\n", builder.Vendor, builder.Image, builder.Description)
		}
		tw.Flush()
	}
	return nil
}
