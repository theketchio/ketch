// Package chart contains types and methods to convert the App CRD to an internal representation of a helm chart
// and install it to a kubernetes cluster.
package chart

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"text/template"

	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/templates"
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

// httpsEndpoint holds cname and its corresponding secret name with SSL certificates.
type httpsEndpoint struct {
	Cname string `json:"cname"`

	// SecretName is a name of a Kubernetes Secret to store SSL certificate for the cname.
	SecretName string `json:"secretName"`
}

// Ingress contains information about entrypoints of an application.
// Both istio and traefik templates use "ingress" to render Kubernetes Ingress objects.
type ingress struct {
	Http  []string        `json:"http"`
	Https []httpsEndpoint `json:"https"`
}

type app struct {
	Name        string        `json:"name"`
	Deployments []deployment  `json:"deployments"`
	Env         []ketchv1.Env `json:"env"`
	Ingress     ingress       `json:"ingress"`

	// IsAccessible if not set, ketch won't create kubernetes objects like Ingress/Gateway to handle incoming request.
	// These objects could be broken without valid routes to the application.
	// For example, "spec.rules" of an Ingress object must contain at least one rule.
	IsAccessible bool `json:"isAccessible"`
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
	ImagePullSecret string `json:"imagePullSecret"`
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
func New(application *ketchv1.App, pool *ketchv1.Pool, opts ...Option) (*ApplicationChart, error) {

	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	var https []string
	if len(pool.Spec.IngressController.ClusterIssuer) > 0 {
		// cluster issuer is mandatory to obtain SSL certificates.
		https = application.Spec.CNames.Https
	}
	values := &values{
		App: &app{
			Name:    application.Name,
			Ingress: newIngress(application.Name, https, application.DefaultCname(pool)),
			Env:     application.Spec.Env,
		},
		IngressController: &pool.Spec.IngressController,
		DockerRegistry: dockerRegistrySpec{
			ImagePullSecret: application.Spec.DockerRegistry.SecretName,
		},
	}

	for _, deploymentSpec := range application.Spec.Deployments {
		deployment := deployment{
			Image:   deploymentSpec.Image,
			Version: deploymentSpec.Version,
			Labels:  deploymentSpec.Labels,
		}
		procfile, err := ProcfileFromProcesses(deploymentSpec.Processes)
		if err != nil {
			return nil, err
		}
		exposedPorts := options.ExposedPorts[deployment.Version]
		c := NewConfigurator(deploymentSpec.KetchYaml, *procfile, exposedPorts, DefaultApplicationPort)
		for _, processSpec := range deploymentSpec.Processes {
			name := processSpec.Name
			isRoutable := procfile.IsRoutable(name)

			process, err := newProcess(name, isRoutable,
				withCmd(c.ProcessCmd(name)),
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
	values.App.IsAccessible = isAppAccessible(values.App)
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

func isAppAccessible(a *app) bool {
	if len(a.Ingress.Http)+len(a.Ingress.Https) == 0 {
		return false
	}
	for _, deployment := range a.Deployments {
		for _, process := range deployment.Processes {
			if process.Routable {
				return true
			}
		}
	}
	return false
}

func newIngress(appName string, httpsCnames []string, defaultCname *string) ingress {
	https := make([]httpsEndpoint, 0, len(httpsCnames))
	for _, cname := range httpsCnames {
		hash := sha256.New()
		hash.Write([]byte(fmt.Sprintf("cname-%s", cname)))
		bs := hash.Sum(nil)
		secretName := fmt.Sprintf("%s-cname-%x", appName, bs[:10])
		https = append(https, httpsEndpoint{Cname: cname, SecretName: secretName})
	}
	var http []string
	if defaultCname != nil {
		http = []string{*defaultCname}
	}
	return ingress{
		Http:  http,
		Https: https,
	}
}
