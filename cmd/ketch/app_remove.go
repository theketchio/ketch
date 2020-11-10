package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/validation"
)

const appRemoveHelp = `
Remove an application.
`

type appRemoveFn func(context.Context, config, string, io.Writer) error

func newAppRemoveCmd(cfg config, out io.Writer, appRemove appRemoveFn) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove APPNAME",
		Short: "Remove an application.",
		Args:  cobra.ExactValidArgs(1),
		Long:  appRemoveHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			appName := args[0]
			if !validation.ValidateName(appName) {
				return ErrInvalidAppName
			}
			return appRemove(cmd.Context(), cfg, appName, out)
		},
	}
	return cmd
}

func appRemove(ctx context.Context, cfg config, appName string, out io.Writer) error {
	app := ketchv1.App{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: appName}, &app); err != nil {
		return fmt.Errorf("failed to get app: %w", err)
	}
	if err := cfg.Client().Delete(ctx, &app); err != nil {
		return fmt.Errorf("failed to delete app: %w", err)
	}
	fmt.Fprintln(out, "Successfully removed!")
	return nil
}
