package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/types"

	"github.com/shipa-corp/ketch/cmd/ketch/output"
	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
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
			return envGet(cmd.Context(), cfg, options, out, cmd.Flags())

		},
	}
	cmd.Flags().StringP("output", "o", "", "used to specify output, e.g. --output format=json")
	cmd.Flags().StringVarP(&options.appName, "app", "a", "", "The name of the app.")
	cmd.MarkFlagRequired("app")
	return cmd
}

type envGetOptions struct {
	appName string   `json:"appName" yaml:"appName"`
	envs    []string `json:"envs" yaml:"envs"`
}

func envGet(ctx context.Context, cfg config, options envGetOptions, out io.Writer, flags *pflag.FlagSet) error {
	app := ketchv1.App{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.appName}, &app); err != nil {
		return fmt.Errorf("failed to get the app: %w", err)
	}
	outputFlag, _ := flags.GetString("output")
	return output.Write(app.Envs(options.envs), out, outputFlag)
}
