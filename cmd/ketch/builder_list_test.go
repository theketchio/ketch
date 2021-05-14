package main

import (
	"bytes"
	"github.com/shipa-corp/ketch/internal/mocks"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/shipa-corp/ketch/cmd/ketch/configuration"
)

const (
	defaultBuilders = `	Google:                gcr.io/buildpacks/builder:v1      GCP Builder for all runtimes                                                              
	Heroku:                heroku/buildpacks:18              heroku-18 base image with buildpacks for Ruby, Java, Node.js, Python, Golang, & PHP       
	Heroku:                heroku/buildpacks:20              heroku-20 base image with buildpacks for Ruby, Java, Node.js, Python, Golang, & PHP       
	Paketo Buildpacks:     paketobuildpacks/builder:base     Small base image with buildpacks for Java, Node.js, Golang, & .NET Core                   
	Paketo Buildpacks:     paketobuildpacks/builder:full     Larger base image with buildpacks for Java, Node.js, Golang, .NET Core, & PHP             
	Paketo Buildpacks:     paketobuildpacks/builder:tiny     Tiny base image (bionic build image, distroless run image) with buildpacks for Golang     
`
	userBuilders = `	Google:                gcr.io/buildpacks/builder:v1      GCP Builder for all runtimes                                                              
	Heroku:                heroku/buildpacks:18              heroku-18 base image with buildpacks for Ruby, Java, Node.js, Python, Golang, & PHP       
	Heroku:                heroku/buildpacks:20              heroku-20 base image with buildpacks for Ruby, Java, Node.js, Python, Golang, & PHP       
	Paketo Buildpacks:     paketobuildpacks/builder:base     Small base image with buildpacks for Java, Node.js, Golang, & .NET Core                   
	Paketo Buildpacks:     paketobuildpacks/builder:full     Larger base image with buildpacks for Java, Node.js, Golang, .NET Core, & PHP             
	Paketo Buildpacks:     paketobuildpacks/builder:tiny     Tiny base image (bionic build image, distroless run image) with buildpacks for Golang     
	test vendor:           test image                        test description                                                                          
`
)

func TestBuilderList(t *testing.T) {

	tests := []struct {
		name     string
		cfg      config
		expected string
	}{
		{
			name: "default values",
			cfg: &mocks.Configuration{
				KetchConfigObj: configuration.KetchConfig{},
			},
			expected: defaultBuilders,
		},
		{
			name: "include user's builders",
			cfg: &mocks.Configuration{
				KetchConfigObj: configuration.KetchConfig{
					AdditionalBuilders: []configuration.AdditionalBuilder{
						{
							Vendor:      "test vendor",
							Image:       "test image",
							Description: "test description",
						},
					},
				},
			},
			expected: userBuilders,
		},
	}

	for _, tt := range tests {
		var buff bytes.Buffer
		cmd := newBuilderListCmd(tt.cfg, &buff)
		cmd.SetArgs([]string{})
		err := cmd.Execute()
		require.Nil(t, err)
		require.Equal(t, tt.expected, buff.String())
	}
}
