package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	registryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
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
			return appDeploy(cmd.Context(), cfg, getImageConfigFile, options, out)
		},
	}

	cmd.Flags().StringVarP(&options.image, "image", "i", "", "the image with the application")
	cmd.Flags().StringVar(&options.procfileFileName, "procfile", "", "the path to Procfile. If not set, ketch will use entrypoint and cmd from the image")
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

type getImageConfigFileFn func(ctx context.Context, kubeClient kubernetes.Interface, args getImageConfigArgs, fn getRemoteImageFn) (*registryv1.ConfigFile, error)

func appDeploy(ctx context.Context, cfg config, getImageConfigFile getImageConfigFileFn, options appDeployOptions, out io.Writer) error {
	app := ketchv1.App{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.appName}, &app); err != nil {
		return fmt.Errorf("failed to get app instance: %w", err)
	}
	pool := ketchv1.Pool{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: app.Spec.Pool}, &pool); err != nil {
		return fmt.Errorf("failed to get pool instance: %w", err)
	}

	args := getImageConfigArgs{
		imageName:       options.image,
		secretName:      app.Spec.DockerRegistry.SecretName,
		secretNamespace: pool.Spec.NamespaceName,
	}
	// we get the image's config file because of several reasons:
	// 1. to support images with exposed ports defined in Dockerfile with EXPOSE directive.
	// 2. to support deployments without Procfile.
	configFile, err := getImageConfigFile(ctx, cfg.KubernetesClient(), args, remote.Image)
	if err != nil {
		return fmt.Errorf("can't use the image: %w", err)
	}

	var procfile *chart.Procfile
	if options.IsProcfileSet() {
		procfile, err = options.Procfile()
		if err != nil {
			return fmt.Errorf("failed to read Procfile: %w", err)
		}
	} else {
		procfile, err = createProcfile(*configFile)
		if err != nil {
			return fmt.Errorf("can't use the image: %w", err)
		}
	}
	ketchYaml, err := options.KetchYaml()
	if err != nil {
		return fmt.Errorf("failed to read ketch.yaml: %w", err)
	}
	processes := make([]ketchv1.ProcessSpec, 0, len(procfile.Processes))
	for _, processName := range procfile.SortedNames() {
		cmd := procfile.Processes[processName]
		processes = append(processes, ketchv1.ProcessSpec{
			Name: processName,
			Cmd:  cmd,
		})
	}
	exposedPorts := make([]ketchv1.ExposedPort, 0, len(configFile.Config.ExposedPorts))
	for port := range configFile.Config.ExposedPorts {
		exposedPort, err := ketchv1.NewExposedPort(port)
		if err != nil {
			// Shouldn't happen
			return err
		}
		exposedPorts = append(exposedPorts, *exposedPort)
	}
	deploymentSpec := ketchv1.AppDeploymentSpec{
		Image:     options.image,
		Version:   ketchv1.DeploymentVersion(app.Spec.DeploymentsCount + 1),
		Processes: processes,
		KetchYaml: ketchYaml,
		RoutingSettings: ketchv1.RoutingSettings{
			Weight: 100,
		},
		ExposedPorts: exposedPorts,
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

func (opts appDeployOptions) IsProcfileSet() bool {
	return len(opts.procfileFileName) > 0
}

type getRemoteImageFn func(ref name.Reference, options ...remote.Option) (registryv1.Image, error)

type getImageConfigArgs struct {
	imageName       string
	secretName      string
	secretNamespace string
}

func getImageConfigFile(ctx context.Context, kubeClient kubernetes.Interface, args getImageConfigArgs, fn getRemoteImageFn) (*registryv1.ConfigFile, error) {
	ref, err := name.ParseReference(args.imageName)
	if err != nil {
		return nil, err
	}
	var options []remote.Option
	if len(args.secretName) > 0 {
		keychainOpts := k8schain.Options{
			Namespace:        args.secretNamespace,
			ImagePullSecrets: []string{args.secretName},
		}
		keychain, err := k8schain.New(ctx, kubeClient, keychainOpts)
		if err != nil {
			return nil, err
		}
		options = append(options, remote.WithAuthFromKeychain(keychain))
	}
	img, err := fn(ref, options...)
	if err != nil {
		return nil, err
	}
	return img.ConfigFile()
}

func createProcfile(configFile registryv1.ConfigFile) (*chart.Procfile, error) {
	cmds := append(configFile.Config.Entrypoint, configFile.Config.Cmd...)
	if len(cmds) == 0 {
		return nil, ErrNoEntrypointAndCmd
	}
	return &chart.Procfile{
		Processes: map[string][]string{
			chart.DefaultRoutableProcessName: cmds,
		},
		RoutableProcessName: chart.DefaultRoutableProcessName,
	}, nil
}
