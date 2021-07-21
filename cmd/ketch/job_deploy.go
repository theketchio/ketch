package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

const jobDeployHelp = `
Deploy a job.
`

const (
	defaultJobVersion       = "v1"
	defaultJobParallelism   = 1
	defaultJobCompletions   = 1
	defaultJobBackoffLimit  = 6
	defaultJobRestartPolicy = "Never"
)

func newJobDeployCmd(cfg config, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [FILENAME]",
		Short: "Deploy a job.",
		Long:  jobDeployHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			filename := args[0]
			return jobDeploy(cmd.Context(), cfg, filename, out)
		},
	}
	return cmd
}

func jobDeploy(ctx context.Context, cfg config, filename string, out io.Writer) error {
	b, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	var spec ketchv1.JobSpec
	err = yaml.Unmarshal(b, &spec)
	if err != nil {
		return err
	}
	setJobSpecDefaults(&spec)
	if err = validateJobSpec(&spec); err != nil {
		return err
	}

	var job ketchv1.Job
	err = cfg.Client().Get(ctx, types.NamespacedName{Name: spec.Name}, &job)
	if err != nil {
		if apierrors.IsNotFound(err) {
			job.ObjectMeta.Name = spec.Name
			job.Spec = spec
			if err := cfg.Client().Create(ctx, &job); err != nil {
				return fmt.Errorf("failed to create job: %w", err)
			}
			fmt.Fprintln(out, "Successfully added!")
			return nil
		}
		return err
	}
	job.Spec = spec
	if err := cfg.Client().Update(ctx, &job); err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}
	fmt.Fprintln(out, "Successfully updated!")
	return nil
}

// setJobSpecDefaults sets defaults on job.Spec for some unset fields
func setJobSpecDefaults(jobSpec *ketchv1.JobSpec) {
	jobSpec.Type = "Job"
	if jobSpec.Version == "" {
		jobSpec.Version = defaultJobVersion
	}
	if jobSpec.Parallelism == 0 {
		jobSpec.Parallelism = defaultJobParallelism
	}
	if jobSpec.Completions == 0 && jobSpec.Parallelism > 1 {
		jobSpec.Completions = defaultJobCompletions
	}
	if jobSpec.BackoffLimit == 0 {
		jobSpec.BackoffLimit = defaultJobBackoffLimit
	}
	if jobSpec.Policy.RestartPolicy == "" {
		jobSpec.Policy.RestartPolicy = defaultJobRestartPolicy
	}
}

// validateJobSpec assures that required fields are populated. Missing fields will throw errors
// when the custom resource is created, but this is a way to surface errors to user clearly.
func validateJobSpec(jobSpec *ketchv1.JobSpec) error {
	if jobSpec.Name == "" || jobSpec.Framework == "" {
		return errors.New("job.name and job.framework are required")
	}
	return nil
}
