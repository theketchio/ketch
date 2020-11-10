package main

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func TestAppRemoveCmd(t *testing.T) {
	pflag.CommandLine = pflag.NewFlagSet("ketch", pflag.ExitOnError)

	tt := []struct {
		description string
		args        []string
		appRemover  appRemoveFn
		wantErr     bool
	}{
		{
			description: "happy path",
			args:        []string{"ketch", "foo-bar"},
			appRemover: func(_ context.Context, _ config, appName string, _ io.Writer) error {
				require.Equal(t, "foo-bar", appName)
				return nil
			},
		},
		{
			description: "bad app name",
			args:        []string{"ketch", "foo@bar"},
			wantErr:     true,
		},
		{
			description: "missing positional arg",
			args:        []string{"ketch"},
			wantErr:     true,
		},
		{
			description: "too many positional args",
			args:        []string{"ketch", "foo-bar", "extra"},
			wantErr:     true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.description, func(t *testing.T) {
			os.Args = tc.args
			cmd := newAppRemoveCmd(nil, nil, tc.appRemover)
			err := cmd.Execute()
			if tc.wantErr {
				require.NotNil(t, err)
				return
			}
			require.Nil(t, err)

		})
	}
}
