package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

const appStartHelp = `
Start an application, or one of the processes of the application.
`

func newAppStartCmd(cfg config, out io.Writer) *cobra.Command {
	options := appStartOptions{}
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start an application, or one of the processes of the application.",
		Args:  cobra.NoArgs,
		Long:  appStartHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			return appStart(cmd.Context(), cfg, options, out)
		},
	}
	cmd.Flags().StringVarP(&options.appName, "app", "a", "", "The name of the app.")
	cmd.Flags().StringVarP(&options.processName, "process", "p", "", "Process name.")
	cmd.Flags().IntVarP(&options.deploymentVersion, "version", "v", 0, "Deployment version.")
	cmd.MarkFlagRequired("app")
	return cmd
}

type appStartOptions struct {
	appName           string
	processName       string
	deploymentVersion int
}

func appStart(ctx context.Context, cfg config, options appStartOptions, out io.Writer) error {
	app := ketchv1.App{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.appName}, &app); err != nil {
		return fmt.Errorf("failed to get app: %w", err)
	}
	s := ketchv1.NewSelector(options.deploymentVersion, options.processName)
	if err := app.Start(s); err != nil {
		return fmt.Errorf("failed to stop app: %w", err)
	}
	if err := cfg.Client().Update(ctx, &app); err != nil {
		return fmt.Errorf("failed to update app: %w", err)
	}
	fmt.Fprintln(out, "Successfully started!")
	return nil
}
