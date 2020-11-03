package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

const appRemoveHelp = `
Remove an application.
`

func newAppRemoveCmd(cfg config, out io.Writer) *cobra.Command {
	options := appRemoveOptions{}
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove an application.",
		Args:  cobra.NoArgs,
		Long:  appRemoveHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			return appRemove(cmd.Context(), cfg, options, out)
		},
	}
	cmd.Flags().StringVarP(&options.appName, "app", "a", "", "The name of the app.")
	cmd.MarkFlagRequired("app")
	return cmd
}

type appRemoveOptions struct {
	appName string
}

func appRemove(ctx context.Context, cfg config, options appRemoveOptions, out io.Writer) error {
	app := ketchv1.App{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.appName}, &app); err != nil {
		return fmt.Errorf("failed to get app: %w", err)
	}
	if err := cfg.Client().Delete(ctx, &app); err != nil {
		// FIXME: remove templates?
		return fmt.Errorf("failed to delete app: %w", err)
	}
	fmt.Fprintln(out, "Successfully removed!")
	return nil
}
