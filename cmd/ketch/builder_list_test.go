package main

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/theketchio/ketch/cmd/ketch/configuration"
)

const (
	defaultBuilders = `VENDOR               IMAGE                            DESCRIPTION
Google               gcr.io/buildpacks/builder:v1     GCP Builder for all runtimes
Heroku               heroku/buildpacks:18             heroku-18 base image with buildpacks for Ruby, Java, Node.js, Python, Golang, & PHP
Heroku               heroku/buildpacks:20             heroku-20 base image with buildpacks for Ruby, Java, Node.js, Python, Golang, & PHP
Paketo Buildpacks    paketobuildpacks/builder:base    Small base image with buildpacks for Java, Node.js, Golang, & .NET Core
Paketo Buildpacks    paketobuildpacks/builder:full    Larger base image with buildpacks for Java, Node.js, Golang, .NET Core, & PHP
Paketo Buildpacks    paketobuildpacks/builder:tiny    Tiny base image (bionic build image, distroless run image) with buildpacks for Golang
`
	userBuilders = `VENDOR               IMAGE                            DESCRIPTION
Google               gcr.io/buildpacks/builder:v1     GCP Builder for all runtimes
Heroku               heroku/buildpacks:18             heroku-18 base image with buildpacks for Ruby, Java, Node.js, Python, Golang, & PHP
Heroku               heroku/buildpacks:20             heroku-20 base image with buildpacks for Ruby, Java, Node.js, Python, Golang, & PHP
Paketo Buildpacks    paketobuildpacks/builder:base    Small base image with buildpacks for Java, Node.js, Golang, & .NET Core
Paketo Buildpacks    paketobuildpacks/builder:full    Larger base image with buildpacks for Java, Node.js, Golang, .NET Core, & PHP
Paketo Buildpacks    paketobuildpacks/builder:tiny    Tiny base image (bionic build image, distroless run image) with buildpacks for Golang
test vendor          test image                       test description
`
)

func TestBuilderList(t *testing.T) {

	tests := []struct {
		name        string
		ketchConfig configuration.KetchConfig
		expected    string
	}{
		{
			name: "default values",
			ketchConfig: configuration.KetchConfig{
				AdditionalBuilders: nil,
			},
			expected: defaultBuilders,
		},
		{
			name: "include user's builders",
			ketchConfig: configuration.KetchConfig{
				AdditionalBuilders: []configuration.AdditionalBuilder{
					{
						Vendor:      "test vendor",
						Image:       "test image",
						Description: "test description",
					},
				},
			},
			expected: userBuilders,
		},
	}

	for _, tt := range tests {
		var buff bytes.Buffer
		cmd := newBuilderListCmd(tt.ketchConfig, &buff)
		cmd.SetArgs([]string{})
		err := cmd.Execute()
		require.Nil(t, err)
		require.Equal(t, tt.expected, buff.String())
	}
}

func TestBuilderList_Names(t *testing.T) {
	tests := []struct {
		name string

		builderList BuilderList
		filter      []string
		want        []string
	}{
		{
			name: "no filter, return all",
			builderList: BuilderList{
				{Image: "img1"},
				{Image: "img2"},
				{Image: "img3"},
			},
			filter: nil,
			want:   []string{"img1", "img2", "img3"},
		},
		{
			name: "filtered result",
			builderList: BuilderList{
				{Image: "img1"},
				{Image: "img2"},
				{Image: "img3"},
			},
			filter: []string{"img1", "img2"},
			want:   []string{"img1", "img2"},
		},
		{
			name: "empty, all filtered",
			builderList: BuilderList{
				{Image: "img1"},
				{Image: "img2"},
				{Image: "img3"},
			},
			filter: []string{"img4"},
			want:   []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.builderList.Names(tt.filter...)
			if !reflect.DeepEqual(tt.want, got) {
				t.Errorf("builderList.Names() want:%v, got%v", tt.want, got)
			}
		})
	}
}
