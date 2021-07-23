package main

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/mocks"
)

func TestJobDeploy(t *testing.T) {
	tests := []struct {
		name        string
		jobName     string
		cfg         config
		filename    string
		yamlData    string
		wantJobSpec ketchv1.JobSpec
		wantOut     string
		wantErr     string
	}{
		{
			name:    "job from yaml file",
			jobName: "hello",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{},
				DynamicClientObjects: []runtime.Object{},
			},
			filename: "job.yaml",
			yamlData: `name: hello
version: v1
framework: myframework
description: test
parallelism: 1
completions: 1
suspend: false
backoffLimit: 6
containers: 
  - name: lister
    image: ubuntu
    command:
      - ls
      - /
policy:
  restartPolicy: Never
`,
			wantJobSpec: ketchv1.JobSpec{
				Name:         "hello",
				Version:      "v1",
				Type:         "Job",
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
			},
			wantOut: "Successfully added!\n",
		},
		{
			name:    "error - validation fail",
			jobName: "hello",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{},
				DynamicClientObjects: []runtime.Object{},
			},
			filename: "job.yaml",
			yamlData: `version: v1
framework: NOFRAMEWORK
description: test`,
			wantErr: "job.name and job.framework are required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.yamlData != "" {
				file, err := os.CreateTemp(t.TempDir(), "*.yaml")
				require.Nil(t, err)
				_, err = file.Write([]byte(tt.yamlData))
				require.Nil(t, err)
				defer os.Remove(file.Name())
				tt.filename = file.Name()
			}
			out := &bytes.Buffer{}
			err := jobDeploy(context.Background(), tt.cfg, tt.filename, out)
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.wantOut, out.String())

			gotJob := ketchv1.Job{}
			err = tt.cfg.Client().Get(context.Background(), types.NamespacedName{Name: tt.jobName}, &gotJob)
			require.Nil(t, err)
			require.Equal(t, tt.wantJobSpec, gotJob.Spec)
		})
	}
}
