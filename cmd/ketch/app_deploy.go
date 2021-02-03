package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	registryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/build"
	"github.com/shipa-corp/ketch/internal/chart"
	"github.com/shipa-corp/ketch/internal/controllers"
	"github.com/shipa-corp/ketch/internal/docker"
	"github.com/shipa-corp/ketch/internal/errors"
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
	// default weight used to route incoming traffic to a deployment
	defaultTrafficWeight = 100
)

func newAppDeployCmd(cfg config, out io.Writer) *cobra.Command {
	options := appDeployOptions{}
	cmd := &cobra.Command{
		Use:   "deploy APPNAME [SOURCE DIRECTORY]",
		Short: "Deploy an app",
		Long:  appDeployHelp,
		Args:  cobra.RangeArgs(1, 2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if options.image == "" {
				return errors.New("missing required image name")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			options.appName = args[0]

			var (
				err       error
				dockerSvc *docker.Client
			)

			if len(args) == 2 {
				dockerSvc, err = docker.New()
				if err != nil {
					return err
				}
				defer dockerSvc.Close()
				options.appSourcePath = args[1]
			}
			return appDeploy(cmd.Context(), cfg, getImageConfigFile, waitHandler(watchAppReconcileEvent), build.GetSourceHandler(dockerSvc), changeAppCRD, options, out)
		},
	}

	cmd.Flags().StringVarP(&options.image, "image", "i", "", "the image with the application")
	cmd.Flags().StringVar(&options.ketchYamlFileName, "ketch-yaml", "", "the path to ketch.yaml")
	cmd.Flags().StringVar(&options.procfileFileName, "procfile", "", "the path to Procfile")
	cmd.Flags().BoolVar(&options.strictKetchYamlDecoding, "strict", false, "strict decoding of ketch.yaml")
	cmd.Flags().IntVar(&options.steps, "steps", 1, "number of steps to roll out the new deployment")
	cmd.Flags().StringVar(&options.stepTimeInterval, "step-interval", "", "time interval between each step. Supported min: m, hour:h, second:s. ex. 1m, 60s, 1h")
	cmd.Flags().BoolVar(&options.wait, "wait", false, "await for reconcile event")
	cmd.Flags().StringVar(&options.timeout, "timeout", "20s", "timeout for await of reconcile. Supported min: m, hour:h, second:s. ex. 1m, 60s, 1h")
	cmd.Flags().StringSliceVar(&options.subPaths, "include-dirs", []string{"."}, "optionally include additional source paths. Additional paths must be relative to source-path")
	cmd.MarkFlagRequired("image")

	return cmd
}

type appDeployOptions struct {
	appName                 string
	image                   string
	ketchYamlFileName       string
	procfileFileName        string
	strictKetchYamlDecoding bool
	steps                   int
	stepWeight              uint8
	stepTimeInterval        string
	wait                    bool
	timeout                 string
	appSourcePath           string
	subPaths                []string
}

func (opts *appDeployOptions) validate() error {
	if opts.steps < 1 || opts.steps > 100 {
		return fmt.Errorf("steps must be within the range 1 to 100")
	}

	opts.stepWeight = uint8(defaultTrafficWeight / opts.steps)

	// normalize to reach 100% traffic
	if defaultTrafficWeight%opts.stepWeight != 0 {
		opts.stepWeight++
	}

	if opts.stepTimeInterval == "" && opts.steps > 1 {
		return fmt.Errorf("step interval is not set")
	}

	if len(opts.stepTimeInterval) > 0 {
		_, err := time.ParseDuration(opts.stepTimeInterval)
		if err != nil {
			return fmt.Errorf("invalid step interval: %w", err)
		}
	}

	_, err := time.ParseDuration(opts.timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}
	return nil
}

type getImageConfigFileFn func(ctx context.Context, kubeClient kubernetes.Interface, args getImageConfigArgs, fn getRemoteImageFn) (*registryv1.ConfigFile, error)
type waitFn func(ctx context.Context, cfg config, app ketchv1.App, timeout time.Duration, out io.Writer) error
type buildFromSourceFn func(context.Context, *build.CreateImageFromSourceRequest, ...build.Option) (*build.CreateImageFromSourceResponse, error)
type changeAppCRDFn func(app *ketchv1.App, args deploymentArguments) error

func appDeploy(ctx context.Context, cfg config, getImageConfigFile getImageConfigFileFn, wait waitFn, buildFromSource buildFromSourceFn, changeAppCRD changeAppCRDFn, options appDeployOptions, out io.Writer) error {
	app := ketchv1.App{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: options.appName}, &app); err != nil {
		return fmt.Errorf("failed to get app instance: %w", err)
	}
	pool := ketchv1.Pool{}
	if err := cfg.Client().Get(ctx, types.NamespacedName{Name: app.Spec.Pool}, &pool); err != nil {
		return fmt.Errorf("failed to get pool instance: %w", err)
	}

	if err := options.validate(); err != nil {
		return err
	}

	if options.isCanarySet() {
		// check for old deployments
		switch deps := len(app.Spec.Deployments); {
		case deps == 0:
			return fmt.Errorf("canary deployment failed. No primary deployment found for the app")
		case deps >= 2:
			return fmt.Errorf("canary deployment failed. Maximum number of two deployments are currently supported")
		}
	}

	// we need ketch.yaml before the build stage is started
	// because it can contain build hooks.
	ketchYaml, err := options.KetchYaml()
	if err != nil {
		return errors.Wrap(err, "ketch.yaml could not be processed")
	}

	// if we build an image from sources it can contain Procfile
	// but --procfile has higher priority.
	var procfile *chart.Procfile

	if options.appSourcePath != "" {

		if app.Spec.Platform == "" {
			return fmt.Errorf("can't build an application without platform")
		}

		var platform ketchv1.Platform
		if err := cfg.Client().Get(ctx, types.NamespacedName{Name: app.Spec.Platform}, &platform); err != nil {
			return fmt.Errorf("failed to get platform: %w", err)
		}

		resp, err := buildFromSource(
			ctx,
			&build.CreateImageFromSourceRequest{
				Image:         options.image,
				AppName:       options.appName,
				PlatformImage: platform.Spec.Image,
			},
			build.WithOutput(out),
			build.WithWorkingDirectory(options.appSourcePath),
			build.WithSourcePaths(options.subPaths...),
			build.MaybeWithBuildHooks(ketchYaml),
		)
		if err != nil {
			return err
		}
		procfile = resp.Procfile
	}

	args := getImageConfigArgs{
		imageName:       options.image,
		secretName:      app.Spec.DockerRegistry.SecretName,
		secretNamespace: pool.Spec.NamespaceName,
	}
	// we get the image's config file because of several reasons:
	// 1. to support images with exposed ports defined in Dockerfile with EXPOSE directive.
	// 2. to support deployments without Procfile, in this case we the image's cmd and entrypoint to define Procfile
	configFile, err := getImageConfigFile(ctx, cfg.KubernetesClient(), args, remote.Image)
	if err != nil {
		return fmt.Errorf("can't use the image: %w", err)
	}

	if len(options.procfileFileName) > 0 {
		procfile, err = options.Procfile()
		if err != nil {
			return fmt.Errorf("failed to read Procfile: %w", err)
		}
	}

	if procfile == nil {
		procfile, err = createProcfile(*configFile)
		if err != nil {
			return fmt.Errorf("can't use the image: %w", err)
		}
	}

	deployArgs := deploymentArguments{
		image:             options.image,
		steps:             options.steps,
		stepWeight:        options.stepWeight,
		procfile:          *procfile,
		ketchYaml:         ketchYaml,
		configFile:        configFile,
		nextScheduledTime: options.nextScheduledTime(),
		started:           time.Now(),
	}
	deployArgs.stepTimeInterval, _ = time.ParseDuration(options.stepTimeInterval)
	err = changeAppCRD(&app, deployArgs)
	if err != nil {
		return err
	}

	if err := cfg.Client().Update(ctx, &app); err != nil {
		return fmt.Errorf("failed to update the app: %v", err)
	}

	if options.wait {
		return wait(ctx, cfg, app, options.Timeout(), out)
	}

	fmt.Fprintln(out, fmt.Sprintf("App %s deployed successfully. Run `ketch app info %s` to check status of the deployment", app.Name, app.Name))
	return nil
}

type deploymentArguments struct {
	image             string
	steps             int
	stepWeight        uint8
	stepTimeInterval  time.Duration
	procfile          chart.Procfile
	ketchYaml         *ketchv1.KetchYamlData
	configFile        *registryv1.ConfigFile
	nextScheduledTime time.Time
	started           time.Time
}

func changeAppCRD(app *ketchv1.App, args deploymentArguments) error {
	processes := make([]ketchv1.ProcessSpec, 0, len(args.procfile.Processes))
	for _, processName := range args.procfile.SortedNames() {
		cmd := args.procfile.Processes[processName]
		processes = append(processes, ketchv1.ProcessSpec{
			Name: processName,
			Cmd:  cmd,
		})
	}
	exposedPorts := make([]ketchv1.ExposedPort, 0, len(args.configFile.Config.ExposedPorts))
	for port := range args.configFile.Config.ExposedPorts {
		exposedPort, err := ketchv1.NewExposedPort(port)
		if err != nil {
			// Shouldn't happen
			return err
		}
		exposedPorts = append(exposedPorts, *exposedPort)
	}

	// default deployment spec for an app
	deploymentSpec := ketchv1.AppDeploymentSpec{
		Image:     args.image,
		Version:   ketchv1.DeploymentVersion(app.Spec.DeploymentsCount + 1),
		Processes: processes,
		KetchYaml: args.ketchYaml,
		RoutingSettings: ketchv1.RoutingSettings{
			Weight: defaultTrafficWeight,
		},
		ExposedPorts: exposedPorts,
	}

	if args.steps > 1 {

		if len(app.Spec.Deployments) != 1 {
			// Shouldn't happen, it's here to avoid tests with incorrect data.
			return errors.New("canary deployment failed: the application has to contain one deployment")
		}

		nextScheduledTime := metav1.NewTime(args.nextScheduledTime)
		started := metav1.NewTime(args.started)
		app.Spec.Canary = ketchv1.CanarySpec{
			Steps:             args.steps,
			StepWeight:        args.stepWeight,
			StepTimeInteval:   args.stepTimeInterval,
			NextScheduledTime: &nextScheduledTime,
			CurrentStep:       1,
			Active:            true,
			Started:           &started,
		}

		// set initial weight for canary deployment to zero.
		// App controller will update the weight once all pods for canary will be on running state.
		deploymentSpec.RoutingSettings.Weight = 0

		// For a canary deployment, canary should be enabled by adding another deployment to the deployment list.
		app.Spec.Deployments = append(app.Spec.Deployments, deploymentSpec)
	} else {
		app.Spec.Deployments = []ketchv1.AppDeploymentSpec{deploymentSpec}
	}

	app.Spec.DeploymentsCount += 1
	return nil
}

type watchReconcileEventFn func(ctx context.Context, kubeClient kubernetes.Interface, app *ketchv1.App) (watch.Interface, error)

func waitHandler(watchReconcileEvent watchReconcileEventFn) waitFn {
	return func(ctx context.Context, cfg config, app ketchv1.App, timeout time.Duration, out io.Writer) error {
		watcher, err := watchReconcileEvent(ctx, cfg.KubernetesClient(), &app)
		if err != nil {
			return err
		}
		defer watcher.Stop()

		// Await for reconcile result
		maxExecTime := time.NewTimer(timeout)
		evtCh := watcher.ResultChan()
		for {
			select {
			case evt, ok := <-evtCh:
				if !ok {
					err := errors.New("events channel unexpectedly closed")
					return err
				}
				e, ok := evt.Object.(*corev1.Event)
				if ok {
					reason, err := controllers.ParseAppReconcileMessage(e.Reason)
					if err != nil {
						return err
					}
					if reason.DeploymentCount == app.Spec.DeploymentsCount {
						switch e.Type {
						case v1.EventTypeNormal:
							fmt.Fprintln(out, "successfully deployed!")
							return nil
						case v1.EventTypeWarning:
							return errors.New(e.Message)
						}
					}
				}
			case <-maxExecTime.C:
				err := fmt.Errorf("maximum execution time exceeded")
				return err
			}
		}
	}
}

func (opts appDeployOptions) Procfile() (*chart.Procfile, error) {
	content, err := ioutil.ReadFile(opts.procfileFileName)
	if err != nil {
		return nil, err
	}
	return chart.ParseProcfile(string(content))
}

// nextScheduledTime is the time for next canary step
func (opts appDeployOptions) nextScheduledTime() time.Time {
	stepInt, _ := time.ParseDuration(opts.stepTimeInterval)
	return time.Now().Add(stepInt)
}

// Timeout parses a user-provided timeout for wait operation and returns it as time.Duration.
func (opts appDeployOptions) Timeout() time.Duration {
	timeout, _ := time.ParseDuration(opts.timeout)
	return timeout
}

func (opts appDeployOptions) isCanarySet() bool {
	return opts.steps > 1
}

func (opts appDeployOptions) KetchYaml() (*ketchv1.KetchYamlData, error) {
	var filename string
	if len(opts.appSourcePath) > 0 {
		// we are ready to read ketch.yaml stored inside the app's source root ...
		ketchYaml := filepath.Join(opts.appSourcePath, "ketch.yaml")
		if stat, err := os.Stat(ketchYaml); err == nil && !stat.IsDir() {
			filename = ketchYaml
		}
	}
	if len(opts.ketchYamlFileName) > 0 {
		// ... but --ketch-yaml=<path-to-ketch-yaml> has higher priority
		filename = opts.ketchYamlFileName
	}
	if len(filename) == 0 {
		return nil, nil
	}
	content, err := ioutil.ReadFile(filename)
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

func watchAppReconcileEvent(ctx context.Context, kubeClient kubernetes.Interface, app *ketchv1.App) (watch.Interface, error) {
	reason := controllers.AppReconcileReason{AppName: app.Name, DeploymentCount: app.Spec.DeploymentsCount}
	selector := fields.Set(map[string]string{
		"involvedObject.apiVersion": v1betaPrefix,
		"involvedObject.kind":       "App",
		"involvedObject.name":       app.Name,
		"reason":                    reason.String(),
	}).AsSelector()
	return kubeClient.CoreV1().
		Events(app.Namespace).Watch(ctx, metav1.ListOptions{FieldSelector: selector.String()})
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
