// +build acceptance

package docker

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/require"
)

// this test runs against your local docker credential store.  It will be skipped unless
// the acceptance build tag is included in go flags and environment variables are set
// containing your user and password to test against.
func Test_getEncodedRegistryAuth(t *testing.T) {
	expectedUser, ok := os.LookupEnv("KETCH_TEST_DOCKER_USER")
	if !ok {
		t.Skip("env KETCH_TEST_DOCKER_USER missing")
	}
	expectedPwd, ok := os.LookupEnv("KETCH_TEST_DOCKER_PASSWORD")
	if !ok {
		t.Skip("env KETCH_TEST_DOCKER_PASSWORD missing")
	}
	res, err := getEncodedRegistryAuth("docker.io")
	require.Nil(t, err)
	b, err := base64.URLEncoding.DecodeString(res)
	require.Nil(t, err)
	var auth types.AuthConfig
	err = json.Unmarshal(b, &auth)
	require.Nil(t, err)
	require.Equal(t, expectedUser, auth.Username)
	require.Equal(t, expectedPwd, auth.Password)
}
