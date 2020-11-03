package main

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

const envSetHelp = `
Set environment variables for an application.
`

func newEnvSetCmd(cfg config, out io.Writer) *cobra.Command {
	options := envSetOptions{}
	cmd := &cobra.Command{
		Use:   "set",
		Args:  cobra.MinimumNArgs(1),
		Short: "Set environment variables for an application.",
		Long:  envSetHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.envs = args
			return envSet(cmd.Context(), cfg, options, out)

		},
	}
	cmd.Flags().StringVarP(&options.appName, "app", "a", "", "The name of the app.")
	cmd.MarkFlagRequired("app")
	return cmd
}

type envSetOptions struct {
	appName string
	envs    []string
}

func envSet(ctx context.Context, cfg config, options envSetOptions, out io.Writer) error {
	envs, err := getEnvs(options.envs)
	if err != nil {
		return fmt.Errorf("failed to get kubernetes client: %w", err)
	}
	app := ketchv1.App{}
	if err = cfg.Client().Get(ctx, types.NamespacedName{Name: options.appName}, &app); err != nil {
		log.Fatalf("failed to get the app: %v", err)
	}
	app.SetEnvs(envs)
	if err := cfg.Client().Update(ctx, &app); err != nil {
		return fmt.Errorf("failed to update the app: %w", err)
	}
	return nil
}
