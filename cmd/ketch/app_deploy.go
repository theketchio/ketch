package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/chart"
)

const appDeployHelp = `
Roll out a new version of an application with an image. 
`

func newAppDeployCmd(cfg config, out io.Writer) *cobra.Command {
	options := appDeployOptions{}
	cmd := &cobra.Command{
		Use:   "deploy APPNAME",
		Short: "Deploy an app",
		Long:  appDeployHelp,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.appName = args[0]
			return appDeploy(cmd.Context(), cfg, options, out)
		},
	}

	cmd.Flags().StringVarP(&options.image, "image", "i", "", "the image with the application")
	cmd.Flags().StringVar(&options.procfileFileName, "procfile", "", "the path to Procfile. If not set, ketch will use Procfile in the current directory")
	cmd.Flags().StringVar(&options.ketchYamlFileName, "ketch-yaml", "", "the path to ketch.yaml")
	cmd.Flags().BoolVar(&options.strictKetchYamlDecoding, "strict", false, "strict decoding of ketch.yaml")
	cmd.MarkFlagRequired("image")

	return cmd
}

type appDeployOptions struct {
	appName                 string
	image                   string
	ketchYamlFileName       string
	procfileFileName        string
	strictKetchYamlDecoding bool
}

func appDeploy(ctx context.Context, cfg config, options appDeployOptions, out io.Writer) error {
	procfile, err := options.Procfile()
	if err != nil {
		return fmt.Errorf("failed to read Procfile: %w", err)
	}
	ketchYaml, err := options.KetchYaml()
	if err != nil {
		return fmt.Errorf("failed to read ketch.yaml: %w", err)
	}
	app := ketchv1.App{}
	err = cfg.Client().Get(ctx, types.NamespacedName{Name: options.appName}, &app)
	if err != nil {
		return fmt.Errorf("failed to get app instance: %w", err)
	}
	processes := make([]ketchv1.ProcessSpec, 0, len(procfile.Processes))
	for name, cmd := range procfile.Processes {
		processes = append(processes, ketchv1.ProcessSpec{
			Name: name,
			Cmd:  cmd,
		})
	}
	deploymentSpec := ketchv1.AppDeploymentSpec{
		Image:     options.image,
		Version:   app.Spec.DeploymentsCount + 1,
		Processes: processes,
		KetchYaml: ketchYaml,
		RoutingSettings: ketchv1.RoutingSettings{
			Weight: 100,
		},
	}
	app.Spec.Deployments = []ketchv1.AppDeploymentSpec{
		deploymentSpec,
	}
	app.Spec.DeploymentsCount += 1
	if err = cfg.Client().Update(ctx, &app); err != nil {
		return fmt.Errorf("failed to update the app: %v", err)
	}
	fmt.Fprintln(out, "Successfully deployed!")
	return nil
}

func (opts appDeployOptions) Procfile() (*chart.Procfile, error) {
	content, err := ioutil.ReadFile(opts.procfileFileName)
	if err != nil {
		return nil, err
	}
	return chart.ParseProcfile(string(content))
}

func (opts appDeployOptions) KetchYaml() (*ketchv1.KetchYamlData, error) {
	if len(opts.ketchYamlFileName) == 0 {
		return nil, nil
	}
	content, err := ioutil.ReadFile(opts.ketchYamlFileName)
	if err != nil {
		return nil, err
	}
	var decodeOpts []yaml.JSONOpt
	if opts.strictKetchYamlDecoding {
		decodeOpts = append(decodeOpts, yaml.DisallowUnknownFields)
	}
	data := &ketchv1.KetchYamlData{}
	if err = yaml.Unmarshal(content, data, decodeOpts...); err != nil {
		return nil, err
	}
	return data, nil
}
