package main

import (
	"io"

	"github.com/spf13/cobra"
)

const builderCmdHelp = `
Manage default builder.
`

func newBuilderCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "builder",
		Short: "Manage default builder",
		Long:  builderCmdHelp,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(newBuilderListCmd(out))
	return cmd
}
