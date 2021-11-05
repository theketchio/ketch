package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"

	"github.com/theketchio/ketch/cmd/ketch/configuration"
)

func Test_newBuilderSetCmd(t *testing.T) {
	type args struct {
		ketchConfig   configuration.KetchConfig
		defaultBulder string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "successfully write to file",
			args: args{
				ketchConfig: configuration.KetchConfig{
					DefaultBuilder: "oldDefault",
				},
				defaultBulder: "newDefault",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootPath := t.TempDir()
			fullPath := filepath.Join(rootPath, "config.toml")
			os.Setenv("KETCH_HOME", rootPath)

			cmd := newBuilderSetCmd(tt.args.ketchConfig)
			cmd.SetArgs([]string{tt.args.defaultBulder})
			err := cmd.Execute()
			require.Nil(t, err)

			var ketchConfig configuration.KetchConfig
			_, err = toml.DecodeFile(fullPath, &ketchConfig)
			require.Nil(t, err)

			require.Equal(t, tt.args.defaultBulder, ketchConfig.DefaultBuilder)
		})
	}
}
