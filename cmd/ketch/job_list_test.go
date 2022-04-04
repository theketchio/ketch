package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"
	"github.com/theketchio/ketch/internal/mocks"
	"github.com/theketchio/ketch/internal/utils/conversions"
)

func TestJobList(t *testing.T) {
	mockJob := &ketchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "hello", Namespace: "ketch-myframework"},
		Spec: ketchv1.JobSpec{
			Name:         "hello",
			Version:      "v1",
			Framework:    "myframework",
			Description:  "test",
			Parallelism:  1,
			Completions:  1,
			Suspend:      false,
			BackoffLimit: conversions.IntPtr(6),
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
		name    string
		cfg     config
		wantOut string
		wantErr string
	}{
		{
			name: "success",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{mockJob},
				DynamicClientObjects: []runtime.Object{},
			},
			wantOut: "NAME     VERSION    FRAMEWORK      DESCRIPTION\nhello    v1         myframework    test\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			err := jobList(context.Background(), tt.cfg, out)
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			} else {
				require.Nil(t, err)
			}
			require.Equal(t, tt.wantOut, out.String())
		})
	}
}

func TestJobListNames(t *testing.T) {
	mockJob := &ketchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "hello", Namespace: "ketch-myframework"},
		Spec: ketchv1.JobSpec{
			Name:         "hello",
			Version:      "v1",
			Framework:    "myframework",
			Description:  "test",
			Parallelism:  1,
			Completions:  1,
			Suspend:      false,
			BackoffLimit: conversions.IntPtr(6),
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
		name    string
		cfg     config
		filter  string
		wantOut []string
		wantErr string
	}{
		{
			name: "success",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{mockJob},
				DynamicClientObjects: []runtime.Object{},
			},
			wantOut: []string{"hello"},
		},
		{
			name: "filter",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{mockJob},
				DynamicClientObjects: []runtime.Object{},
			},
			filter:  "goodbye",
			wantOut: []string{},
		},
		{
			name: "filter",
			cfg: &mocks.Configuration{
				CtrlClientObjects:    []runtime.Object{mockJob},
				DynamicClientObjects: []runtime.Object{},
			},
			filter:  "hello",
			wantOut: []string{"hello"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := jobListNames(tt.cfg, tt.filter)
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.wantOut, out)
		})
	}
}
