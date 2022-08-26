package main

import (
	"context"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	"github.com/theketchio/ketch/cmd/ketch/output"
	ketchv1 "github.com/theketchio/ketch/internal/api/v1"
	"github.com/theketchio/ketch/internal/validation"
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
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.name, Namespace: "default"}, &job); err != nil {
		return err
	}
	return output.WriteToFileOrOut(job.Spec, out, options.filename)
}
