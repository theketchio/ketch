// Package deploy is purposed to deploy an app.  This concern encompasses creating the app CRD if it doesn't exist,
// possibly creating the app image from source code, and then creating a deployment that will the image in a k8s cluster.
package deploy

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultTrafficWeight = 100
	minimumSteps         = 2
	maximumSteps         = 100
)

// Options contains the parameters needed to run the deployment.
type Options struct {
	AppName                 string
	Image                   string
	KetchYamlFileName       string
	ProcfileFileName        string
	StrictKetchYamlDecoding bool
	Steps                   int
	StepTimeInterval        string
	Wait                    bool
	Timeout                 string
	AppSourcePath           string
	SubPaths                []string

	Pool                 string
	Description          string
	Envs                 []string
	DockerRegistrySecret string
	// this goes bye bye
	Platform string
}

type clusterClient interface {
	Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error
	Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error
}

// Runner is concerned with managing and running the deployment.
type Runner struct {
	stepWeight uint8
}

// New creates a Runner which will execute the deployment.
func New(client clusterClient, opts Options) (*Runner, error) {
	if err := validate(opts); err != nil {
		return nil, err
	}

	var r Runner
	if opts.Steps != 0 {
		r.stepWeight = uint8(defaultTrafficWeight / opts.Steps)
	}

	return &r, nil
}

// Run executes the deployment. This includes creating the application CRD if it doesn't already exist, possibly building
// source code and creating an image and creating and applying a deployment CRD to the cluster.
func (r Runner) Run(ctx context.Context) error {
	return nil
}

func validate(opts Options) error {
	if opts.AppName == "" {
		panic("app name should be present")
	}
	if opts.Image == "" {
		panic("image name should be present")
	}

	if err := validateCanary(opts.Steps, opts.StepTimeInterval, opts.Timeout); err != nil {
		return err
	}
	if err := validatePaths(opts.AppSourcePath, opts.SubPaths); err != nil {
		return err
	}

	return nil
}

func validateCanary(steps int, stepTimeInterval, timeout string) error {
	if steps == 0 {
		log.Printf("canary deployment not enabled")
		return nil
	}

	if steps < minimumSteps || steps > maximumSteps {
		return NewError("steps must be within the range %d to %d", minimumSteps, maximumSteps)
	}
	if defaultTrafficWeight%steps != 0 {
		return NewError("the number of steps must be an even divisor of 100")
	}

	if stepTimeInterval == "" {
		return NewError("step interval is not set")
	}

	if _, err := time.ParseDuration(stepTimeInterval); err != nil {
		return NewError("invalid step interval: %s", err)
	}

	if timeout != "" {
		if _, err := time.ParseDuration(timeout); err != nil {
			return NewError("invalid timeout: %s", err)
		}
	}

	return nil
}

func validatePaths(root string, subpaths []string) error {
	check := func(path string) error {
		fi, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			return fmt.Errorf("%q is not a directory", path)
		}
		return nil
	}
	if root == "" {
		return nil
	}
	if err := check(root); err != nil {
		return err
	}

	for _, subpath := range subpaths {
		if err := check(path.Join(root, subpath)); err != nil {
			return err
		}
	}

	return nil
}
