package output

import (
	"errors"
	"io"
	"os"
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

// GetOutputFile creates file, erring if it exists
func GetOutputFile(filename string) (*os.File, error) {
	_, err := os.Stat(filename)
	if !os.IsNotExist(err) {
		return nil, ErrFileExists
	}
	return os.Create(filename)
}
