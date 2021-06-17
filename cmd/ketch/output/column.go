package output

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"
	"text/tabwriter"
)

// Column represents data and a writer for standard output type
type Column struct {
	Data   interface{}
	Writer io.Writer
}

type val struct {
	tag   reflect.StructTag
	value reflect.Value
}

type valSet []val

// Write implements Writer for type Column
func (c *Column) Write() error {
	d, err := c.Marshal(c.Data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(c.Writer, string(d))
	return err
}

// Marshal creates valSets from v, depending on whether it is a slice, struct, or pointer.
// It then prints data to a tabwriter. Column headings are pulled from the "column" struct tag
// or spaced-and-capitalized if the tag does not exist. Data is then printed to the tabwriter.
func (c *Column) Marshal(v interface{}) ([]byte, error) {
	var valSets []valSet

	value := reflect.ValueOf(v)
	switch value.Kind() {
	case reflect.Struct:
		valSet := newValSet(value)
		valSets = append(valSets, valSet)
	case reflect.Slice:
		for i := 0; i < reflect.Value.Len(value); i++ {
			valSet := newValSet(value.Index(i))
			valSets = append(valSets, valSet)
		}
	case reflect.Ptr:
		valSet := newValSet(value.Elem())
		valSets = append(valSets, valSet)
	default:
		return nil, fmt.Errorf("unsupported kind: %s", value.Kind())
	}

	// no data
	if len(valSets) < 1 {
		return nil, nil
	}

	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 4, 4, ' ', 0)

	// headers
	for i, val := range valSets[0] {
		tag := val.tag.Get("column")
		// omit?
		if tag == "-" {
			continue
		}

		fmt.Fprint(w, tag)
		// tab
		if i+1 < len(valSets[0]) {
			fmt.Fprint(w, "\t")
		}
	}
	fmt.Fprint(w, "\n")

	// fields
	for i, valSet := range valSets {
		for j, val := range valSet {
			// omit?
			if val.tag.Get("column") == "-" {
				continue
			}

			fmt.Fprint(w, val.value)

			// tab
			if j+1 < len(valSet) {
				fmt.Fprint(w, "\t")

			}
		}
		// newline
		if i+1 < len(valSets) {
			fmt.Fprint(w, "\n")
		}
	}
	err := w.Flush()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// newValSet iterates over a value's fields and assigns the Value and StructTag to a valSet
func newValSet(value reflect.Value) valSet {
	var valSet valSet
	for i := 0; i < value.NumField(); i++ {
		tag := value.Type().Field(i).Tag
		// use Field Type as StructTag
		if tag.Get("column") == "" {
			fieldName := value.Type().Field(i).Name
			// split and uppercase field name
			var builder strings.Builder
			for i, r := range fieldName {
				if r > 96 && r < 123 {
					builder.WriteString(string(r - 32))
				} else {
					if i > 0 {
						builder.WriteString(" ")
					}
					builder.WriteString(string(r))
				}
			}
			tag = reflect.StructTag(fmt.Sprintf("column:\"%s\"", builder.String()))
		}
		valSet = append(valSet, val{tag: tag, value: value.Field(i)})
	}
	return valSet
}
