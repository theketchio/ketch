package main

import (
	"io"

	"github.com/spf13/cobra"
)

const jobHelp = `
Manage jobs.
`

func newJobCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "job",
		Short: "Manage Jobs",
		Long:  jobHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(newJobListCmd(cfg, out))
	cmd.AddCommand(newJobDeployCmd(cfg, out))
	cmd.AddCommand(newJobRemoveCmd(cfg, out))
	cmd.AddCommand(newJobExportCmd(cfg, out))
	return cmd
}
