package main

import (
	"io"

	"github.com/spf13/cobra"
)

const builderCmdHelp = `
Manage builders.
`

func newBuilderCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "builder",
		Short: builderCmdHelp,
		Long:  builderCmdHelp,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(newBuilderListCmd(cfg, out))
	return cmd
}
