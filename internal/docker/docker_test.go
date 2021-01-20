package docker

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)




func TestNew(t *testing.T) {
	var req BuildRequest
	req.BuildDirectory = "/tmp/ketch-build-361728058"

	req.Image = "murphybytes/foo:v0.1"
	req.Out = os.Stdout
	req.DockerConfigDir = "/home/jam/.docker"

	c, err := New()
	require.Nil(t, err )
	require.NotNil(t, c )

	res, err := c.Build(context.Background(), &req)
	require.Nil(t, err)
	t.Logf("%v", res)

}


func TestAuth(t *testing.T) {

	a, err := getEncodedRegistryAuth("/home/jam/.docker", "docker.io", false )
	require.Nil(t, err )
	t.Logf("%+v", a)

	a, err = getEncodedRegistryAuth("/home/jam/.docker", "murphybytes.jfrog.io", false)
	require.Nil(t, err )
	t.Logf("%+v", a)

}

