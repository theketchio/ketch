package docker

import (
	"encoding/base64"
	"encoding/json"
	"github.com/docker/docker/api/types"
	v1 "k8s.io/api/core/v1"
	"sort"
	"strconv"
	"strings"
)

type authConfigFn func(image string) (*types.AuthConfig, error)

type secretGenerator struct {
	authConfig authConfigFn
}

func GenerateSecret(image, namespace string) (*v1.Secret, error) {
	gen := secretGenerator{
		authConfig: GetImageAuthConfig,
	}
	return gen.generate(image, namespace)

}

func (sg secretGenerator) generate(image, namespace string) (*v1.Secret, error) {
	auth, err := sg.authConfig(image)
	if err != nil {
		return nil, err
	}

	var secret v1.Secret
	pName, err := makeK8sName(auth.Username, auth.ServerAddress)
	if err != nil {
		return nil, err
	}

	secret.Name = *pName
	secret.Namespace = namespace
	secret.Type = v1.SecretTypeDockerConfigJson
	secret.Data = map[string][]byte{}

	entry := ConfigEntry{
		Username: auth.Username,
		Password: auth.Password,
		Email:    auth.Email,
		Auth:     base64.StdEncoding.EncodeToString([]byte(auth.Username + ":" + auth.Password)),
	}

	conf := ConfigJSON{
		Auths: map[string]ConfigEntry{auth.ServerAddress: entry},
	}

	jsonContent, err := json.Marshal(conf)
	if err != nil {
		return nil, err
	}

	secret.Data[v1.DockerConfigJsonKey] = jsonContent
	return &secret, nil
}

type ConfigEntry struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Email    string `json:"email,omitempty"`
	Auth     string `json:"auth,omitempty"`
}

type ConfigJSON struct {
	Auths       map[string]ConfigEntry `json:"auths"`
	HttpHeaders map[string]string      `json:"HttpHeaders,omitempty"`
}

type nameError string

func (ne nameError) Error() string { return string(ne) }

const (
	maxNameLength = 253
	minNameLength = 6
	spaceChar     = 32
	escapeChar    = 126

	errNonPrintableCharacter nameError = "can't use non-printable character to generate secret name"
	errNameTooLong           nameError = "name exceeds maximum length"
	errNameTooShort          nameError = "name must be at least six characters"
)

// Create a name that can be used for k8s resources conforming to
// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names
// Note that the name we produce is repeatable, the same inputs yield the same output. Therefore, if
// the length of the parts is too long, we return an error. We could truncate and return the name, but there
// would be no way to compare too names because we wouldn't have access to the removed characters.
func makeK8sName(parts ...string) (*string, error) {
	var builder strings.Builder
	builder.Grow(maxNameLength * 2)

	sort.Strings(parts)

	for _, part := range parts {
		if err := mapBytesToLegalNameChars(&builder, part); err != nil {
			return nil, err
		}
		if builder.Len() > maxNameLength {
			return nil, errNameTooLong
		}
	}

	if builder.Len() < minNameLength {
		return nil, errNameTooShort
	}
	result := builder.String()
	return &result, nil
}

func mapBytesToLegalNameChars(builder *strings.Builder, s string) error {
	for _, b := range []byte(s) {
		if !isPrintable(b) {
			return errNonPrintableCharacter
		}
		switch {
		case b >= 'a' && b <= 'z':
			builder.WriteByte(b)
		case b >= 'A' && b <= 'Z':
			builder.WriteByte(b | 32)
		case b >= '0' && b <= '9':
			builder.WriteByte(b)
		default:
			builder.WriteString(strconv.FormatUint(uint64(b), 16))
		}
	}

	return nil
}

func isPrintable(b byte) bool {
	return b >= spaceChar && b <= escapeChar
}
