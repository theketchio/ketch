// Package errors contains various helpful utilities to assist with error handling.
package errors

import (
	"fmt"
	"path"
	"runtime"
)

// Wrap wraps error and supplies the line and the file where the error occurred.
func Wrap(err error, fmtStr string, params ...interface{}) error {
	_, fl, line, _ := runtime.Caller(1)
	msg := fmt.Sprintf(fmtStr, params...)
	return fmt.Errorf("message: %q; error: \"%w\"; file: %s; line: %d", msg, err, path.Base(fl), line)
}

func New(fmtStr string, params ...interface{}) error {
	_, fl, line, _ := runtime.Caller(1)
	msg := fmt.Sprintf(fmtStr, params...)
	return fmt.Errorf("message: %q; file: %s; line: %d", msg, path.Base(fl), line)
}
