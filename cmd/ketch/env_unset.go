package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

const envUnsetHelp = `
Unset environment variables for an application.

ketch env-unset <ENVIRONMENT_VARIABLE1> [ENVIRONMENT_VARIABLE2] ... [ENVIRONMENT_VARIABLEN]
`

func newEnvUnsetCmd(cfg config, out io.Writer) *cobra.Command {
	options := envUnsetOptions{}
	cmd := &cobra.Command{
		Use:   "unset",
		Args:  cobra.MinimumNArgs(1),
		Short: "Unset environment variables for an application.",
		Long:  envUnsetHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.envs = args
			return envUnset(cmd.Context(), cfg, options, out)

		},
	}
	cmd.Flags().StringVarP(&options.appName, "app", "a", "", "The name of the app.")
	cmd.MarkFlagRequired("app")
	return cmd
}

type envUnsetOptions struct {
	appName string
	envs    []string
}

func envUnset(ctx context.Context, cfg config, options envUnsetOptions, out io.Writer) error {
	app := ketchv1.App{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.appName}, &app); err != nil {
		return fmt.Errorf("failed to get the app: %w", err)
	}
	app.UnsetEnvs(options.envs)
	if err := cfg.Client().Update(ctx, &app); err != nil {
		return fmt.Errorf("failed to update the app: %w", err)
	}
	return nil
}
