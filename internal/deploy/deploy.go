// Package deploy is purposed to deploy an app.  This concern encompasses creating the app CRD if it doesn't exist,
// possibly creating the app image from source code, and then creating a deployment that will the image in a k8s cluster.
package deploy

import (
	"context"
	"fmt"
	"os"
	"path"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Options contains the parameters needed to run the deployment.
type Options struct {
	AppName                 string
	Image                   string
	KetchYamlFileName       string
	ProcfileFileName        string
	StrictKetchYamlDecoding bool
	Steps                   int
	StepWeight              uint8
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
type Runner struct{}

// New creates a Runner which will execute the deployment.
func New(client clusterClient, opts Options) (*Runner, error) {
	var r Runner

	return &r, nil
}

// Run executes the deployment. This includes creating the application CRD if it doesn't already exist, possibly building
// source code and creating an image and creating and applying a deployment CRD to the cluster.
func (r Runner) Run(ctx context.Context) error {
	return nil
}

func Validate(opts Options) error {
	if opts.AppName == "" {
		panic("app name should be present")
	}

	if err := validatePaths(opts.AppSourcePath, opts.SubPaths); err != nil {
		return err
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
    	return  nil
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
