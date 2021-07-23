package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	"github.com/shipa-corp/ketch/cmd/ketch/output"
	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/deploy"
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
	if options.filename != "" {
		f, err := output.GetOutputFile(options.filename)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}
	b, err := yaml.Marshal(application)
	if err != nil {
		return err
	}
	_, err = out.Write(b)
	return err
}
