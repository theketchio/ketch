package main

import (
	"io"

	"github.com/spf13/cobra"
)

const unitHelp = `
Manage an app's units
`

func newUnitCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unit",
		Short: "Manage an app's units",
		Long:  unitHelp,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(newUnitAddCmd(cfg, out))
	cmd.AddCommand(newUnitSetCmd(cfg, out))
	cmd.AddCommand(newUnitRemoveCmd(cfg, out))
	return cmd
}
