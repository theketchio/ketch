package docker

import (
	"encoding/base64"
	"encoding/json"
	"github.com/docker/distribution/reference"
	"strings"

	"github.com/docker/cli/cli/config"
	"github.com/docker/docker/api/types"
	"github.com/shipa-corp/ketch/internal/errors"
)

const (
	officialHost = "docker"
	dockerIndex  = "index.docker.io"
)

// GetImageAuthConfig checks for local docker configuration and extracts registry authorization information for an image
// if it exists.
func GetImageAuthConfig(image string) (*types.AuthConfig, error) {
	cfg, err := config.Load(config.Dir())
	if err != nil {
		return nil, errors.Wrap(err, "could not load docker config")
	}

	host, err := indexHostFromImage(image)
	if err != nil {
		return nil, errors.Wrap(err, "could not extract domain from image")
	}

	auth, err := cfg.GetAuthConfig(host)
	if err != nil {
		return nil, errors.Wrap(err, "could not obtain auth config")
	}

	return &types.AuthConfig{
		Username:      auth.Username,
		Password:      auth.Password,
		Auth:          auth.Auth,
		Email:         auth.Email,
		ServerAddress: auth.ServerAddress,
		IdentityToken: auth.IdentityToken,
		RegistryToken: auth.RegistryToken,
	}, nil

}

func getEncodedRegistryAuth(image string) (string, error) {
	auth, err := GetImageAuthConfig(image)
	if err != nil {
		return "", err
	}

	jsonAuth, err := json.Marshal(auth)
	if err != nil {
		return "", errors.Wrap(err, "could not json encode docker auth config")
	}

	return base64.URLEncoding.EncodeToString(jsonAuth), nil
}

func indexHostFromImage(img string) (string, error) {
	named, err := reference.ParseNormalizedNamed(img)
	if err != nil {
		return "", err
	}

	domain := reference.Domain(named)
	if isHostOfficial(domain) {
		return dockerIndex, nil
	}

	return reference.Domain(named), nil
}

func isHostOfficial(regHost string) bool {
	return strings.Contains(regHost, officialHost)
}
