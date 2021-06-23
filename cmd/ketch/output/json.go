package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// jsonOutput represents data and a writer for json output type
type jsonOutput struct {
	data   interface{}
	writer io.Writer
}

// write implements Writer for type JSON
func (j *jsonOutput) write() error {
	d, err := json.MarshalIndent(j.data, "", "\t")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(j.writer, string(d))
	return err
}
