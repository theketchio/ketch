package deploy

import (
	"fmt"
	"os"

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
}

type Process struct {
	Name  string `json:"name"`  // required
	Units *int   `json:"units"` // default 1
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
	Secure  bool   `json:"secure"`
}

const (
	defaultVersion  = "v1"
	defaultAppUnit  = 1
	typeApplication = "Application"
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
	// processes
	var processes []ketchv1.ProcessSpec
	if application.Processes != nil {
		for _, process := range application.Processes {
			processes = append(processes, ketchv1.ProcessSpec{
				Name:  process.Name,
				Units: process.Units,
				Env:   envs,
			})
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
		timeout:              &o.Timeout,
		wait:                 &o.Wait,
	}
	if o.AppSourcePath != "" {
		c.sourcePath = &o.AppSourcePath
	}
	if application.CName != nil {
		c.cname = &ketchv1.CnameList{{Name: application.CName.DNSName, Secure: application.CName.Secure}}
	}
	if application.Environment != nil {
		c.envs = &application.Environment
	}
	if application.BuildPacks != nil {
		c.buildPacks = &application.BuildPacks
	}
	if len(processes) > 0 {
		c.processes = &processes
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

	if c.processes != nil {
		for i := range *c.processes {
			if (*c.processes)[i].Units == nil {
				(*c.processes)[i].Units = conversions.IntPtr(defaultAppUnit)
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
		application.Image = conversions.StrPtr(deployment.Image)
		for _, process := range deployment.Processes {
			application.Processes = append(application.Processes, Process{
				Name:  process.Name,
				Units: process.Units,
			})
		}
	}

	if len(app.Spec.Ingress.Cnames) > 0 {
		application.CName = &CName{
			DNSName: app.Spec.Ingress.Cnames[0].Name,
			Secure:  app.Spec.Ingress.Cnames[0].Secure,
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
