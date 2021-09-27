// Package chart contains types and methods to convert the App CRD to an internal representation of a helm chart
// and install it to a kubernetes cluster.
package chart

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"helm.sh/helm/v3/pkg/chartutil"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/pkg/errors"
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
	IngressController *ketchv1.IngressControllerSpec `json:"ingressController"`
}

// httpsEndpoint holds cname and its corresponding secret name with SSL certificates.
type httpsEndpoint struct {
	Cname string `json:"cname"`

	// SecretName is a name of a Kubernetes Secret to store SSL certificate for the cname.
	SecretName string `json:"secretName"`

	// ClusterIssuer is the name of the cert-manager's ClusterIssuer to use for this endpoint (overriding the Framework default).
	ClusterIssuer string `json:"clusterIssuer"`
}

// Ingress contains information about entrypoints of an application.
// Both istio and traefik templates use "ingress" to render Kubernetes Ingress objects.
type ingress struct {

	// Https is a list of http entrypoints.
	Http []string `json:"http"`

	// Https is a list of https entrypoints.
	Https []httpsEndpoint `json:"https"`
}

// gatewayService contains values for populating the gateway_service.yaml
type gatewayService struct {
	Deployment deployment
	Process    process
}

type app struct {
	Name        string        `json:"name"`
	Deployments []deployment  `json:"deployments"`
	Env         []ketchv1.Env `json:"env"`
	Ingress     ingress       `json:"ingress"`
	// IsAccessible if not set, ketch won't create kubernetes objects like Ingress/Gateway to handle incoming request.
	// These objects could be broken without valid routes to the application.
	// For example, "spec.rules" of an Ingress object must contain at least one rule.
	IsAccessible bool   `json:"isAccessible"`
	Group        string `json:"group"`
	Secret       string `json:"secret"`
	Service      *gatewayService
}

type deployment struct {
	ImagePullSecrets []v1.LocalObjectReference `json:"imagePullSecrets"`
	Image            string                    `json:"image"`
	Version          ketchv1.DeploymentVersion `json:"version"`
	Processes        []process                 `json:"processes"`
	Labels           []ketchv1.Label           `json:"labels"`
	RoutingSettings  ketchv1.RoutingSettings   `json:"routingSettings"`
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

func imagePullSecrets(deploymentImagePullSecrets []v1.LocalObjectReference, spec ketchv1.DockerRegistrySpec) []v1.LocalObjectReference {
	if len(deploymentImagePullSecrets) > 0 {
		// imagePullSecrets defined for this particular deployment is higher priority.
		return deploymentImagePullSecrets
	}
	if len(spec.SecretName) == 0 {
		return nil
	}
	return []v1.LocalObjectReference{
		{Name: spec.SecretName},
	}
}

// New returns an ApplicationChart instance.
func New(application *ketchv1.App, framework *ketchv1.Framework, opts ...Option) (*ApplicationChart, error) {

	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}

	ingress, err := newIngress(*application, *framework)
	if err != nil {
		return nil, err
	}

	values := &values{
		App: &app{
			Name:    application.Name,
			Ingress: *ingress,
			Env:     application.Spec.Env,
			Group:   ketchv1.Group,
			Secret:  application.Spec.SecretName,
		},
		IngressController: &framework.Spec.IngressController,
	}

	for _, deploymentSpec := range application.Spec.Deployments {
		deployment := deployment{
			Image:   deploymentSpec.Image,
			Version: deploymentSpec.Version,
			Labels:  deploymentSpec.Labels,
			RoutingSettings: ketchv1.RoutingSettings{
				Weight: deploymentSpec.RoutingSettings.Weight,
			},
			ImagePullSecrets: imagePullSecrets(deploymentSpec.ImagePullSecrets, application.Spec.DockerRegistry),
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
				withCmd(c.procfile.Processes[name]),
				withUnits(processSpec.Units),
				withEnvs(processSpec.Env),
				withPortsAndProbes(c),
				withLifecycle(c.Lifecycle()),
				withSecurityContext(processSpec.SecurityContext),
				withResourceRequirements(processSpec.Resources),
				withVolumes(processSpec.Volumes),
				withVolumeMounts(processSpec.VolumeMounts),
				withLabels(application.Spec.Labels, deployment.Version),
				withAnnotations(application.Spec.Annotations, deployment.Version),
			)
			if err != nil {
				return nil, err
			}

			// the most recent version will always be the last entry in the array. In the event of
			// a rollback the most recent version is still in the array, but its weight will be changed to 0
			if isRoutable && deploymentSpec.RoutingSettings.Weight > 0 {
				values.App.Service = &gatewayService{
					Deployment: deployment,
					Process:    *process,
				}
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

func (chrt ApplicationChart) getValuesMap() (map[string]interface{}, error) {
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

// ChartConfig contains data used to render the helm chart's "Chart.yaml" file.
type ChartConfig struct {
	Version     string
	Description string
	AppName     string
	AppVersion  string
}

// NewChartConfig returns a ChartConfig instance based on the given application.
func NewChartConfig(app ketchv1.App) ChartConfig {
	version := fmt.Sprintf("v%v", app.ObjectMeta.Generation)
	chartVersion := fmt.Sprintf("v0.0.%v", app.ObjectMeta.Generation)
	if app.Spec.Version != nil {
		version = *app.Spec.Version
	}
	return ChartConfig{
		Version:     chartVersion,
		Description: app.Spec.Description,
		AppName:     app.Name,
		AppVersion:  version,
	}
}

func (config ChartConfig) render() ([]byte, error) {
	buf := bytes.Buffer{}
	t := template.Must(template.New("chart.yaml").Parse(chartYaml))
	err := t.Execute(&buf, config)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ExportToDirectory saves the chart to the provided directory inside a folder with app_Name_TIMESTAMP
//  for example, for any app with name `hello`, it will save chart inside a folder with name `hello_11_Dec_20_12_30_IST`
func (chrt ApplicationChart) ExportToDirectory(directory string, chartConfig ChartConfig) error {
	timestamp := time.Now().Format(time.RFC822)
	replacer := strings.NewReplacer(" ", "_", ":", "_")
	chartDir := chartConfig.AppName + "_" + replacer.Replace(timestamp)
	targetDir := filepath.Join(directory, chartDir)

	err := os.MkdirAll(filepath.Join(targetDir, "templates"), os.ModePerm)
	if err != nil {
		return err
	}
	for filename, content := range chrt.templates {
		path := filepath.Join(targetDir, "templates", filename)
		err = ioutil.WriteFile(path, []byte(content), 0644)
		if err != nil {
			return err
		}
	}
	valuesBytes, err := yaml.Marshal(chrt.values)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath.Join(targetDir, "values.yaml"), valuesBytes, 0644)
	if err != nil {
		return err
	}
	chartYamlContent, err := chartConfig.render()
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath.Join(targetDir, "Chart.yaml"), chartYamlContent, 0644)
	if err != nil {
		return err
	}
	return nil
}

// GetName returns the app name, satisfying TemplateValuer
func (a ApplicationChart) GetName() string {
	return a.values.App.Name
}

// GetTemplates returns the app templates, satisfying TemplateValuer
func (a ApplicationChart) GetTemplates() map[string]string {
	return a.templates
}

// GetValues returns the app values, satisfying TemplateValuer
func (a ApplicationChart) GetValues() interface{} {
	return a.values
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

func newIngress(app ketchv1.App, framework ketchv1.Framework) (*ingress, error) {
	var http []string
	var https []httpsEndpoint

	for _, cname := range app.Spec.Ingress.Cnames {
		if cname.Secure {
			secretName := app.Spec.SecretName
			clusterIssuer := fmt.Sprintf("%s-clusterissuer", app.Spec.SecretName)

			if secretName == "" {
				if len(framework.Spec.IngressController.ClusterIssuer) == 0 {
					return nil, errors.New("secure cnames require a framework.Ingress.ClusterIssuer to be specified")
				}
				clusterIssuer = framework.Spec.IngressController.ClusterIssuer
				secretName = generateSecret(app.Name, cname.Name)
			}
			https = append(https, httpsEndpoint{Cname: cname.Name, SecretName: secretName, ClusterIssuer: clusterIssuer})
		} else {
			http = append(http, cname.Name)
		}
	}

	defaultCname := app.DefaultCname(&framework)
	if defaultCname != nil {
		http = append(http, *defaultCname)
	}
	return &ingress{
		Http:  http,
		Https: https,
	}, nil
}

func generateSecret(appName, cname string) string {
	hash := sha256.New()
	hash.Write([]byte(fmt.Sprintf("cname-%s", cname)))
	bs := hash.Sum(nil)
	return fmt.Sprintf("%s-cname-%x", appName, bs[:10])
}
