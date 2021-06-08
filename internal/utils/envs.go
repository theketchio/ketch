package utils

import (
	"errors"
	"strings"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

// MakeEnvironments takes an array of name value pairs delimited by '=' and convert
// them to an array of Env structures.
func MakeEnvironments(envs []string) ([]ketchv1.Env, error) {
	splittedEnvs := make([]ketchv1.Env, 0, len(envs))
	for _, env := range envs {
		parts := strings.Split(env, "=")
		if len(parts) != 2 {
			return nil, errors.New("env variables should have NAME=VALUE format")
		}
		splittedEnvs = append(splittedEnvs,
			ketchv1.Env{Name: strings.TrimSpace(parts[0]), Value: strings.TrimSpace(parts[1])})
	}
	return splittedEnvs, nil
}
