package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

const appExportHelp = `
Export templates used to render an application's helm chart. 
`

func newAppExportCmd(cfg config, out io.Writer) *cobra.Command {
	options := appExportOptions{}
	cmd := &cobra.Command{
		Use:   "export APPNAME",
		Short: "Export an app's chart templates",
		Long:  appExportHelp,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.appName = args[0]
			return appExport(cmd.Context(), cfg, options, out)
		},
	}
	cmd.Flags().StringVarP(&options.directory, "directory", "d", "", "The directory with the templates")
	cmd.MarkFlagRequired("directory")
	return cmd
}

type appExportOptions struct {
	appName   string
	directory string
}

func appExport(ctx context.Context, cfg config, options appExportOptions, out io.Writer) error {
	app := ketchv1.App{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.appName}, &app); err != nil {
		return fmt.Errorf("failed to get app: %w", err)
	}
	tpls, err := cfg.Storage().Get(app.TemplatesConfigMapName())
	if err != nil {
		return fmt.Errorf("failed to get the app's templates: %w", err)
	}
	if err := tpls.ExportToDirectory(options.directory); err != nil {
		return fmt.Errorf("failed to get export the app's templates: %w", err)
	}
	fmt.Fprintln(out, "Successfully exported!")
	return nil
}
