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
	Client     getterCreator
	KubeClient kubernetes.Interface

	Builder     SourceBuilderFn
	RemoteImage RemoteImageFn
	Wait        WaitFn

	Writer io.Writer
}

// NewCommand creates a command that will run the app deploy
func NewCommand(params *Params) *cobra.Command {
	var options Options

	cmd := &cobra.Command{
		Use:   "deploy APPNAME [SOURCE DIRECTORY]",
		Short: "Deploy an app",
		Long:  appDeployHelp,
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.AppName = args[0]
			if len(args) == 2 {
				options.AppSourcePath = args[1]
			}
			return New(options.GetChangeSet(cmd.Flags())).Run(cmd.Context(), params)
		},
	}

	cmd.Flags().StringVarP(&options.Image, FlagImage, FlagImageShort, "", "the image that will be deployed")
	cmd.Flags().StringVar(&options.KetchYamlFileName, FlagKetchYaml, "", "the path to ketch.yaml")

	cmd.Flags().StringVar(&options.ProcfileFileName, FlagProcFile, "", "the path to Procfile")
	cmd.Flags().BoolVar(&options.StrictKetchYamlDecoding, FlagStrict, false, "strict decoding of ketch.yaml")
	cmd.Flags().IntVar(&options.Steps, FlagSteps, 2, "number of steps to roll out the new deployment")
	cmd.Flags().StringVar(&options.StepTimeInterval, FlagStepInterval, "", "time interval between each step. Supported min: m, hour:h, second:s. ex. 1m, 60s, 1h")
	cmd.Flags().BoolVar(&options.Wait, FlagWait, false, "await for reconcile event")
	cmd.Flags().StringVar(&options.Timeout, FlagTimeout, "20s", "timeout for await of reconcile. Supported min: m, hour:h, second:s. ex. 1m, 60s, 1h")
	cmd.Flags().StringSliceVar(&options.SubPaths, FlagIncludeDirs, []string{"."}, "optionally include additional source paths. Additional paths must be relative to source-path")

	cmd.Flags().StringVarP(&options.Platform, FlagPlatform, FlagPlatformShort, "", "Platform name")
	cmd.Flags().StringVarP(&options.Description, FlagDescription, FlagDescriptionShort, "", "App description")
	cmd.Flags().StringSliceVarP(&options.Envs, FlagEnvironment, FlagEnvironmentShort, []string{}, "App env variables")
	cmd.Flags().StringVarP(&options.Pool, FlagPool, FlagPoolShort, "", "Pool to deploy your app")
	cmd.Flags().StringVarP(&options.DockerRegistrySecret, FlagRegistrySecret, "", "", "A name of a Secret with docker credentials. This secret must be created in the same namespace of the pool.")

	return cmd
}
