package v1beta1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/theketchio/ketch/internal/api/v1beta1/mocks"
)

type mockManager struct {
	client *mocks.MockClient
}

func (m *mockManager) GetClient() client.Client {
	return m.client
}

func TestJob_ValidateDelete(t *testing.T) {
	tests := []struct {
		name string
		job  Job
	}{
		{
			name: "success",
			job: Job{
				Status: JobStatus{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.job.ValidateDelete()
			require.Nil(t, err)
		})
	}
}

func TestJob_ValidateCreate(t *testing.T) {

	const listError Error = "error"

	tests := []struct {
		name    string
		job     Job
		client  *mocks.MockClient
		wantErr error
	}{
		{
			name: "error getting a list of jobs",
			client: &mocks.MockClient{
				OnList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					return listError
				},
			},
			wantErr: listError,
		},
		{
			name: "job already exists",
			client: &mocks.MockClient{
				OnList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					jobs := list.(*JobList)
					jobs.Items = []Job{
						{Spec: JobSpec{Name: "test-job"}},
					}
					return nil
				},
			},
			job: Job{
				Spec: JobSpec{
					Name: "test-job",
				},
			},
			wantErr: ErrJobExists,
		},
		{
			name: "success",
			client: &mocks.MockClient{
				OnList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					jobs := list.(*JobList)
					jobs.Items = []Job{
						{Spec: JobSpec{Name: "test-job"}},
					}
					return nil
				},
			},
			job: Job{
				Spec: JobSpec{
					Name: "another-test-job",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobmgr = &mockManager{client: tt.client}
			if err := tt.job.ValidateCreate(); err != tt.wantErr {
				t.Errorf("ValidateCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJob_ValidateUpdate(t *testing.T) {

	const listError Error = "error"

	tests := []struct {
		name    string
		job     Job
		old     runtime.Object
		client  *mocks.MockClient
		wantErr error
	}{
		{
			name: "error getting a list of jobs",
			job: Job{
				Spec: JobSpec{Name: "test-job"},
			},
			client: &mocks.MockClient{
				OnList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					return listError
				},
			},
			old: &Job{
				Spec: JobSpec{Name: "test-job"},
			},
			wantErr: listError,
		},
		{
			name: "job already exists",
			job: Job{
				ObjectMeta: metav1.ObjectMeta{Name: "job-1"},
				Spec:       JobSpec{Name: "test-job"},
			},
			client: &mocks.MockClient{
				OnList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					jobs := list.(*JobList)
					jobs.Items = []Job{
						{ObjectMeta: metav1.ObjectMeta{Name: "job-1"}, Spec: JobSpec{Name: "test-job"}},
					}
					return nil
				},
			},
			old: &Job{
				Spec: JobSpec{Name: "test-job"},
			},
			wantErr: ErrJobExists,
		},
		{
			name: "everything is ok",
			job: Job{
				ObjectMeta: metav1.ObjectMeta{Name: "job-1"},
				Spec:       JobSpec{Name: "another-job"},
			},
			client: &mocks.MockClient{
				OnList: func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
					jobs := list.(*JobList)
					jobs.Items = []Job{
						{ObjectMeta: metav1.ObjectMeta{Name: "job-1"}, Spec: JobSpec{Name: "another-job"}},
					}
					return nil
				},
			},
			old: &Job{
				Spec: JobSpec{Name: "test-job"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobmgr = &mockManager{client: tt.client}
			if err := tt.job.ValidateUpdate(tt.old); err != tt.wantErr {
				t.Errorf("ValidateUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
