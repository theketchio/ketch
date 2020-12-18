package main

import (
	"errors"
	"io/ioutil"
	"testing"

	"github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestPlatformAdd(t *testing.T) {
	sf := func(s string) string { return "-" + s }
	tt := []struct {
		name    string
		args    []string
		testFn  func(obj runtime.Object) error
		called  bool
		wantErr bool
	}{
		{
			name: "happy path",
			args: []string{
				"java",
				sf(imageShortFlag),
				"somerepo/someimage:latest",
			},
			called: true,
			testFn: func(obj runtime.Object) error {
				p, ok := obj.(*v1beta1.Platform)
				require.True(t, ok)
				require.Equal(t, "java", p.ObjectMeta.Name)
				require.Equal(t, "somerepo/someimage:latest", p.Spec.Image)
				return nil
			},
		},
		{
			name: "official image",
			args: []string{
				"java",
			},
			called: true,
			testFn: func(obj runtime.Object) error {
				p, ok := obj.(*v1beta1.Platform)
				require.True(t, ok)
				require.Equal(t, "java", p.ObjectMeta.Name)
				require.Equal(t, "shipasoftware/java:v1.2", p.Spec.Image)
				return nil
			},
		},
		{
			name: "ambiguous commands",
			args: []string{
				"java",
				sf(imageShortFlag),
				"somerepo/someimage:latest",
				sf(dockerfileShortFlag),
				"/home/bob/Dockerfile",
			},
			wantErr: true,
		},
		{
			name: "unknown official platform",
			args: []string{
				"pascal",
			},
			wantErr: true,
		},
		{
			name: "client failed",
			args: []string{
				"java",
				sf(imageShortFlag),
				"somerepo/someimage:latest",
			},

			testFn: func(obj runtime.Object) error {
				return errors.New("xxx")
			},
			wantErr: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cli := &resourceCreatorMock{tc.testFn, false}
			cmd := newPlatformAddCmd(cli, ioutil.Discard)
			cmd.SetArgs(tc.args)
			cmd.SetOut(ioutil.Discard)
			err := cmd.Execute()
			if tc.wantErr {
				require.NotNil(t, err)
				return
			}
			require.Nil(t, err)
			require.Equal(t, tc.called, cli.called)
		})
	}
}
