package main

import (
	"io"

	"github.com/spf13/cobra"
)

func newCnameCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cname",
		Short: "Manage cnames of an application",
		Long:  "Manage cnames of an application",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(newCnameAddCmd(cfg, out))
	cmd.AddCommand(newCnameRemoveCmd(cfg, out))
	return cmd
}
