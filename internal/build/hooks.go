package build

import (
	"io/ioutil"
	"os"
	"path"
	"sigs.k8s.io/yaml"

	"github.com/shipa-corp/ketch/internal/errors"
)

// Shipa holds the decoded results of a shipa.yaml file
type Shipa struct {
	Hooks *Hooks `json:"hooks,omitempty"`
}

// Hooks contain build hooks from a shipa yaml file. Hooks are shell scripts that
// run during deployments. See https://learn.shipa.io/docs/shipayml
type Hooks struct {
	Build []string `json:"build,omitempty"`
}



func getHooks(workingDir string, shipaFileSearchPaths []string)([]string,error){
	for _, searchPath := range shipaFileSearchPaths {
		fullPath := path.Join(workingDir, searchPath)
		for _, base := range []string{"shipa.yaml", "shipa.yml"} {
			hooks, err := decodeHooks(path.Join(fullPath, base))
			if err != nil {
				return nil, err
			}
			if len(hooks) > 0 {
				return hooks, nil
			}
		}
	}
	return nil, nil
}

func decodeHooks(candidate string)([]string, error){
	_, err := os.Stat(candidate)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "test for shipa file failed")
	}

	b, err := ioutil.ReadFile(candidate)
	if err != nil {
		return nil, errors.Wrap(err, "could not read shipa file %q", candidate)
	}
	var s Shipa
	if err := yaml.Unmarshal(b, &s); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal shipa yaml")
	}
	if s.Hooks != nil {
		return s.Hooks.Build, nil
	}
	return nil, nil
}