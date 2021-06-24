package output

import (
	"io"
)

type writer interface {
	write() error
}

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
