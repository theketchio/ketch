// Package deploy is purposed to deploy an app.  This concern encompasses creating the app CRD if it doesn't exist,
// possibly creating the app image from source code, and then creating a deployment that will the image in a k8s cluster.
package deploy

import (
	"context"
	"fmt"
	"time"

	registryv1 "github.com/google/go-containerregistry/pkg/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/build"
	"github.com/shipa-corp/ketch/internal/chart"
	"github.com/shipa-corp/ketch/internal/errors"
)

const (
	defaultTrafficWeight = 100
	minimumSteps         = 2
	maximumSteps         = 100
)

type getter interface {
	Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error
}

type creator interface {
	Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error
}

type updater interface {
	Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error
}

type getterCreator interface {
	getter
	creator
	updater
}

type SourceBuilderFn func(context.Context, *build.CreateImageFromSourceRequest, ...build.Option) (*build.CreateImageFromSourceResponse, error)

// runner is concerned with managing and running the deployment.
type runner struct {
	params *ChangeSet
}

// newRunner creates a runner which will execute the deployment.
func newRunner(params *ChangeSet) *runner {
	var r runner
	r.params = params
	return &r
}

// run executes the deployment. This includes creating the application CRD if it doesn't already exist, possibly building
// source code and creating an image and creating and applying a deployment CRD to the cluster.
func (r runner) run(ctx context.Context, svc *Params) error {
	app := new(ketchv1.App)
	err := svc.Client.Get(ctx, types.NamespacedName{Name: r.params.appName}, app)
	if apierrors.IsNotFound(err) {
		app, err = createApp(ctx, svc.Client, r.params)
	} else if err == nil {
		app, err = maybeUpdateApp(ctx, svc.Client, r.params, app)
	}
	if err != nil {
		return err
	}

	if r.params.sourcePath != nil {
		return deployFromSource(ctx, svc, app, r.params)
	}

	return deployFromImage(ctx, svc, app, r.params)
}

func createApp(ctx context.Context, client getterCreator, params *ChangeSet) (*ketchv1.App, error) {
	if err := validateCreateApp(ctx, client, params.appName, params); err != nil {
		return nil, err
	}
	var app ketchv1.App
	app.ObjectMeta.Name = params.appName
	app.Spec.Deployments = []ketchv1.AppDeploymentSpec{}
	app.Spec.Ingress = ketchv1.IngressSpec{
		GenerateDefaultCname: true,
	}

	plat, err := params.getPlatform(ctx, client)
	if err := assign(err, func() {
		app.Spec.Platform = plat
	}); err != nil {
		return nil, err
	}

	framework, err := params.getFramework(ctx, client)
	if err := assign(err, func() {
		app.Spec.Framework = framework
	}); err != nil {
		return nil, err
	}

	description, _ := params.getDescription()
	app.Spec.Description = description

	envs, err := params.getEnvironments()
	if err := assign(err, func() {
		app.Spec.Env = envs
	}); err != nil {
		return nil, err
	}

	secret, err := params.getDockerRegistrySecret()
	app.Spec.DockerRegistry = ketchv1.DockerRegistrySpec{
		SecretName: secret,
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return client.Create(ctx, &app)
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create app %q", params.appName)
	}

	return &app, nil
}

func maybeUpdateApp(ctx context.Context, client getterCreator, params *ChangeSet, app *ketchv1.App) (*ketchv1.App, error) {
	var changed bool
	desc, err := params.getDescription()
	if err := assign(err, func() {
		app.Spec.Description = desc
		changed = true
	}); err != nil {
		return nil, err
	}

	envs, err := params.getEnvironments()
	if err := assign(err, func() {
		app.Spec.Env = envs
		changed = true
	}); err != nil {
		return nil, err
	}

	framework, err := params.getFramework(ctx, client)
	if err := assign(err, func() {
		app.Spec.Framework = framework
		changed = true
	}); err != nil {
		return nil, err
	}

	secret, err := params.getDockerRegistrySecret()
	if err := assign(err, func() {
		app.Spec.DockerRegistry.SecretName = secret
		changed = true
	}); err != nil {
		return nil, err
	}

	platform, err := params.getPlatform(ctx, client)
	if err := assign(err, func() {
		app.Spec.Platform = platform
		changed = true
	}); err != nil {
		return nil, err
	}

	if changed {
		if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			return client.Update(ctx, app)
		}); err != nil {
			return nil, err
		}
	}
	return app, nil
}

func deployFromSource(ctx context.Context, svc *Params, app *ketchv1.App, params *ChangeSet) error {
	if err := validateSourceDeploy(params, app); err != nil {
		return err
	}
	ketchYaml, err := params.getKetchYaml()
	if err != nil {
		return err
	}
	var platform ketchv1.Platform
	if err := svc.Client.Get(ctx, types.NamespacedName{Name: app.Spec.Platform}, &platform); err != nil {
		return errors.Wrap(err, "failed to get platform %q", app.Spec.Platform)
	}

	var framework ketchv1.Framework
	if err := svc.Client.Get(ctx, types.NamespacedName{Name: app.Spec.Framework}, &framework); err != nil {
		return errors.Wrap(err, "failed to get framework %q", app.Spec.Framework)
	}

	image, _ := params.getImage()
	sourcePath, _ := params.getSourceDirectory()
	includeDirs, _ := params.getIncludeDirs()

	resp, err := svc.Builder(
		ctx,
		&build.CreateImageFromSourceRequest{
			Image:         image,
			AppName:       params.appName,
			PlatformImage: platform.Spec.Image,
		},
		build.WithOutput(svc.Writer),
		build.WithWorkingDirectory(sourcePath),
		build.WithSourcePaths(includeDirs...),
		build.MaybeWithBuildHooks(ketchYaml),
	)
	if err != nil {
		return errors.Wrap(err, "build from source failed")
	}

	imageRequest := imageConfigRequest{
		imageName:       image,
		secretName:      app.Spec.DockerRegistry.SecretName,
		secretNamespace: framework.Spec.NamespaceName,
		client:          svc.KubeClient,
	}
	imgConfig, err := svc.GetImageConfig(ctx, imageRequest)
	if err != nil {
		return err
	}

	procfile := resp.Procfile
	if procfile == nil {
		if procfile, err = makeProcfile(imgConfig, params); err != nil {
			return err
		}
	}

	var updateRequest updateAppCRDRequest

	updateRequest.image = image
	steps, _ := params.getSteps()
	updateRequest.steps = steps
	stepWeight, _ := params.getStepWeight()
	updateRequest.stepWeight = stepWeight
	updateRequest.procFile = procfile
	updateRequest.ketchYaml = ketchYaml
	updateRequest.configFile = imgConfig
	interval, _ := params.getStepInterval()
	updateRequest.stepTimeInterval = interval
	updateRequest.nextScheduledTime = time.Now().Add(interval)
	updateRequest.started = time.Now()

	if app, err = updateAppCRD(ctx, svc, params.appName, updateRequest); err != nil {
		return errors.Wrap(err, "deploy from source failed")
	}

	wait, _ := params.getWait()
	if wait {
		timeout, _ := params.getTimeout()
		return svc.Wait(ctx, svc, app, timeout)
	}

	return nil
}

func deployFromImage(ctx context.Context, svc *Params, app *ketchv1.App, params *ChangeSet) error {
	if err := validateDeploy(params, app); err != nil {
		return err
	}
	ketchYaml, err := params.getKetchYaml()
	if err != nil {
		return err
	}
	var platform ketchv1.Platform
	if err := svc.Client.Get(ctx, types.NamespacedName{Name: app.Spec.Platform}, &platform); err != nil {
		return errors.Wrap(err, "failed to get platform %q", app.Spec.Platform)
	}

	var framework ketchv1.Framework
	if err := svc.Client.Get(ctx, types.NamespacedName{Name: app.Spec.Framework}, &framework); err != nil {
		return errors.Wrap(err, "failed to get framework %q", app.Spec.Framework)
	}

	image, _ := params.getImage()

	imageRequest := imageConfigRequest{
		imageName:       image,
		secretName:      app.Spec.DockerRegistry.SecretName,
		secretNamespace: framework.Spec.NamespaceName,
		client:          svc.KubeClient,
	}
	imgConfig, err := svc.GetImageConfig(ctx, imageRequest)
	if err != nil {
		return err
	}

	procfile, err := makeProcfile(imgConfig, params)
	if err != nil {
		return err
	}

	var updateRequest updateAppCRDRequest
	updateRequest.image = image
	steps, _ := params.getSteps()
	updateRequest.steps = steps
	stepWeight, _ := params.getStepWeight()
	updateRequest.stepWeight = stepWeight
	updateRequest.procFile = procfile
	updateRequest.ketchYaml = ketchYaml
	updateRequest.configFile = imgConfig
	interval, _ := params.getStepInterval()
	updateRequest.stepTimeInterval = interval
	updateRequest.nextScheduledTime = time.Now().Add(interval)
	updateRequest.started = time.Now()

	if app, err = updateAppCRD(ctx, svc, params.appName, updateRequest); err != nil {
		return errors.Wrap(err, "deploy from image failed[")
	}

	wait, _ := params.getWait()
	if wait {
		timeout, _ := params.getTimeout()
		return svc.Wait(ctx, svc, app, timeout)
	}

	return nil
}

func makeProcfile(cfg *registryv1.ConfigFile, params *ChangeSet) (*chart.Procfile, error) {
	procFileName, err := params.getProcfileName()
	if !isMissing(err) {
		return chart.NewProcfile(procFileName)
	}

	cmds := append(cfg.Config.Entrypoint, cfg.Config.Cmd...)
	if len(cmds) == 0 {
		return nil, fmt.Errorf("can't use image, no entrypoint or commands")
	}
	return &chart.Procfile{
		Processes: map[string][]string{
			chart.DefaultRoutableProcessName: cmds,
		},
		RoutableProcessName: chart.DefaultRoutableProcessName,
	}, nil
}

type updateAppCRDRequest struct {
	image             string
	steps             int
	stepWeight        uint8
	procFile          *chart.Procfile
	ketchYaml         *ketchv1.KetchYamlData
	configFile        *registryv1.ConfigFile
	nextScheduledTime time.Time
	started           time.Time
	stepTimeInterval  time.Duration
}

func updateAppCRD(ctx context.Context, svc *Params, appName string, args updateAppCRDRequest) (*ketchv1.App, error) {
	var updated ketchv1.App
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := svc.Client.Get(ctx, types.NamespacedName{Name: appName}, &updated); err != nil {
			return errors.Wrap(err, "could not get app to deploy %q", appName)
		}

		processes := make([]ketchv1.ProcessSpec, 0, len(args.procFile.Processes))
		for _, processName := range args.procFile.SortedNames() {
			cmd := args.procFile.Processes[processName]
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
			Version:   ketchv1.DeploymentVersion(updated.Spec.DeploymentsCount + 1),
			Processes: processes,
			KetchYaml: args.ketchYaml,
			RoutingSettings: ketchv1.RoutingSettings{
				Weight: defaultTrafficWeight,
			},
			ExposedPorts: exposedPorts,
		}

		if args.steps > 1 {
			nextScheduledTime := metav1.NewTime(args.nextScheduledTime)
			started := metav1.NewTime(args.started)
			updated.Spec.Canary = ketchv1.CanarySpec{
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
			updated.Spec.Deployments = append(updated.Spec.Deployments, deploymentSpec)
		} else {
			updated.Spec.Deployments = []ketchv1.AppDeploymentSpec{deploymentSpec}
		}

		updated.Spec.DeploymentsCount += 1

		return svc.Client.Update(ctx, &updated)
	})
	return &updated, err
}
