package output

import (
	"io"

	"github.com/spf13/pflag"
)

type Writer interface {
	Write() error
}

func Write(data interface{}, out io.Writer, flags *pflag.FlagSet) error {
	outputFlag, err := flags.GetString("output")
	if err != nil {
		outputFlag = ""
	}
	var w Writer
	switch outputFlag {
	case "json", "JSON", "Json", "j":
		w = &JSON{
			Data:   data,
			Writer: out,
		}
	case "yaml", "YAML", "Yaml", "y":
		w = &YAML{
			Data:   data,
			Writer: out,
		}
	default:
		w = &Column{
			Data:   data,
			Writer: out,
		}
	}
	return w.Write()
}
