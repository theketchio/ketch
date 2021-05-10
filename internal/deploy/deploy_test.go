package deploy

import (
	"context"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"path"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_validatePaths(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) (string, []string, error)
		wantErr bool
	}{
		{
			name: "happy path",
			setup: func(t *testing.T) (string, []string, error) {
				root := t.TempDir()
				subpaths := []string{"one", "two"}
				for _, p := range subpaths {
					_ = os.MkdirAll(path.Join(root, p), 0700)
				}
				return root, subpaths, nil
			},
		},
		{
			name: "invalid",
			setup: func(t *testing.T) (string, []string, error) {
				root := t.TempDir() + "xxx"
				return root, nil, nil
			},
			wantErr: true,
		},
		{
			name: "empty root",
			setup: func(t *testing.T) (string, []string, error) {
				return "", nil, nil
			},
		},
		{
			name: "missing subpath",
			setup: func(t *testing.T) (string, []string, error) {
				root := t.TempDir()
				return root, []string{"missing"}, nil
			},
			wantErr: true,
		},
		{
			name: "file not directory",
			setup: func(t *testing.T) (string, []string, error) {
				root := t.TempDir()
				temp, _ := ioutil.TempFile(root, "test")
				fileName := path.Base(temp.Name())
				return root, []string{fileName}, nil
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, subpaths, err := tt.setup(t)
			require.Nil(t, err)
			if err := validatePaths(root, subpaths); (err != nil) != tt.wantErr {
				t.Errorf("validatePaths() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_validateCanary(t *testing.T) {
	type args struct {
		steps            int
		stepTimeInterval string
		timeout          string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "happy path",
			args: args{
				steps:            4,
				stepTimeInterval: "1h",
				timeout:          "2h",
			},
		},
		{
			name: "steps not an even divisor of 100",
			args: args{
				steps:            7,
				stepTimeInterval: "1h",
				timeout:          "2h",
			},
			wantErr: true,
		},
		{
			name: "steps below expected value",
			args: args{
				steps:            1,
				stepTimeInterval: "1h",
				timeout:          "2h",
			},
			wantErr: true,
		},
		{
			name: "steps equals 0",
			args: args{
				steps:            0,
				stepTimeInterval: "1h",
				timeout:          "2h",
			},
		},
		{
			name: "steps above expected value",
			args: args{
				steps:            102,
				stepTimeInterval: "1h",
				timeout:          "2h",
			},
			wantErr: true,
		},
		{
			name: "stepTimeInterval not set",
			args: args{
				steps:   4,
				timeout: "2h",
			},
			wantErr: true,
		},
		{
			name: "illicit stepTimeInterval format",
			args: args{
				steps:            4,
				stepTimeInterval: "bad format",
				timeout:          "2h",
			},
			wantErr: true,
		},
		{
			name: "illicit timeout format",
			args: args{
				steps:            4,
				stepTimeInterval: "1h",
				timeout:          "bad format",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateCanary(tt.args.steps, tt.args.stepTimeInterval, tt.args.timeout); (err != nil) != tt.wantErr {
				t.Errorf("validateCanary() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TODO: create an appropriate mock
type fakeClusterClient struct{}

func (cc fakeClusterClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	return nil
}

func (cc fakeClusterClient) Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
	return nil
}

func TestNew(t *testing.T) {
	type args struct {
		client clusterClient
		opts   Options
	}
	tests := []struct {
		name    string
		args    args
		want    *Runner
		wantErr bool
	}{
		{
			name: "happy path",
			args: args{
				client: fakeClusterClient{},
				opts: Options{
					AppName:          "app",
					Image:            "image",
					Steps:            4,
					StepTimeInterval: "1h",
					Timeout:          "2h",
				},
			},
			want: &Runner{
				stepWeight: 25,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.args.client, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() got = %v, want %v", got, tt.want)
			}
		})
	}
}
