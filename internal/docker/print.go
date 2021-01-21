package docker

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"k8s.io/apimachinery/pkg/util/json"

	"github.com/shipa-corp/ketch/internal/errors"
)

// ResponseLine represents the json status messages streamed from docker operations.  We decode these message
// and turn them into log lines to be viewed by user.
type ResponseLine struct {
	Status *string `json:"status,omitempty"`
	Stream *string `json:"stream,omitempty"`
	// Error if this is populated some aspect of the operation, say a build, failed. In these cases ImageBuild will
	// not return an error because the docker code worked, but our docker file was hosed for example.
	Error          *string         `json:"error,omitempty"`
	Progress       *string         `json:"progress,omitempty"`
	Aux            *Aux            `json:"aux,omitempty"`
	ErrorDetail    *ErrorDetail    `json:"errorDetail,omitempty"`
	ProgressDetail *ProgressDetail `json:"progressDetail,omitempty"`
	ID             *string         `json:"id,omitempty"`
}

type Aux struct {
	Tag    *string `json:"Tag,omitempty"`
	Digest *string `json:"Digest,omitempty"`
	Size   *int    `json:"Size,omitempty"`
	ID     *string `json:"ID,omitempty"`
}

type Builder struct {
	strings.Builder
}

func (b *Builder) Appendf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	if b.Len() > 0 {
		b.WriteString(fmt.Sprintf(", %s", s))
		return
	}
	b.WriteString(s)
}

func (b *Builder) Append(v string) {
	if v == "" {
		return
	}

	if b.Len() > 0 {
		b.WriteString(fmt.Sprintf(", %s", v))
		return
	}
	b.WriteString(v)
}

func (a Aux) String() string {
	var resp Builder
	if a.ID != nil {
		resp.Appendf("ID: %s", *a.ID)
	}
	if a.Tag != nil {
		resp.Appendf("Tag: %s", *a.Tag)
	}
	if a.Digest != nil {
		resp.Appendf("Digest: %s", *a.Digest)
	}
	if a.Size != nil {
		resp.Appendf("Size: %d", *a.Size)
	}
	return resp.String()
}

type ErrorDetail struct {
	Message string `json:"message"`
}

type ProgressDetail struct {
	Current *int `json:"current,omitempty"`
	Total   *int `json:"total,omitempty"`
}

func (pd ProgressDetail) String() string {
	if pd.Current != nil && pd.Total != nil {
		return fmt.Sprintf("current %d, total %d", *pd.Current, *pd.Total)
	}
	return ""
}

func NewLine(b []byte) (*ResponseLine, error) {
	var rl ResponseLine
	if err := json.Unmarshal(b, &rl); err != nil {
		return nil, err
	}
	return &rl, nil
}

func print(rdr io.ReadCloser, wtr io.Writer) error {
	defer rdr.Close()
	scanner := bufio.NewScanner(rdr)

	for scanner.Scan() {
		rl, err := NewLine(scanner.Bytes())
		if err != nil {
			fmt.Fprintf(wtr, "could not process message: %q\n", scanner.Bytes())
			continue
		}

		fmt.Fprint(wtr, rl)

		if err := Error(rl); err != nil {
			return err
		}
	}
	return nil
}

// String converts the ResponseLine into a textual representation for human consumption.
func (rl ResponseLine) String() string {
	if rl.Stream != nil {
		return alf(*rl.Stream)
	}
	if rl.Aux != nil {
		return alf(rl.Aux.String())
	}
	if rl.ErrorDetail != nil {
		return alf(rl.ErrorDetail.Message)
	}
	if rl.Status != nil {
		var s Builder
		s.Append(*rl.Status)
		if rl.ProgressDetail != nil {
			s.Append(rl.ProgressDetail.String())
		}
		if rl.Progress != nil {
			s.Appendf("progress: %s", *rl.Progress)
		}
		if rl.ID != nil {
			s.Appendf("id: %s", *rl.ID)
		}
		return alf(s.String())
	}
	return ""
}

func alf(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

func Error(rl *ResponseLine) error {
	if rl.Error != nil {
		return errors.New(*rl.Error)
	}
	return nil
}
