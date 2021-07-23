package main

import (
	"context"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	"github.com/shipa-corp/ketch/cmd/ketch/output"
	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

type frameworkExportOptions struct {
	frameworkName string
	filename      string
}

const frameworkExportHelp = `Export a framework's configuration file.`

func newFrameworkExportCmd(cfg config, out io.Writer) *cobra.Command {
	var options frameworkExportOptions

	cmd := &cobra.Command{
		Use:   "export FRAMEWORK",
		Args:  cobra.ExactValidArgs(1),
		Short: "Export a framework to file.",
		Long:  frameworkExportHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.frameworkName = args[0]
			return exportFramework(cmd.Context(), cfg, options, out)
		},
	}
	cmd.Flags().StringVarP(&options.filename, "file", "f", "", "filename for framework export")
	return cmd
}

func exportFramework(ctx context.Context, cfg config, options frameworkExportOptions, out io.Writer) error {
	var framework ketchv1.Framework
	err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.frameworkName}, &framework)
	if err != nil {
		return err
	}
	framework.Spec.Name = framework.Name

	if options.filename != "" {
		f, err := output.GetOutputFile(options.filename)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}
	b, err := yaml.Marshal(framework.Spec)
	if err != nil {
		return err
	}
	_, err = out.Write(b)
	return err
}
