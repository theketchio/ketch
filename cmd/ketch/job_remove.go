package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1"
	"github.com/theketchio/ketch/internal/validation"
)

const jobRemoveHelp = `
Remove a job.
`

func newJobRemoveCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove [NAME]",
		Short: "Remove a job.",
		Long:  jobRemoveHelp,
		Args:  cobra.ExactValidArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobName := args[0]
			if !validation.ValidateName(jobName) {
				return ErrInvalidJobName
			}
			return jobRemove(cmd.Context(), cfg, jobName, out)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return autoCompleteJobNames(cfg, toComplete)
		},
	}
	return cmd
}

func jobRemove(ctx context.Context, cfg config, jobName string, out io.Writer) error {
	var job ketchv1.Job
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: jobName, Namespace: "default"}, &job); err != nil {
		return err
	}

	if err := cfg.Client().Delete(ctx, &job); err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}
	fmt.Fprintln(out, "Successfully removed!")
	return nil
}
