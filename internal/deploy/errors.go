package deploy

import (
	"fmt"
)

type Error struct {
	description string
}

func (e Error) Error() string {
	return e.description
}

func (e Error) String() string {
	return e.description
}

// NewError creates a deploy error
func NewError(fmtVal string, v ...interface{}) error {
	return &Error{
		description: fmt.Sprintf(fmtVal, v...),
	}
}
