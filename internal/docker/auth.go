package docker

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/docker/cli/cli/config"
	"github.com/shipa-corp/ketch/internal/errors"
)

const (
	officialHost = "docker"
	dockerIndex  = "index.docker.io"
)

func getEncodedRegistryAuth(regHost string) (string, error) {
	cfg, err := config.Load(config.Dir())
	if err != nil {
		return "", errors.Wrap(err, "could not load docker config")
	}

	auth, err := cfg.GetAuthConfig(norm(regHost))
	if err != nil {
		return "", errors.Wrap(err, "could not load auth from docker config")
	}

	jsonAuth, err := json.Marshal(auth)
	if err != nil {
		return "", errors.Wrap(err, "could not json encode docker auth config")
	}

	return base64.URLEncoding.EncodeToString(jsonAuth), nil
}

func norm(regHost string) string {
	if isHostOfficial(regHost) {
		return dockerIndex
	}
	return regHost
}

func isHostOfficial(regHost string) bool {
	return strings.Contains(regHost, officialHost)
}
