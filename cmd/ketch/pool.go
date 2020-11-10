package main

import (
	"io"

	"github.com/spf13/cobra"
)

const poolHelp = `
Manage pools.
`

func newPoolCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pool",
		Short: "Manage pools",
		Long:  poolHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(newPoolListCmd(cfg, out))
	cmd.AddCommand(newPoolAddCmd(cfg, out, addPool))
	cmd.AddCommand(newPoolRemoveCmd(cfg, out))
	cmd.AddCommand(newPoolUpdateCmd(cfg, out))
	return cmd
}
