package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type YamlFile struct {
	Name    string
	Content string
}

type context struct {
	Yamls []YamlFile
}

var (
	yamlsTemplate = `
package templates

type YamlFile struct {
	Name string
	Content string
}

var (
  DefaultYamls = map[string]string {
{{- range $_, $yaml := .Yamls }}
    "{{ $yaml.Name }}": 
{{ $yaml.Content }},
{{- end }}
  }
)
`
)

func main() {
	infos, err := ioutil.ReadDir("./internal/templates")
	if err != nil {
		panic(err)
	}

	yamls := []YamlFile{}
	for _, info := range infos {
		if !strings.HasSuffix(info.Name(), ".yaml") && !strings.HasSuffix(info.Name(), ".tpl") {
			continue
		}
		filename := filepath.Join("./internal/templates", info.Name())
		content, err := ioutil.ReadFile(filename)
		if err != nil {
			panic(err)
		}
		yamls = append(yamls, YamlFile{
			Name:    info.Name(),
			Content: fmt.Sprintf("`%s`", string(content)),
		})
	}

	tmpl, err := template.New("tpl").Parse(yamlsTemplate)

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, context{Yamls: yamls})
	if err != nil {
		panic(err)
	}
	rawFile := buf.Bytes()
	formatedFile, err := format.Source(rawFile)
	if err != nil {
		panic(err)
	}
	out := filepath.Join("./internal/templates", "yamls.go")
	file, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0660)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	file.Write(formatedFile)
}
