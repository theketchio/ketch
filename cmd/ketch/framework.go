package main

import (
	"io"

	"github.com/spf13/cobra"
)

const frameworkHelp = `
Manage frameworks.
`

func newFrameworkCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "framework",
		Short: "Manage frameworks",
		Long:  frameworkHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(newFrameworkListCmd(cfg, out))
	cmd.AddCommand(newFrameworkAddCmd(cfg, out, addFramework))
	cmd.AddCommand(newFrameworkRemoveCmd(cfg, out))
	cmd.AddCommand(newFrameworkUpdateCmd(cfg, out))
	return cmd
}
