package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"
)

const jobDeployHelp = `
Deploy a job.
`

const (
	defaultJobVersion       = "v1"
	defaultJobParallelism   = 1
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

	job := &ketchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: spec.Name}}
	res, err := controllerutil.CreateOrUpdate(ctx, cfg.Client(), job, func() error {
		job.Spec = spec
		return nil
	})
	if err != nil {
		if strings.Contains(err.Error(), ketchv1.ErrJobExists.Error()) {
			return ketchv1.ErrJobExists
		}
		return err
	}
	if res == controllerutil.OperationResultNone {
		return fmt.Errorf("job \"%s\" already exists and is unchanged", job.Spec.Name)
	}

	fmt.Fprintln(out, "Successfully added!")
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
	if jobSpec.Completions == 0 {
		jobSpec.Completions = jobSpec.Parallelism
	}
	if jobSpec.Policy.RestartPolicy == "" {
		jobSpec.Policy.RestartPolicy = defaultJobRestartPolicy
	}
}

// validateJobSpec assures that required fields are populated. Missing fields will throw errors
// when the custom resource is created, but this is a way to surface errors to user clearly.
func validateJobSpec(jobSpec *ketchv1.JobSpec) error {
	if jobSpec.Name == "" || jobSpec.Namespace == "" {
		return errors.New("job.name and job.namespace are required")
	}
	return nil
}
