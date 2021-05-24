package deploy

import (
	"io"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

const (
	appDeployHelp = `
Roll out a new version of an application with an image.

Deploy from source code. <source> is path to source code. The image in this case is required
and will be built using the selected source code and platform and will be used to deploy the app.
  ketch app deploy <app name> <source> -i myregistry/myimage:latest

  Ketch looks for Procfile and ketch.yaml inside the source directory by default
  but you can provide a custom path with --procfile or --ketch-yaml.

Deploy from an image:
  ketch app deploy <app name> -i myregistry/myimage:latest

  Ketch uses the image's cmd and entrypoint but you can redefine what exactly to run with --procfile.

`
)

type Params struct {
	// Client gets updates and creates ketch CRDs
	Client getterCreator
	// Kubernetes client
	KubeClient kubernetes.Interface
	// Builder references source builder from internal/builder package
	Builder SourceBuilderFn
	// Function that retrieve image config
	GetImageConfig GetImageConfigFn
	// Wait is a function that will wait until it detects the a deployment is finished
	Wait WaitFn
	// Writer probably points to stdout or stderr, receives textual output
	Writer io.Writer
}

// NewCommand creates a command that will run the app deploy
func NewCommand(params *Params) *cobra.Command {
	var options Options

	cmd := &cobra.Command{
		Use:   "deploy APPNAME [SOURCE DIRECTORY]",
		Short: "Deploy an app.",
		Long:  appDeployHelp,
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.AppName = args[0]
			if len(args) == 2 {
				options.AppSourcePath = args[1]
			}
			return newRunner(options.GetChangeSet(cmd.Flags())).run(cmd.Context(), params)
		},
	}

	cmd.Flags().StringVarP(&options.Image, flagImage, flagImageShort, "", "Name of the image to be deployed.")
	cmd.Flags().StringVar(&options.KetchYamlFileName, flagKetchYaml, "", "Path to ketch.yaml.")

	cmd.Flags().StringVar(&options.ProcfileFileName, flagProcFile, "", "Path to procfile.")
	cmd.Flags().BoolVar(&options.StrictKetchYamlDecoding, flagStrict, false, "Enforces strict decoding of ketch.yaml.")
	cmd.Flags().IntVar(&options.Steps, flagSteps, 2, "Number of steps for a canary deployment.")
	cmd.Flags().StringVar(&options.StepTimeInterval, flagStepInterval, "", "Time interval between canary deployment steps. Supported min: m, hour:h, second:s. ex. 1m, 60s, 1h.")
	cmd.Flags().BoolVar(&options.Wait, flagWait, false, "If true blocks until deploy completes or a timeout occurs.")
	cmd.Flags().StringVar(&options.Timeout, flagTimeout, "20s", "Defines the length of time to block waiting for deployment completion. Supported min: m, hour:h, second:s. ex. 1m, 60s, 1h.")
	cmd.Flags().StringSliceVar(&options.SubPaths, flagIncludeDirs, []string{"."}, "Optionally include additional source paths. Additional paths must be relative to source-path.")

	cmd.Flags().StringVarP(&options.Platform, flagPlatform, flagPlatformShort, "", "Platform name.")
	cmd.Flags().StringVarP(&options.Description, flagDescription, flagDescriptionShort, "", "App description.")
	cmd.Flags().StringSliceVarP(&options.Envs, flagEnvironment, flagEnvironmentShort, []string{}, "App env variables.")
	cmd.Flags().StringVarP(&options.Framework, flagFramework, flagFrameworkShort, "", "Framework to deploy your app.")
	cmd.Flags().StringVarP(&options.DockerRegistrySecret, flagRegistrySecret, "", "", "A name of a Secret with docker credentials. This secret must be created in the same namespace of the framework.")

	return cmd
}
