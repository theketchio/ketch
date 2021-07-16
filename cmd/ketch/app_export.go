package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/deploy"
)

const appExportHelp = `
Export an application as a yaml file.
`

type appExportFn func(ctx context.Context, cfg config, options appExportOptions) error

func newAppExportCmd(cfg config, appExport appExportFn) *cobra.Command {
	options := appExportOptions{}
	cmd := &cobra.Command{
		Use:   "export APPNAME",
		Short: "Export an app's yaml",
		Long:  appExportHelp,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.appName = args[0]
			return appExport(cmd.Context(), cfg, options)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return autoCompleteAppNames(cfg, toComplete)
		},
	}
	cmd.Flags().StringVarP(&options.filename, "file", "f", "app.yaml", "filename for app export")
	return cmd
}

type appExportOptions struct {
	appName  string
	filename string
}

func exportApp(ctx context.Context, cfg config, options appExportOptions) error {
	app := ketchv1.App{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.appName}, &app); err != nil {
		return fmt.Errorf("failed to get app: %w", err)
	}
	application := deploy.GetApplicationFromKetchApp(app)
	// open file, err if exist, write application
	_, err := os.Stat(options.filename)
	if !os.IsNotExist(err) {
		return errFileExists
	}
	f, err := os.Create(options.filename)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := yaml.Marshal(application)
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	return err
}
