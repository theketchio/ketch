package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// JSON represents data and a writer for json output type
type JSON struct {
	Data   interface{}
	Writer io.Writer
}

// Write implements Writer for type JSON
func (j *JSON) Write() error {
	d, err := json.MarshalIndent(j.Data, "", "\t")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(j.Writer, string(d))
	return err
}
