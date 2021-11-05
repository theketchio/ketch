package main

import (
	"github.com/spf13/cobra"

	"github.com/theketchio/ketch/cmd/ketch/configuration"
)

const builderSetHelp = `
Manage the default builder to be used when deploying from source. This value can also be set manually in the config.toml
by specifying "default-builder".`

func newBuilderSetCmd(ketchConfig configuration.KetchConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "set default builder",
		Long:  builderSetHelp,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setDefaultBuilder(ketchConfig, args[0])
		},
	}
	return cmd
}

func setDefaultBuilder(ketchConfig configuration.KetchConfig, defaultBuilder string) error {
	ketchConfig.DefaultBuilder = defaultBuilder
	path, err := configuration.DefaultConfigPath()
	if err != nil {
		return err
	}
	return configuration.Write(ketchConfig, path)
}
