package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuilderList(t *testing.T) {
	expected := `	Google:                gcr.io/buildpacks/builder:v1      GCP Builder for all runtimes                                                              
	Heroku:                heroku/buildpacks:18              heroku-18 base image with buildpacks for Ruby, Java, Node.js, Python, Golang, & PHP       
	Heroku:                heroku/buildpacks:20              heroku-20 base image with buildpacks for Ruby, Java, Node.js, Python, Golang, & PHP       
	Paketo Buildpacks:     paketobuildpacks/builder:base     Small base image with buildpacks for Java, Node.js, Golang, & .NET Core                   
	Paketo Buildpacks:     paketobuildpacks/builder:full     Larger base image with buildpacks for Java, Node.js, Golang, .NET Core, & PHP             
	Paketo Buildpacks:     paketobuildpacks/builder:tiny     Tiny base image (bionic build image, distroless run image) with buildpacks for Golang     
`

	var buff bytes.Buffer
	cmd := newBuilderListCmd(&buff)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Nil(t, err)
	require.Equal(t, expected, buff.String())
}
