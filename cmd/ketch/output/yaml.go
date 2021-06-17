package output

import (
	"fmt"
	"io"

	"sigs.k8s.io/yaml"
)

// YAML represents data and a writer for yaml output type
type YAML struct {
	Data   interface{}
	Writer io.Writer
}

// Write implements Writer for type YAML
func (y *YAML) Write() error {
	d, err := yaml.Marshal(y.Data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(y.Writer, string(d))
	return err
}
