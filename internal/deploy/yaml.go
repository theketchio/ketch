package deploy

import (
	"fmt"
	"os"
	"strings"

	"sigs.k8s.io/yaml"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/errors"
	"github.com/shipa-corp/ketch/internal/utils"
	"github.com/shipa-corp/ketch/internal/utils/conversions"
)

// Application represents the fields in an application.yaml file that will be
// transitioned to a ChangeSet.
type Application struct {
	Version        *string   `json:"version,omitempty"`
	Type           *string   `json:"type"`
	Name           *string   `json:"name"`
	Image          *string   `json:"image,omitempty"`
	Framework      *string   `json:"framework"`
	Description    *string   `json:"description,omitempty"`
	Environment    []string  `json:"environment,omitempty"`
	RegistrySecret *string   `json:"registrySecret,omitempty"`
	Builder        *string   `json:"builder,omitempty"`
	BuildPacks     []string  `json:"buildPacks,omitempty"`
	Processes      []Process `json:"processes,omitempty"`
	CName          *CName    `json:"cname,omitempty"`
	AppUnit        *int      `json:"appUnit,omitempty"`
}

type Process struct {
	Name  string `json:"name"`  // required
	Cmd   string `json:"cmd"`   // required
	Units *int   `json:"units"` // unset? get from AppUnit
	Ports []Port `json:"ports"` // appDeploymentSpec
	Hooks Hooks  `json:"hooks"`
}

type Port struct {
	Protocol   string `json:"protocol"`
	Port       int    `json:"port"`
	TargetPort int    `json:"targetPort"`
}

type Hooks struct {
	Restart Restart `json:"restart"`
}

type Restart struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

type CName struct {
	DNSName string `json:"dnsName"`
}

const (
	defaultVersion  = "v1"
	defaultAppUnit  = 1
	typeApplication = "Application"
	typeJob         = "Job"
)

// GetChangeSetFromYaml reads an application.yaml file and returns a ChangeSet
// from the file's values.
func (o *Options) GetChangeSetFromYaml(filename string) (*ChangeSet, error) {
	var application Application
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(b, &application)
	if err != nil {
		return nil, err
	}

	var envs []ketchv1.Env
	if application.Environment != nil {
		envs, err = utils.MakeEnvironments(application.Environment)
		if err != nil {
			return nil, err
		}
	}
	// processes, hooks, ports
	var processes []ketchv1.ProcessSpec
	var ketchYamlData ketchv1.KetchYamlData
	if application.Processes != nil {
		var beforeHooks []string
		var afterHooks []string
		ketchYamlProcessConfig := make(map[string]ketchv1.KetchYamlProcessConfig)
		for _, process := range application.Processes {
			processes = append(processes, ketchv1.ProcessSpec{
				Name:  process.Name,
				Cmd:   strings.Split(process.Cmd, " "),
				Units: process.Units,
				Env:   envs,
			})
			if process.Hooks.Restart.Before != "" {
				beforeHooks = append(beforeHooks, process.Hooks.Restart.Before)
			}
			if process.Hooks.Restart.After != "" {
				afterHooks = append(afterHooks, process.Hooks.Restart.After)
			}

			var ports []ketchv1.KetchYamlProcessPortConfig
			for _, port := range process.Ports {
				ports = append(ports, ketchv1.KetchYamlProcessPortConfig{
					Protocol:   port.Protocol,
					Port:       port.Port,
					TargetPort: port.TargetPort,
				})
			}
			if len(process.Ports) > 0 {
				ketchYamlProcessConfig[process.Name] = ketchv1.KetchYamlProcessConfig{
					Ports: ports,
				}
			}
		}

		// assign hooks and ports (kubernetes processConfig) to ketch yaml data
		// NOTE: there is a disparity in that the yaml file format implies that hooks and ports
		// are per-process, while the AppSpec makes them per-deployment.
		ketchYamlData = ketchv1.KetchYamlData{
			Hooks: &ketchv1.KetchYamlHooks{
				Restart: ketchv1.KetchYamlRestartHooks{
					Before: beforeHooks,
					After:  afterHooks,
				},
			},
			Kubernetes: &ketchv1.KetchYamlKubernetesConfig{
				Processes: ketchYamlProcessConfig,
			},
		}
	}
	c := &ChangeSet{
		appName:              *application.Name,
		appVersion:           application.Version,
		appType:              application.Type,
		image:                application.Image,
		description:          application.Description,
		framework:            application.Framework,
		dockerRegistrySecret: application.RegistrySecret,
		builder:              application.Builder,
		appUnit:              application.AppUnit,
		timeout:              &o.Timeout,
		wait:                 &o.Wait,
	}
	if o.AppSourcePath != "" {
		c.sourcePath = &o.AppSourcePath
	}
	if application.CName != nil {
		c.cname = &ketchv1.CnameList{application.CName.DNSName}
	}
	if application.Environment != nil {
		c.envs = &application.Environment
	}
	if application.BuildPacks != nil {
		c.buildPacks = &application.BuildPacks
	}
	if len(processes) > 0 {
		c.processes = &processes
		c.ketchYamlData = &ketchYamlData
	}
	c.applyDefaults()
	return c, c.validate()
}

// apply defaults sets default values for a ChangeSet
func (c *ChangeSet) applyDefaults() {
	if c.appVersion == nil {
		c.appVersion = conversions.StrPtr(defaultVersion)
	}
	if c.appType == nil {
		c.appType = conversions.StrPtr(typeApplication)
	}
	c.yamlStrictDecoding = true

	// default to AppUnits if process.Units is unset
	if c.appUnit == nil {
		c.appUnit = conversions.IntPtr(defaultAppUnit)
	}
	if c.processes != nil {
		for i := range *c.processes {
			if (*c.processes)[i].Units == nil {
				(*c.processes)[i].Units = c.appUnit
			}
		}
	}
}

// validate assures that a ChangeSet's required fields are set
func (c *ChangeSet) validate() error {
	if c.framework == nil {
		return errors.New("missing required field framework")
	}
	if c.image == nil {
		return errors.New("missing required field image")
	}
	if c.appName == "" {
		return errors.New("missing required field name")
	}
	if c.sourcePath == nil && c.processes != nil {
		return errors.New("running defined processes require a sourcePath")
	}
	return nil
}

// GetApplicationFromKetchApp takes an App parameter and returns a yaml-file friendly Application
func GetApplicationFromKetchApp(app ketchv1.App) *Application {
	application := &Application{
		Version:   app.Spec.Version,
		Type:      conversions.StrPtr(typeApplication),
		Name:      &app.Name,
		Framework: &app.Spec.Framework,
	}

	deployment := getLatestDeployment(app.Spec.Deployments)
	if deployment != nil {
		application.Image = &deployment.Image
		var processes []Process
		if deployment.KetchYaml != nil {
			var hooks Hooks
			if deployment.KetchYaml.Hooks != nil {
				hooks = Hooks{
					Restart: Restart{
						Before: strings.Join(deployment.KetchYaml.Hooks.Restart.Before, " "),
						After:  strings.Join(deployment.KetchYaml.Hooks.Restart.After, " "),
					},
				}
			}

			if deployment.KetchYaml.Kubernetes != nil && deployment.KetchYaml.Kubernetes.Processes != nil {
				for _, process := range deployment.Processes {
					var ports []Port
					if processConfig, ok := deployment.KetchYaml.Kubernetes.Processes[process.Name]; ok {
						for _, port := range processConfig.Ports {
							ports = append(ports, Port{
								Protocol:   port.Protocol,
								Port:       port.Port,
								TargetPort: port.TargetPort,
							})
						}
					}
					processes = append(processes, Process{
						Name:  process.Name,
						Cmd:   strings.Join(process.Cmd, " "),
						Units: process.Units,
						Ports: ports,
						Hooks: hooks,
					})
				}
				application.Processes = processes
			}
		}
	}

	if len(app.Spec.Ingress.Cnames) > 0 {
		application.CName = &CName{
			DNSName: app.Spec.Ingress.Cnames[0],
		}
	}
	if app.Spec.Description != "" {
		application.Description = &app.Spec.Description
	}
	if app.Spec.DockerRegistry.SecretName != "" {
		application.RegistrySecret = &app.Spec.DockerRegistry.SecretName
	}
	if app.Spec.Builder != "" {
		application.Builder = &app.Spec.Builder
	}
	if len(app.Spec.BuildPacks) > 0 {
		application.BuildPacks = app.Spec.BuildPacks
	}
	var environment []string
	for _, env := range app.Spec.Env {
		environment = append(environment, fmt.Sprintf("%s=%s", env.Name, env.Value))
		application.Environment = environment
	}

	return application
}

// getLatestDeployment returns the AppDeploymentSpec of the highest Version or nil
func getLatestDeployment(deployments []ketchv1.AppDeploymentSpec) *ketchv1.AppDeploymentSpec {
	if len(deployments) == 0 {
		return nil
	}
	latestIndex := 0
	for i, deployment := range deployments {
		if deployment.Version > deployments[latestIndex].Version {
			deployments[latestIndex].Version = deployment.Version
			latestIndex = i
		}
	}
	return &deployments[latestIndex]
}
