package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/mocks"
)

func TestJobExport(t *testing.T) {
	mockJob := &ketchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "hello"},
		Spec: ketchv1.JobSpec{
			Name:         "hello",
			Version:      "v1",
			Framework:    "myframework",
			Description:  "test",
			Parallelism:  1,
			Completions:  1,
			Suspend:      false,
			BackoffLimit: 6,
			Containers: []ketchv1.Container{
				{
					Name:    "lister",
					Image:   "ubuntu",
					Command: []string{"ls", "/"},
				},
			},
			Policy: ketchv1.Policy{
				RestartPolicy: "Never",
			},
			Type: "Job",
		},
	}
	tests := []struct {
		name         string
		cfg          config
		options      jobExportOptions
		wantYamlData string
		wantErr      string
	}{
		{
			name: "success",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{mockJob},
				DynamicClientObjects: []runtime.Object{},
			},
			options: jobExportOptions{
				name: "hello",
			},
			wantYamlData: `backoffLimit: 6
completions: 1
containers:
- command:
  - ls
  - /
  image: ubuntu
  name: lister
description: test
framework: myframework
name: hello
parallelism: 1
policy:
  restartPolicy: Never
type: Job
version: v1
`,
		},
		{
			name: "error - no job found",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{mockJob},
				DynamicClientObjects: []runtime.Object{},
			},
			options: jobExportOptions{
				name: "no-exist",
			},
			wantErr: `jobs.theketch.io "no-exist" not found`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			err := jobExport(context.Background(), tt.cfg, tt.options, out)
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.wantYamlData, out.String())
		})
	}
}
