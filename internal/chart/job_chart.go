package chart

import (
	"fmt"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"
)

// JobChart is an internal representation of a helm chart converted from the Job CRD
// and is used to render a helm chart.
type JobChart struct {
	values    jobValues
	templates map[string]string
}

type jobValues struct {
	Job Job `json:"job"`
}

type Job struct {
	ketchv1.JobSpec
	Group string `json:"group"`
}

// NewJobChart returns a JobChart instance from a ketchv1.Job and []Option
func NewJobChart(job *ketchv1.Job, opts ...Option) *JobChart {
	jobChart := &JobChart{
		values: jobValues{
			Job: Job{
				JobSpec: job.Spec,
				Group:   ketchv1.Group,
			},
		},
	}
	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}
	jobChart.templates = options.Templates.Yamls
	return jobChart
}

// NewJobChartConfig returns a ChartConfig instance based on the given job.
func NewJobChartConfig(job ketchv1.Job) ChartConfig {
	version := fmt.Sprintf("v%v", job.ObjectMeta.Generation)
	chartVersion := fmt.Sprintf("v0.0.%v", job.ObjectMeta.Generation)
	if job.Spec.Version != "" {
		version = job.Spec.Version
	}
	return ChartConfig{
		Version:            chartVersion,
		Description:        job.Spec.Description,
		AppName:            job.Spec.Name,
		AppVersion:         version,
		DeploymentVersions: []int{int(job.ObjectMeta.Generation)},
	}
}

// GetName returns the job name, satisfying TemplateValuer
func (j *JobChart) GetName() string {
	return j.values.Job.Name
}

// GetTemplates returns the job templates, satisfying TemplateValuer
func (j *JobChart) GetTemplates() map[string]string {
	return j.templates
}

// GetValues returns the job values, satisfying TemplateValuer
func (j *JobChart) GetValues() interface{} {
	return j.values
}
