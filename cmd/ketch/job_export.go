package main

import (
	"context"
	"io"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/validation"
)

const jobExportHelp = `
Export a job's configuration file.
`

type jobExportOptions struct {
	filename string
	name     string
}

func newJobExportCmd(cfg config, out io.Writer) *cobra.Command {
	var options jobExportOptions
	cmd := &cobra.Command{
		Use:   "export JOB",
		Short: "Export a job.",
		Long:  jobExportHelp,
		Args:  cobra.ExactValidArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.name = args[0]
			if !validation.ValidateName(options.name) {
				return ErrInvalidJobName
			}
			return jobExport(cmd.Context(), cfg, options, out)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return autoCompleteJobNames(cfg, toComplete)
		},
	}
	cmd.Flags().StringVarP(&options.filename, "filename", "f", "", "filename for job export")
	return cmd
}

func jobExport(ctx context.Context, cfg config, options jobExportOptions, out io.Writer) error {
	var job ketchv1.Job
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.name}, &job); err != nil {
		return err
	}
	if options.filename != "" {
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
		out = f
	}
	b, err := yaml.Marshal(job.Spec)
	if err != nil {
		return err
	}
	_, err = out.Write(b)
	return err
}
