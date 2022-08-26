package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	"github.com/theketchio/ketch/cmd/ketch/output"
	ketchv1 "github.com/theketchio/ketch/internal/api/v1"
)

const envGetHelp = `
Retrieve environment variables for an application.

ketch env-get [-a/--app appname] [ENVIRONMENT_VARIABLE1] [ENVIRONMENT_VARIABLE2] ...
`

func newEnvGetCmd(cfg config, out io.Writer) *cobra.Command {
	options := envGetOptions{}
	cmd := &cobra.Command{
		Use:   "get ENV_VAR1 ENV_VAR2 ...",
		Short: "Retrieve environment variables for an application.",
		Long:  envGetHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.envs = args
			return envGet(cmd.Context(), cfg, options, out)

		},
	}
	cmd.Flags().StringVarP(&options.appName, "app", "a", "", "The name of the app.")
	cmd.MarkFlagRequired("app")
	return cmd
}

type envGetOptions struct {
	appName string
	envs    []string
}

func envGet(ctx context.Context, cfg config, options envGetOptions, out io.Writer) error {
	app := ketchv1.App{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.appName}, &app); err != nil {
		return fmt.Errorf("failed to get the app: %w", err)
	}
	return output.Write(app.Envs(options.envs), out, "column")
}
