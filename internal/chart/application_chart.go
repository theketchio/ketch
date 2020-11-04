// Package chart contains types and methods to convert the App CRD to an internal representation of a helm chart
// and install it to a kubernetes cluster.
package chart

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/pkg/chartutil"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/templates"
	"helm.sh/helm/v3/pkg/chart/loader"
)

// ApplicationChart is an internal representation of a helm chart converted from the App CRD
// and is used to render a helm chart.
type ApplicationChart struct {
	values    values
	templates map[string]string
}

type values struct {
	App               *app                           `json:"app"`
	DockerRegistry    dockerRegistrySpec             `json:"dockerRegistry"`
	IngressController *ketchv1.IngressControllerSpec `json:"ingressController"`
}

type app struct {
	Name        string            `json:"name"`
	Deployments []deployment      `json:"deployments"`
	Env         []ketchv1.Env     `json:"env"`
	Cnames      ketchv1.CnameList `json:"cnames"`
}

type deployment struct {
	Image           string                    `json:"image"`
	Version         ketchv1.DeploymentVersion `json:"version"`
	Processes       []process                 `json:"processes"`
	Labels          []ketchv1.Label           `json:"labels"`
	RoutingSettings ketchv1.RoutingSettings   `json:"routingSettings"`
	DeploymentExtra deploymentExtra           `json:"extra"`
}

type deploymentExtra struct {
	Volumes []v1.Volume `json:"volumes,omitempty"`
}

type dockerRegistrySpec struct {
	ImagePullSecret       string `json:"imagePullSecret"`
	CreateImagePullSecret bool   `json:"createImagePullSecret"`
	RegistryName          string `json:"registryName"`
	Username              string `json:"username"`
	Password              string `json:"password"`
}

type Option func(opts *Options)

type Options struct {
	// ExposedPorts are ports exposed by an image of each deployment.
	ExposedPorts map[ketchv1.DeploymentVersion][]ketchv1.ExposedPort
	Templates    templates.Templates
}

func WithExposedPorts(ports map[ketchv1.DeploymentVersion][]ketchv1.ExposedPort) Option {
	return func(opts *Options) {
		opts.ExposedPorts = make(map[ketchv1.DeploymentVersion][]ketchv1.ExposedPort, len(ports))
		for version, ps := range ports {
			if len(ps) == 0 {
				opts.ExposedPorts[version] = []ketchv1.ExposedPort{{Port: DefaultApplicationPort, Protocol: "TCP"}}
			} else {
				opts.ExposedPorts[version] = ps
			}
		}
	}
}

func WithTemplates(tpls templates.Templates) Option {
	return func(opts *Options) {
		opts.Templates = tpls
	}
}

// New returns an ApplicationChart instance.
func New(name string, appSpec ketchv1.AppSpec, pool ketchv1.PoolSpec, opts ...Option) (*ApplicationChart, error) {
	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	cnames := appSpec.Ingress.Cnames
	ingress := pool.IngressController
	if appSpec.Ingress.GenerateDefaultCname && len(ingress.ServiceEndpoint) > 0 {
		domain := ketchv1.ShipaCloudDomain
		if len(ingress.Domain) > 0 {
			domain = ingress.Domain
		}
		cnames = append(cnames, fmt.Sprintf("%s.%s.%s", name, ingress.ServiceEndpoint, domain))
	}
	values := &values{
		App: &app{
			Name:   name,
			Cnames: cnames,
			Env:    appSpec.Env,
		},
		IngressController: &ingress,
		DockerRegistry: dockerRegistrySpec{
			ImagePullSecret: appSpec.DockerRegistry.SecretName,
		},
	}

	for _, deploymentSpec := range appSpec.Deployments {
		content := make([]string, 0, len(deploymentSpec.Processes))
		for _, spec := range deploymentSpec.Processes {
			content = append(content, fmt.Sprintf("%s:%s", spec.Name, spec.Cmd[0]))
		}
		deployment := deployment{
			Image:   deploymentSpec.Image,
			Version: deploymentSpec.Version,
			Labels:  deploymentSpec.Labels,
		}
		procfileContent := strings.Join(content, "\n")
		procfile, err := ParseProcfile(procfileContent)
		if err != nil {
			return nil, err
		}
		exposedPorts := options.ExposedPorts[deployment.Version]
		c := NewConfigurator(deploymentSpec.KetchYaml, *procfile, exposedPorts, DefaultApplicationPort)
		for _, processSpec := range deploymentSpec.Processes {
			name := processSpec.Name
			isRoutable := procfile.IsRoutable(name)

			process, err := newProcess(name, isRoutable,
				withCmd(c.ProcessCmd(name, deploymentSpec.WorkingDir, nil)),
				withUnits(processSpec.Units),
				withPortsAndProbes(c),
				withLifecycle(c.Lifecycle()),
				withSecurityContext(processSpec.SecurityContext))

			if err != nil {
				return nil, err
			}

			deployment.Processes = append(deployment.Processes, *process)
		}
		values.App.Deployments = append(values.App.Deployments, deployment)
	}
	return &ApplicationChart{
		values:    *values,
		templates: options.Templates.Yamls,
	}, nil
}

func (chrt ApplicationChart) getValues() (map[string]interface{}, error) {
	bs, err := yaml.Marshal(chrt.values)
	if err != nil {
		return nil, err
	}
	vals, err := chartutil.ReadValues(bs)
	if err != nil {
		return nil, err
	}
	return vals, nil
}

const (
	chartYaml = `apiVersion: v2
name: {{ .AppName }}
description: {{ .Description }} 
type: application
version: {{ .Version }}
{{- if .AppVersion }}
appVersion: {{ .AppVersion }}
{{- end }}
`
)

type chartYamlContext struct {
	AppName     string
	Version     string
	Description string
	AppVersion  string
}

// ChartConfig contains data used to render the helm chart's "Chart.yaml" file.
type ChartConfig struct {
	ChartVersion string
	AppName      string
	AppVersion   string
}

func (chrt ApplicationChart) ExportToDirectory(directory string, chartConfig ChartConfig) error {
	dir := path.Join(directory, chartConfig.AppName)
	err := os.RemoveAll(dir)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Join(dir, "templates"), os.ModePerm)
	if err != nil {
		return err
	}
	for filename, content := range chrt.templates {
		path := filepath.Join(dir, "templates", filename)
		err = ioutil.WriteFile(path, []byte(content), 0644)
		if err != nil {
			return err
		}
	}
	valuesBytes, err := yaml.Marshal(chrt.values)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath.Join(dir, "values.yaml"), valuesBytes, 0644)
	if err != nil {
		return err
	}

	chartYamlBuf := bytes.Buffer{}
	t := template.Must(template.New("chart.yaml").Parse(chartYaml))
	context := chartYamlContext{
		AppName:     chartConfig.AppName,
		Description: "application chart",
		Version:     chartConfig.ChartVersion,
		AppVersion:  chartConfig.AppVersion,
	}
	err = t.Execute(&chartYamlBuf, context)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath.Join(dir, "Chart.yaml"), chartYamlBuf.Bytes(), 0644)
	if err != nil {
		return err
	}
	return nil
}

func (chrt ApplicationChart) bufferedFiles(chartConfig ChartConfig) ([]*loader.BufferedFile, error) {
	files := make([]*loader.BufferedFile, 0, len(chrt.templates)+1)
	for filename, content := range chrt.templates {
		files = append(files, &loader.BufferedFile{
			Name: filepath.Join("templates", filename),
			Data: []byte(content),
		})
	}
	valuesBytes, err := yaml.Marshal(chrt.values)
	if err != nil {
		return nil, err
	}
	files = append(files, &loader.BufferedFile{
		Name: "values.yaml",
		Data: valuesBytes,
	})

	chartYamlBuf := bytes.Buffer{}
	t := template.Must(template.New("chart.yaml").Parse(chartYaml))
	context := chartYamlContext{AppName: chartConfig.AppName,
		Description: "application chart",
		Version:     chartConfig.ChartVersion,
		AppVersion:  chartConfig.AppVersion,
	}
	err = t.Execute(&chartYamlBuf, context)
	if err != nil {
		return nil, err
	}
	files = append(files, &loader.BufferedFile{
		Name: "Chart.yaml",
		Data: chartYamlBuf.Bytes(),
	})
	return files, nil
}

// AppName returns a name of the application.
func (chrt ApplicationChart) AppName() string {
	return chrt.values.App.Name
}
