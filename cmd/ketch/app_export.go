package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	"github.com/theketchio/ketch/cmd/ketch/output"
	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"
	"github.com/theketchio/ketch/internal/deploy"
)

const appExportHelp = `
Export an application as a yaml file.
`

type appExportFn func(ctx context.Context, cfg config, options appExportOptions, out io.Writer) error

func newAppExportCmd(cfg config, appExport appExportFn, out io.Writer) *cobra.Command {
	options := appExportOptions{}
	cmd := &cobra.Command{
		Use:   "export APPNAME",
		Short: "Export an app's yaml",
		Long:  appExportHelp,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.appName = args[0]
			return appExport(cmd.Context(), cfg, options, out)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return autoCompleteAppNames(cfg, toComplete)
		},
	}
	cmd.Flags().StringVarP(&options.filename, "file", "f", "", "filename for app export")
	return cmd
}

type appExportOptions struct {
	appName  string
	filename string
}

func exportApp(ctx context.Context, cfg config, options appExportOptions, out io.Writer) error {
	app := ketchv1.App{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.appName}, &app); err != nil {
		return fmt.Errorf("failed to get app: %w", err)
	}
	application := deploy.GetApplicationFromKetchApp(app)
	return output.WriteToFileOrOut(application, out, options.filename)
}
