package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
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

Deploy from an image:
  ketch app deploy <app name> -i myregistry/myimage:latest

`
	// default weight used to route incoming traffic to a deployment
	defaultTrafficWeight = 100
	// default step TimeInterval for canary deployments
	defaultStepTimeInterval = "1h"
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
			if len(args) == 2 {
				options.appPath = args[1]
			}
			return appDeploy(cmd.Context(), metav1.Now, cfg, getImageConfigFile, watchAppReconcileEvent, options, out)
		},
	}

	cmd.Flags().StringVarP(&options.image, "image", "i", "", "the image with the application")
	cmd.Flags().StringVar(&options.ketchYamlFileName, "ketch-yaml", "", "the path to ketch.yaml")
	cmd.Flags().StringVar(&options.procfileFileName, "procfile", "", "the path to Procfile. If not set, ketch will use entrypoint and cmd from the image")
	cmd.Flags().BoolVar(&options.strictKetchYamlDecoding, "strict", false, "strict decoding of ketch.yaml")
	cmd.Flags().IntVar(&options.steps, "steps", 1, "number of steps to roll out the new deployment")
	cmd.Flags().StringVar(&options.stepTimeInterval, "step-interval", "1h", "time interval between each step. Supported min: m, hour:h, second:s. ex. 1m, 60s, 1h")
	cmd.Flags().BoolVar(&options.wait, "wait", false, "await for reconcile event")
	cmd.Flags().Uint8Var(&options.timeout, "timeout", 20, "timeout for await of reconcile (seconds)")
	cmd.Flags().StringSliceVar(&options.subPaths, "include-dirs", []string{"."}, "optionally include additional source paths. additional paths must be relative to source-path")
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
	timeout                 uint8
	appPath                 string
	subPaths                []string
}

func (opts *appDeployOptions) validateCanaryOpts() error {
	if opts.steps < 1 || opts.steps > 100 {
		return fmt.Errorf("steps must be within the range 1 to 100")
	}

	opts.stepWeight = uint8(defaultTrafficWeight / opts.steps)

	// normalize to reach 100% traffic
	if defaultTrafficWeight%opts.stepWeight != 0 {
		opts.stepWeight++
	}

	if opts.stepTimeInterval == "" {
		opts.stepTimeInterval = defaultStepTimeInterval
	}

	_, err := time.ParseDuration(opts.stepTimeInterval)
	if err != nil {
		return fmt.Errorf("invalid step interval: %w", err)
	}

	return nil
}

func (opts appDeployOptions) isCanarySet() bool {
	return opts.steps > 1
}

type getImageConfigFileFn func(ctx context.Context, kubeClient kubernetes.Interface, args getImageConfigArgs, fn getRemoteImageFn) (*registryv1.ConfigFile, error)

type watchReconcileEventFn func(ctx context.Context, kubeClient kubernetes.Interface, app *ketchv1.App) (watch.Interface, error)

// pass timeNowFn to appDeploy(). Useful for testing canary deployments.
type timeNowFn func() metav1.Time

func appDeploy(ctx context.Context, timeNow timeNowFn, cfg config, getImageConfigFile getImageConfigFileFn, watchReconcileEvent watchReconcileEventFn, options appDeployOptions, out io.Writer) error {
	if options.appPath != "" {
		dockerSvc, err := docker.New()
		if err != nil {
			return err
		}
		defer dockerSvc.Close()

		ketchYaml, err := options.KetchYaml()
		if err != nil {
			return errors.Wrap(err, "ketch.yaml could not be processed")
		}

		_, err = build.GetSourceHandler(dockerSvc, cfg.Client())(
			ctx,
			&build.CreateImageFromSourceRequest{
				Image:   options.image,
				AppName: options.appName,
			},
			build.WithOutput(out),
			build.WithWorkingDirectory(options.appPath),
			build.WithSourcePaths(options.subPaths...),
			build.MaybeWithBuildHooks(ketchYaml),
		)
		if err != nil {
			return err
		}
	}
	return appDeployImage(ctx, timeNow, cfg, getImageConfigFile, watchReconcileEvent, options, out)
}

func appDeployImage(ctx context.Context, timeNow timeNowFn, cfg config, getImageConfigFile getImageConfigFileFn, watchReconcileEvent watchReconcileEventFn, options appDeployOptions, out io.Writer) error {
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

	// default deployment spec for an app
	deploymentSpec := ketchv1.AppDeploymentSpec{
		Image:     options.image,
		Version:   ketchv1.DeploymentVersion(app.Spec.DeploymentsCount + 1),
		Processes: processes,
		KetchYaml: ketchYaml,
		RoutingSettings: ketchv1.RoutingSettings{
			Weight: defaultTrafficWeight,
		},
		ExposedPorts: exposedPorts,
	}

	if options.isCanarySet() {
		if err := options.validateCanaryOpts(); err != nil {
			return err
		}

		// check for old deployments
		switch deps := len(app.Spec.Deployments); {
		case deps == 0:
			return fmt.Errorf("canary deployment failed. No primary deployment found for the app")
		case deps >= 2:
			return fmt.Errorf("canary deployment failed. Maximum number of two deployments are currently supported")
		}

		// parses step interval string to time.Duration
		stepInt, _ := time.ParseDuration(options.stepTimeInterval)
		// nextScheduledTime is the time for next canary step
		nextScheduledTime := metav1.NewTime(timeNow().Add(stepInt))
		app.Spec.Canary = ketchv1.CanarySpec{
			Steps:             options.steps,
			StepWeight:        options.stepWeight,
			StepTimeInteval:   stepInt,
			NextScheduledTime: &nextScheduledTime,
			CurrentCanaryStep: 1,
			IsActiveCanary:    true,
		}

		// set weight for canary deployment
		deploymentSpec.RoutingSettings.Weight = options.stepWeight

		//  update old deployment weight
		app.Spec.Deployments[0].RoutingSettings.Weight = defaultTrafficWeight - options.stepWeight

		// For a canary deployment, canary should be enabled by adding another deployment to the deployment list.
		app.Spec.Deployments = append(app.Spec.Deployments, deploymentSpec)
	} else {
		app.Spec.Deployments = []ketchv1.AppDeploymentSpec{deploymentSpec}
	}

	app.Spec.DeploymentsCount += 1

	var watcher watch.Interface
	if options.wait {
		watcher, err = watchReconcileEvent(ctx, cfg.KubernetesClient(), &app)
		if err != nil {
			return err
		}
		defer watcher.Stop()
	}

	if err = cfg.Client().Update(ctx, &app); err != nil {
		return fmt.Errorf("failed to update the app: %v", err)
	}

	// Await for reconcile result
	if options.wait {
		maxExecTime := time.NewTimer(time.Second * time.Duration(options.timeout))
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
	fmt.Fprintln(out, "app crd updated successfully, check the app’s events to understand results of the deployment")
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

func watchAppReconcileEvent(ctx context.Context, kubeClient kubernetes.Interface, app *ketchv1.App) (watch.Interface, error) {
	reason := controllers.AppReconcileReason{Name: app.Name, DeploymentCount: app.Spec.DeploymentsCount}
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
