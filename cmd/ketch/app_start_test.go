package main

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func TestAppStart(t *testing.T) {
	pflag.CommandLine = pflag.NewFlagSet("ketch", pflag.ExitOnError)

	tt := []struct {
		description string
		args        []string
		appStart    appStartFn
		wantErr     bool
	}{
		{
			description: "happy path",
			args:        []string{"ketch", "myapp", "-p", "myprocess", "-v", "2"},
			appStart: func(_ context.Context, _ config, opts appStartOptions, _ io.Writer) error {
				require.Equal(t, "myapp", opts.appName)
				require.Equal(t, "myprocess", opts.processName)
				require.Equal(t, 2, opts.deploymentVersion)
				return nil
			},
		},
		{
			description: "missing positional",
			args:        []string{"ketch", "-p", "myprocess", "-v", "2"},
			wantErr:     true,
		},
		{
			description: "extra positional",
			args:        []string{"ketch", "myapp", "extra", "-p", "myprocess", "-v", "2"},
			wantErr:     true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.description, func(t *testing.T) {
			os.Args = tc.args
			cmd := newAppStartCmd(nil, nil, tc.appStart)
			err := cmd.Execute()
			if tc.wantErr {
				require.NotNil(t, err)
				return
			}
			require.Nil(t, err)

		})
	}
}
