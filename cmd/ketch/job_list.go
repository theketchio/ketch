package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/theketchio/ketch/cmd/ketch/output"
	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"
)

const jobListHelp = `
List all jobs.
`

type jobListOutput struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Namespace   string `json:"namespace"`
	Description string `json:"description"`
}

func newJobListCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all jobs.",
		Long:  jobListHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			return jobList(cmd.Context(), cfg, out)
		},
	}
	return cmd
}

func jobList(ctx context.Context, cfg config, out io.Writer) error {
	jobs := ketchv1.JobList{}
	if err := cfg.Client().List(ctx, &jobs); err != nil {
		return fmt.Errorf("failed to get list of jobs: %w", err)
	}
	return output.Write(generateJobListOutput(jobs), out, "column")
}

func generateJobListOutput(jobs ketchv1.JobList) []jobListOutput {
	var output []jobListOutput
	for _, item := range jobs.Items {
		output = append(output, jobListOutput{
			Name:        item.Name,
			Version:     item.Spec.Version,
			Namespace:   item.Spec.Namespace,
			Description: item.Spec.Description,
		})
	}
	return output
}

func jobListNames(cfg config, nameFilter ...string) ([]string, error) {
	jobs := ketchv1.JobList{}
	if err := cfg.Client().List(context.TODO(), &jobs); err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	jobNames := make([]string, 0)
	for _, j := range jobs.Items {
		if len(nameFilter) == 0 {
			jobNames = append(jobNames, j.Name)
		}

		for _, filter := range nameFilter {
			if strings.Contains(j.Name, filter) {
				jobNames = append(jobNames, j.Name)
			}
		}
	}
	return jobNames, nil
}
