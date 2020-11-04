package main

import (
	"io"

	"github.com/spf13/cobra"
)

const envCmdHelp = `
Manage an app's environment variables.
`

func newEnvCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage an app's environment variables",
		Long:  envCmdHelp,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(newEnvSetCmd(cfg, out))
	cmd.AddCommand(newEnvGetCmd(cfg, out))
	cmd.AddCommand(newEnvUnsetCmd(cfg, out))
	return cmd
}
