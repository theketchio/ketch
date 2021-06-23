package output

import (
	"fmt"
	"io"

	"sigs.k8s.io/yaml"
)

// yamlOutput represents data and a writer for yaml output type
type yamlOutput struct {
	data   interface{}
	writer io.Writer
}

// write implements Writer for type YAML
func (y *yamlOutput) write() error {
	d, err := yaml.Marshal(y.data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(y.writer, string(d))
	return err
}
