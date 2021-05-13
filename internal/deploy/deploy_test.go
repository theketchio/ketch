package deploy

import (
	"os"
	"path"
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
