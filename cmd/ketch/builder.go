package main

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/shipa-corp/ketch/cmd/ketch/configuration"
)

const builderCmdHelp = `
Manage pack builders.

A builder is an image that contains all the components needed to build your project into an image.
There are already a number of builders available for use by all developers, as well as the option to build and use your own.

You can learn more about builders at: https://buildpacks.io/docs/concepts/components/builder/
`

func newBuilderCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "builder",
		Short: "Manage pack builders",
		Long:  builderCmdHelp,
		Args:  cobra.NoArgs,
	}

	configStruct := cfg.(*configuration.Configuration)

	cmd.AddCommand(newBuilderListCmd(configStruct.KetchConfig, out))
	return cmd
}
