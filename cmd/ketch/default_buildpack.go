package main

import (
	"io"

	"github.com/spf13/cobra"
)

const (
	defaultBuildPackHelp = `
Set default builder used by other commands.

** For suggested builders simply leave builder name empty. **

Usage:
  ketch set-default-builder [builder-name] [flags]

Examples:
ketch set-default-builder cnbs/sample-builder:bionic

Flags:
  -h, --help   Help for 'set-default-builder'
`
)

func newDefaultBuilderCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use: "set-default-builder <builder-name>",
		Short: "Sets the default builder",
		Long: defaultBuildPackHelp,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return suggestBuilders(cmd)
			}

			return nil
		},

	}
	return cmd
}

func suggestBuilders(cmd *cobra.Command) error {
	return nil
}