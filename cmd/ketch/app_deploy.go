package main

import (
	"github.com/spf13/cobra"

	"github.com/theketchio/ketch/internal/deploy"
	"github.com/theketchio/ketch/internal/validation"
)

const (
	appDeployHelp = `
Roll out a new version of an application with an image.

Deploy from source code. <source> is path to source code. The image in this case is required
and will be built using the selected source code and builder and will be used to deploy the app.

Similarly, the source path's root directory must contain Procfile as specified by pack.
Details about Procfile conventions can be found here: https://devcenter.heroku.com/articles/procfile
  ketch app deploy <app name> <source> -i myregistry/myimage:latest

  Ketch looks for ketch.yaml inside the source directory by default
  but you can provide a custom path with --ketch-yaml.

Deploy from an image:
  ketch app deploy <app name> -i myregistry/myimage:latest

Users can deploy from image or source code by passing a filename such as app.yaml containing fields like:
	name: test
	image: gcr.io/shipa-ci/sample-go-app:latest
	namespace: mynamespace
`
)

// NewCommand creates a command that will run the app deploy
func newAppDeployCmd(cfg config, params *deploy.Services, configDefaultBuilder string) *cobra.Command {
	var options deploy.Options

	cmd := &cobra.Command{
		Use:   "deploy [APPNAME|FILENAME] [SOURCE DIRECTORY]",
		Short: "Deploy an app.",
		Long:  appDeployHelp,
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.AppName = args[0]
			if len(args) == 2 {
				options.AppSourcePath = args[1]
			}
			if configDefaultBuilder != "" {
				deploy.DefaultBuilder = configDefaultBuilder
			}
			return appDeploy(cmd, options, params)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return autoCompleteAppNames(cfg, toComplete)
		},
	}

	cmd.Flags().StringVarP(&options.Image, deploy.FlagImage, deploy.FlagImageShort, "", "Name of the image to be deployed.")
	cmd.Flags().StringVar(&options.KetchYamlFileName, deploy.FlagKetchYaml, "", "Path to ketch.yaml.")

	cmd.Flags().BoolVar(&options.StrictKetchYamlDecoding, deploy.FlagStrict, false, "Enforces strict decoding of ketch.yaml.")
	cmd.Flags().IntVar(&options.Steps, deploy.FlagSteps, 0, "Number of steps for a canary deployment.")
	cmd.Flags().StringVar(&options.StepTimeInterval, deploy.FlagStepInterval, "", "Time interval between canary deployment steps. Supported min: m, hour:h, second:s. ex. 1m, 60s, 1h.")
	cmd.Flags().BoolVar(&options.Wait, deploy.FlagWait, false, "If true blocks until deploy completes or a timeout occurs.")
	cmd.Flags().StringVar(&options.Timeout, deploy.FlagTimeout, "20s", "Defines the length of time to block waiting for deployment completion. Supported min: m, hour:h, second:s. ex. 1m, 60s, 1h.")

	cmd.Flags().StringVarP(&options.Description, deploy.FlagDescription, deploy.FlagDescriptionShort, "", "App description.")
	cmd.Flags().StringSliceVarP(&options.Envs, deploy.FlagEnvironment, deploy.FlagEnvironmentShort, []string{}, "App env variables.")
	cmd.Flags().StringVarP(&options.Namespace, deploy.FlagNamespace, deploy.FlagNamespaceShort, "", "Namespace to deploy your app.")
	cmd.Flags().StringVarP(&options.DockerRegistrySecret, deploy.FlagRegistrySecret, "", "", "A name of a Secret with docker credentials. This secret must be created in the same namespace.")
	cmd.Flags().StringVar(&options.Builder, deploy.FlagBuilder, "", "Builder to use when building from source.")
	cmd.Flags().StringSliceVar(&options.BuildPacks, deploy.FlagBuildPacks, nil, "A list of build packs.")
	cmd.Flags().StringVar(&options.Volume, "volume", "", "Name of the volume to bind to the application.")
	cmd.Flags().StringVar(&options.VolumeMountPath, "volume-mount-path", "", "Path to mount a volume.")
	cmd.Flags().StringToStringVar(&options.VolumeMountOptions, "volume-mount-options", nil, "Options for volume mount.")
	cmd.Flags().Int64Var(&options.FSGroup, "fs-group", 0, "The fsGroup for pod's security context; root if not set.")
	cmd.Flags().Int64Var(&options.RunAsUser, "run-as-user", 0, "The user to use for running pod's processes; root if not set.")

	cmd.Flags().IntVar(&options.Units, deploy.FlagUnits, 1, "Set number of units for deployment.")
	cmd.Flags().IntVar(&options.Version, deploy.FlagVersion, 1, "Specify version whose units to update. Must be used with units flag!")
	cmd.Flags().StringVar(&options.Process, deploy.FlagProcess, "", "Specify process whose units to update. Must be used with units flag!")

	cmd.RegisterFlagCompletionFunc(deploy.FlagNamespace, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return autoCompleteNamespaces(cfg, toComplete)
	})

	cmd.RegisterFlagCompletionFunc(deploy.FlagBuilder, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return autoCompleteBuilderNames(cfg, toComplete)
	})
	return cmd
}

func appDeploy(cmd *cobra.Command, options deploy.Options, params *deploy.Services) error {
	var changeSet *deploy.ChangeSet
	var err error
	switch {
	case validation.ValidateYamlFilename(options.AppName):
		changeSet, err = options.GetChangeSetFromYaml(options.AppName)
		if err != nil {
			return err
		}
	default:
		changeSet = options.GetChangeSet(cmd.Flags())
	}
	return deploy.New(changeSet).Run(cmd.Context(), params)
}
