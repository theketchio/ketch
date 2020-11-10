package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/chart"
)

const appExportHelp = `
Export an application as a helm chart. 
`

type appExportFn func(ctx context.Context, cfg config, chartNew chartNewFn, options appExportOptions, out io.Writer) error

func newAppExportCmd(cfg config, out io.Writer, appExport appExportFn) *cobra.Command {
	options := appExportOptions{}
	cmd := &cobra.Command{
		Use:   "export APPNAME",
		Short: "Export an app's chart templates",
		Long:  appExportHelp,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.appName = args[0]
			return appExport(cmd.Context(), cfg, chart.New, options, out)
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

type chartNewFn func(application *ketchv1.App, pool *ketchv1.Pool, opts ...chart.Option) (*chart.ApplicationChart, error)

func appExport(ctx context.Context, cfg config, chartNew chartNewFn, options appExportOptions, out io.Writer) error {
	app := ketchv1.App{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.appName}, &app); err != nil {
		return fmt.Errorf("failed to get app: %w", err)
	}
	pool := ketchv1.Pool{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: app.Spec.Pool}, &pool); err != nil {
		return fmt.Errorf("failed to get pool: %w", err)
	}
	tpls, err := cfg.Storage().Get(app.TemplatesConfigMapName(pool.Spec.IngressController.IngressType))
	if err != nil {
		return fmt.Errorf("failed to get the app's templates: %w", err)
	}
	chartOptions := []chart.Option{
		chart.WithExposedPorts(app.ExposedPorts()),
		chart.WithTemplates(*tpls),
	}
	appChrt, err := chartNew(&app, &pool, chartOptions...)
	if err != nil {
		return fmt.Errorf("failed to create helm chart: %w", err)
	}
	if err := appChrt.ExportToDirectory(options.directory, chart.NewChartConfig(app)); err != nil {
		return fmt.Errorf("failed to export the app's chart: %w", err)
	}
	fmt.Fprintln(out, "Successfully exported!")
	return nil
}
