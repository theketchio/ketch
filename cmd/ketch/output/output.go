package output

import (
	"errors"
	"io"
	"os"

	"sigs.k8s.io/yaml"
)

type writer interface {
	write() error
}

var ErrFileExists = errors.New("file already exists")

// Write writes data to out, switching marshaling type based on outputFlag
func Write(data interface{}, out io.Writer, outputFlag string) error {
	var w writer
	switch outputFlag {
	default:
		w = &columnOutput{
			data:   data,
			writer: out,
		}
	}
	return w.write()
}

// WriteToFileOrOut marshals output to yaml and writes to file, if a filename is passed, or out.
func WriteToFileOrOut(output interface{}, out io.Writer, filename string) error {
	if filename != "" {
		_, err := os.Stat(filename)
		if !os.IsNotExist(err) {
			return ErrFileExists
		}
		f, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}
	b, err := yaml.Marshal(output)
	if err != nil {
		return err
	}
	_, err = out.Write(b)
	return err
}
