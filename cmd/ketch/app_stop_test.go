package main

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func TestAppStop(t *testing.T) {
	pflag.CommandLine = pflag.NewFlagSet("ketch", pflag.ExitOnError)

	tt := []struct {
		description string
		args        []string
		appStop     appStopFn
		wantErr     bool
	}{
		{
			description: "happy path",
			args:        []string{"ketch", "myapp", "-p", "myprocess", "-v", "3"},
			appStop: func(_ context.Context, _ config, opts appStopOptions, _ io.Writer) error {
				require.Equal(t, "myapp", opts.appName)
				require.Equal(t, "myprocess", opts.processName)
				require.Equal(t, 3, opts.deploymentVersion)
				return nil
			},
		},
		{
			description: "missing positional arg",
			args:        []string{"ketch", "-p", "myprocess", "-v", "3"},
			wantErr:     true,
		},
		{
			description: "too many positionals",
			args:        []string{"ketch", "myapp", "extra", "-p", "myprocess", "-v", "3"},
			wantErr:     true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.description, func(t *testing.T) {
			os.Args = tc.args
			cmd := newAppStopCmd(nil, nil, tc.appStop)
			err := cmd.Execute()
			if tc.wantErr {
				require.NotNil(t, err)
				return
			}
			require.Nil(t, err)
		})
	}
}
