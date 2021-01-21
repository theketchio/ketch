package docker

import (
	"encoding/base64"
	"github.com/docker/cli/cli/config"
	cliTypes "github.com/docker/cli/cli/config/types"
	"github.com/docker/docker/api/types"
	regTypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/registry"
	"k8s.io/apimachinery/pkg/util/json"
	"strings"

	"github.com/shipa-corp/ketch/internal/errors"
)

const officialHost = "docker"

func getEncodedRegistryAuth(configPath string, regHost string, insecure bool) (string, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return "", errors.Wrap(err, "could not load docker config")
	}
	info := &regTypes.IndexInfo{Name: regHost, Secure: !insecure, Official: official(regHost)}
	auth := registry.ResolveAuthConfig(convert(cfg.AuthConfigs), info)
	jsonAuth, err := json.Marshal(auth)
	if err != nil {
		return "", errors.Wrap(err, "could not json encode docker auth config")
	}
	return base64.URLEncoding.EncodeToString(jsonAuth), nil
}

func official(regHost string) bool {
	return strings.Contains(regHost, officialHost)
}

func convert(auths map[string]cliTypes.AuthConfig) map[string]types.AuthConfig {
	result := make(map[string]types.AuthConfig)
	for k, v := range auths {
		result[k] = types.AuthConfig{
			Username:      v.Username,
			Password:      v.Password,
			Auth:          v.Auth,
			Email:         v.Email,
			ServerAddress: v.ServerAddress,
			IdentityToken: v.IdentityToken,
			RegistryToken: v.RegistryToken,
		}

	}
	return result
}
