/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package controllers contains App and Framework reconcilers to be used with controller-runtime.
package controllers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/api/autoscaling/v2beta1"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"
	"github.com/theketchio/ketch/internal/chart"
	"github.com/theketchio/ketch/internal/templates"
)

// AppReconciler reconciles a App object.
type AppReconciler struct {
	client.Client
	Log            logr.Logger
	Scheme         *runtime.Scheme
	TemplateReader templates.Reader
	HelmFactoryFn  helmFactoryFn
	Now            timeNowFn
	Recorder       record.EventRecorder
	// Group stands for k8s group of Ketch App CRD.
	Group  string
	Config *rest.Config
	// CancelMap tracks cancelFunc functions for goroutines AppReconciler starts to watch deployment events.
	CancelMap *CancelMap
}

// timeNowFn knows how to get the current time.
// Useful for canary deployments using App Reconclier.
type timeNowFn func() time.Time

type helmFactoryFn func(namespace string) (Helm, error)

// Helm has methods to update/delete helm charts.
type Helm interface {
	UpdateChart(tv chart.TemplateValuer, config chart.ChartConfig, opts ...chart.InstallOption) (*release.Release, error)
	DeleteChart(appName string) error
}

const (
	DeploymentProgressing         = "Progressing"
	deadlineExeceededProgressCond = "ProgressDeadlineExceeded"
	DefaultPodRunningTimeout      = 10 * time.Minute
	maxWaitTimeDuration           = time.Duration(120) * time.Second
)

// +kubebuilder:rbac:groups=theketch.io,resources=apps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=theketch.io,resources=apps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=theketch.io,resources=frameworks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=theketch.io,resources=frameworks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="apps",resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="networking.k8s.io",resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="networking.istio.io",resources=gateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="networking.istio.io",resources=virtualservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="networking.istio.io",resources=destinationrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="cert-manager.io",resources=certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="traefik.containo.us",resources=ingressroutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="traefik.containo.us",resources=ingressroutes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="traefik.containo.us",resources=traefikservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="traefik.containo.us",resources=traefikservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="traefik.containo.us",resources=middlewares,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch;update;delete;list;watch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;create;update
// +kubebuilder:rbac:groups="autoscaling",resources=horizontalpodautoscalers,verbs=list

func (r *AppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("app", req.NamespacedName)

	app := ketchv1.App{}
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !controllerutil.ContainsFinalizer(&app, ketchv1.KetchFinalizer) {
		controllerutil.AddFinalizer(&app, ketchv1.KetchFinalizer)
		if err := r.Update(ctx, &app); err != nil {
			logger.Error(err, "failed to add ketch finalizer")
			return ctrl.Result{}, err
		}
	}

	if !app.ObjectMeta.DeletionTimestamp.IsZero() {
		err := r.deleteChart(ctx, &app)
		return ctrl.Result{}, err
	}

	scheduleResult := r.reconcile(ctx, &app, logger)
	if scheduleResult.isConflictError() || isCanceledError(scheduleResult.err) {
		// we don't want to create an event with this conflict error and show it to the user.
		// ketch will eventually reconcile the app.
		logger.Error(scheduleResult.err, "failed to reconcile app")
		return ctrl.Result{}, scheduleResult.err
	}

	var (
		err    error
		result ctrl.Result
	)

	if scheduleResult.err != nil {
		err = scheduleResult.err
		outcome := ketchv1.AppReconcileOutcome{AppName: app.Name, DeploymentCount: app.Spec.DeploymentsCount}
		r.Recorder.Event(&app, v1.EventTypeWarning, ketchv1.AppReconcileOutcomeReason, outcome.String(err))
		app.SetCondition(ketchv1.Scheduled, v1.ConditionFalse, scheduleResult.err.Error(), metav1.NewTime(time.Now()))
	} else {
		app.Status.Framework = scheduleResult.framework
		outcome := ketchv1.AppReconcileOutcome{AppName: app.Name, DeploymentCount: app.Spec.DeploymentsCount}
		r.Recorder.Event(&app, v1.EventTypeNormal, ketchv1.AppReconcileOutcomeReason, outcome.String())
		app.SetCondition(ketchv1.Scheduled, v1.ConditionTrue, "", metav1.NewTime(time.Now()))
	}

	if err := r.Status().Update(context.Background(), &app); err != nil {
		if k8sErrors.IsConflict(err) {
			// we don't want to create an event with this conflict error and show it to the user.
			// ketch will eventually reconcile the app.
			logger.Error(err, "failed to reconcile app status")
			return ctrl.Result{}, err
		}
		outcome := ketchv1.AppReconcileOutcome{AppName: app.Name, DeploymentCount: app.Spec.DeploymentsCount}
		r.Recorder.Event(&app, v1.EventTypeWarning, ketchv1.AppReconcileOutcomeReason, outcome.String(err))
		return result, err
	}

	// use canary step interval as the timeout when canary is active
	if app.Spec.Canary.Active {
		result = ctrl.Result{RequeueAfter: app.Spec.Canary.StepTimeInteval}
	}

	if scheduleResult.useTimeout {
		// set default timeout
		result = ctrl.Result{RequeueAfter: reconcileTimeout}
	}
	return result, err
}

func hpaTargetMap(app *ketchv1.App, hpaList v2beta1.HorizontalPodAutoscalerList) map[string]bool {
	targets := map[string]v2beta1.CrossVersionObjectReference{}
	for _, target := range hpaList.Items {
		targets[target.Spec.ScaleTargetRef.Name] = target.Spec.ScaleTargetRef
	}

	hpaTargets := map[string]bool{}
	for _, deployment := range app.Spec.Deployments {
		for _, process := range deployment.Processes {

			deploymentName := fmt.Sprintf("%s-%s-%s", app.Name, process.Name, deployment.Version)
			if details, ok := targets[deploymentName]; ok {
				// even if a target name is a match, it could be targeting a different kind than deployment
				if details.Kind == "Deployment" && details.APIVersion == "apps/v1" {
					hpaTargets[process.Name] = true
				}
			}
		}
	}
	return hpaTargets
}

type appReconcileResult struct {
	framework  *v1.ObjectReference
	useTimeout bool
	err        error
}

// isConflictError returns true if AppReconciler was trying to update an App CR and got a conflict error.
func (r appReconcileResult) isConflictError() bool {
	err := r.err
	for {
		// we need this loop to properly handle cases like:
		// fmt.Error("some err %w", conflictErr)
		// fmt.Error("some err %w", fmt.Error("some err %w", conflictErr))
		if err == nil {
			return false
		}
		if k8sErrors.IsConflict(err) {
			return true
		}
		err = errors.Unwrap(err)
	}
}

// isCanceledError returns true if the given error is "context.Canceled" error.
func isCanceledError(err error) bool {
	for {
		if err == nil {
			return false
		}
		if err == context.Canceled {
			return true
		}
		err = errors.Unwrap(err)
	}
}

type condition struct {
	Type   string
	Reason string
}

// workload contains the needed information for watchDeployEvents logic
// deployments and statefulsets are both supported so it became necessary
// to abstract their common properties into a separate type
type workload struct {
	Name               string
	Replicas           int
	UpdatedReplicas    int
	ReadyReplicas      int
	Generation         int
	ObservedGeneration int
	Conditions         []condition
}

type workloadClient struct {
	workloadType      ketchv1.AppType
	workloadName      string
	workloadNamespace string
	k8sClient         kubernetes.Interface
}

// Get populates workload values based on the workloadType and returns the populated struct
func (cli workloadClient) Get(ctx context.Context) (*workload, error) {
	switch cli.workloadType {
	case ketchv1.DeploymentAppType:
		o, err := cli.k8sClient.AppsV1().Deployments(cli.workloadNamespace).Get(ctx, cli.workloadName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		w := workload{
			Name:               o.Name,
			UpdatedReplicas:    int(o.Status.UpdatedReplicas),
			ReadyReplicas:      int(o.Status.ReadyReplicas),
			Generation:         int(o.Generation),
			ObservedGeneration: int(o.Status.ObservedGeneration),
		}
		if o.Spec.Replicas != nil {
			w.Replicas = int(*o.Spec.Replicas)
		}
		for _, c := range o.Status.Conditions {
			w.Conditions = append(w.Conditions, condition{Type: string(c.Type), Reason: c.Reason})
		}
		return &w, nil
	case ketchv1.StatefulSetAppType:
		o, err := cli.k8sClient.AppsV1().StatefulSets(cli.workloadNamespace).Get(ctx, cli.workloadName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		w := workload{
			Name:               o.Name,
			UpdatedReplicas:    int(o.Status.UpdatedReplicas),
			ReadyReplicas:      int(o.Status.ReadyReplicas),
			Generation:         int(o.Generation),
			ObservedGeneration: int(o.Status.ObservedGeneration),
		}
		if o.Spec.Replicas != nil {
			w.Replicas = int(*o.Spec.Replicas)
		}
		for _, c := range o.Status.Conditions {
			w.Conditions = append(w.Conditions, condition{Type: string(c.Type), Reason: c.Reason})
		}
		return &w, nil
	}
	return nil, fmt.Errorf("unknown workload type")
}

func (r *AppReconciler) reconcile(ctx context.Context, app *ketchv1.App, logger logr.Logger) appReconcileResult {

	framework := ketchv1.Framework{}
	if err := r.Get(ctx, types.NamespacedName{Name: app.Spec.Framework}, &framework); err != nil {
		return appReconcileResult{
			err: fmt.Errorf(`framework "%s" is not found`, app.Spec.Framework),
		}
	}
	ref, err := reference.GetReference(r.Scheme, &framework)
	if err != nil {
		return appReconcileResult{err: err}
	}
	if framework.Status.Namespace == nil {
		return appReconcileResult{
			err: fmt.Errorf(`framework "%s" is not linked to a kubernetes namespace`, framework.Name),
		}
	}
	tpls, err := r.TemplateReader.Get(templates.IngressConfigMapName(framework.Spec.IngressController.IngressType.String()))
	if err != nil {
		return appReconcileResult{
			err: fmt.Errorf(`failed to read configmap with the app's chart templates: %w`, err),
		}
	}
	if !framework.HasApp(app.Name) && framework.Spec.AppQuotaLimit != nil && len(framework.Status.Apps) >= *framework.Spec.AppQuotaLimit && *framework.Spec.AppQuotaLimit != -1 {
		return appReconcileResult{
			err: fmt.Errorf(`you have reached the limit of apps`),
		}
	}

	appChrt, err := chart.New(app, &framework,
		chart.WithExposedPorts(app.ExposedPorts()),
		chart.WithTemplates(*tpls))
	if err != nil {
		return appReconcileResult{err: err}
	}

	patchedFramework := framework
	if !patchedFramework.HasApp(app.Name) {
		patchedFramework.Status.Apps = append(patchedFramework.Status.Apps, app.Name)
		mergePatch := client.MergeFrom(&framework)
		if err := r.Status().Patch(ctx, &patchedFramework, mergePatch); err != nil {
			return appReconcileResult{
				err: fmt.Errorf("failed to update framework status: %w", err),
			}
		}
	}
	targetNamespace := framework.Status.Namespace.Name
	helmClient, err := r.HelmFactoryFn(targetNamespace)
	if err != nil {
		return appReconcileResult{err: err}
	}

	// check for canary deployment
	if app.Spec.Canary.Active {
		// ensures that the canary deployment exists
		if len(app.Spec.Deployments) <= 1 {
			// reset canary specs
			app.Spec.Canary = ketchv1.CanarySpec{}
			return appReconcileResult{
				err: fmt.Errorf("no canary deployment found"),
			}
		}

		// retry until all pods for canary deployment comes to running state.
		if _, err := checkPodStatus(r.Group, r.Client, app.Name, app.Spec.Deployments[1].Version); len(app.Spec.Deployments) > 1 && err != nil {

			if !timeoutExpired(app.Spec.Canary.Started, r.Now()) {
				return appReconcileResult{
					err:        fmt.Errorf("canary update failed: %w", err),
					useTimeout: true,
				}
			}

			// Do rollback if timeout expired
			app.DoRollback()
			if err := r.Update(ctx, app); err != nil {
				return appReconcileResult{
					err: fmt.Errorf("failed to update app crd: %w", err),
				}
			}
		}

		var hpaList v2beta1.HorizontalPodAutoscalerList
		if err := r.List(ctx, &hpaList, &client.ListOptions{Namespace: framework.Status.Namespace.Name}); err != nil {
			return appReconcileResult{
				err: fmt.Errorf("failed to find HPAs"),
			}
		}

		// Once all pods are running then Perform canary deployment, do not scale pods for a process that is a HPA target.
		if err = app.DoCanary(metav1.NewTime(r.Now()), logger, r.Recorder, hpaTargetMap(app, hpaList)); err != nil {
			return appReconcileResult{
				err: fmt.Errorf("canary update failed: %w", err),
			}
		}
		if err := r.Update(ctx, app); err != nil {
			return appReconcileResult{
				err: fmt.Errorf("canary update failed: %w", err),
			}
		}
	}

	_, err = helmClient.UpdateChart(*appChrt, chart.NewChartConfig(*app))
	if err != nil {
		return appReconcileResult{
			err: fmt.Errorf("failed to update helm chart: %w", err),
		}
	}

	if len(app.Spec.Deployments) > 0 && !app.Spec.Canary.Active {
		// use latest deployment and watch events for each process
		latestDeployment := app.Spec.Deployments[len(app.Spec.Deployments)-1]
		for _, process := range latestDeployment.Processes {

			cli, err := kubernetes.NewForConfig(r.Config)
			if err != nil {
				return appReconcileResult{
					err: err,
				}
			}

			wc := workloadClient{
				k8sClient:         cli,
				workloadName:      fmt.Sprintf("%s-%s-%d", app.GetName(), process.Name, latestDeployment.Version),
				workloadNamespace: framework.Spec.NamespaceName,
				workloadType:      app.Spec.GetType(),
			}

			wl, err := wc.Get(ctx)
			if err != nil {
				return appReconcileResult{
					err: err,
				}
			}

			err = r.watchDeployEvents(ctx, app, &wc, wl, &process, r.Recorder)
			if err != nil {
				return appReconcileResult{
					err: fmt.Errorf("failed to get deploy events: %w", err),
				}
			}
		}
		// We useTimeout here to set reconcile.ReququeAfter in the Reconciler
		// in order to ensure events actually get sent. It seems the lazyRecorder we use
		// can stop with unhandled messages if the reconciler rapidly requeues.
		return appReconcileResult{
			framework:  ref,
			useTimeout: true,
		}
	}

	return appReconcileResult{
		framework: ref,
	}
}

// watchDeployEvents watches a namespace for events and, after a deployment has started updating, records events
// with updated deployment status and/or healthcheck and timeout failures
func (r *AppReconciler) watchDeployEvents(ctx context.Context, app *ketchv1.App, cli *workloadClient, wl *workload, process *ketchv1.ProcessSpec, recorder record.EventRecorder) error {
	opts := metav1.ListOptions{
		FieldSelector: "involvedObject.kind=Pod",
		Watch:         true,
	}
	watcher, err := cli.k8sClient.CoreV1().Events(cli.workloadNamespace).Watch(ctx, opts) // requires "watch" permission on events in clusterrole
	if err != nil {
		if strings.Contains(err.Error(), "unknown (get events)") {
			err = errors.WithMessagef(err, "assure clusterrole 'manager-role' has 'watch' permissions on event resources")
		}
		watchErrorEvent := newAppDeploymentEvent(app, ketchv1.AppReconcileError, fmt.Sprintf("error watching deployments for workload %s: %s", wl.Name, err.Error()), process.Name, "")
		recorder.AnnotatedEventf(app, watchErrorEvent.Annotations, v1.EventTypeWarning, watchErrorEvent.Reason, watchErrorEvent.Description)
		return err
	}

	// wait for Deployment Generation
	timeout := time.After(DefaultPodRunningTimeout)
	for wl.ObservedGeneration < wl.Generation {
		wl, err = cli.Get(ctx)
		if isCanceledError(err) {
			// if the context is canceled,
			// probably, the app CR has been updated and controller-runtime is canceling our operation.
			// we don't want to emit an event in this case
			return err
		}
		if err != nil {
			recorder.Eventf(app, v1.EventTypeWarning, ketchv1.AppReconcileError, "error getting deployments: %s", err.Error())
			return err
		}
		select {
		case <-time.After(100 * time.Millisecond):
		case <-timeout:
			recorder.Event(app, v1.EventTypeWarning, ketchv1.AppReconcileError, "timeout waiting for deployment generation to update")
			return errors.Errorf("timeout waiting for deployment generation to update")
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	// assign current cancelFunc and cancel the previous one
	cleanup := r.CancelMap.replaceAndCancelPrevious(wl.Name, cancel)

	reconcileStartedEvent := newAppDeploymentEvent(app, ketchv1.AppReconcileStarted, fmt.Sprintf("Updating units [%s]", process.Name), process.Name, "")
	recorder.AnnotatedEventf(app, reconcileStartedEvent.Annotations, v1.EventTypeNormal, reconcileStartedEvent.Reason, reconcileStartedEvent.Description)
	go r.watchFunc(ctx, cleanup, app, process.Name, recorder, watcher, cli, wl, timeout)
	return nil
}

func (r *AppReconciler) watchFunc(ctx context.Context, cleanup cleanupFunc, app *ketchv1.App, processName string, recorder record.EventRecorder, watcher watch.Interface, cli *workloadClient, wl *workload, timeout <-chan time.Time) error {
	defer cleanup()

	var err error
	watchCh := watcher.ResultChan()
	defer watcher.Stop()

	specReplicas := wl.Replicas
	oldUpdatedReplicas := -1
	oldReadyUnits := -1
	oldPendingTermination := -1
	now := time.Now()
	var healthcheckTimeout <-chan time.Time

	for {
		for i := range wl.Conditions {
			c := wl.Conditions[i]
			if c.Type == DeploymentProgressing && c.Reason == deadlineExeceededProgressCond {
				deadlineExceededEvent := newAppDeploymentEvent(app, ketchv1.AppReconcileError, fmt.Sprintf("deployment %q exceeded its progress deadline", wl.Name), processName, "")
				recorder.AnnotatedEventf(app, deadlineExceededEvent.Annotations, v1.EventTypeWarning, deadlineExceededEvent.Reason, deadlineExceededEvent.Description)
				return errors.Errorf("deployment %q exceeded its progress deadline", wl.Name)
			}
		}
		if oldUpdatedReplicas != wl.UpdatedReplicas {
			unitsCreatedEvent := newAppDeploymentEvent(app, ketchv1.AppReconcileUpdate, fmt.Sprintf("%d of %d new units created", wl.UpdatedReplicas, specReplicas), processName, "")
			recorder.AnnotatedEventf(app, unitsCreatedEvent.Annotations, v1.EventTypeNormal, unitsCreatedEvent.Reason, unitsCreatedEvent.Description)
		}

		if healthcheckTimeout == nil && wl.UpdatedReplicas == specReplicas {
			_, err := checkPodStatus(r.Group, r.Client, app.Name, app.Spec.Deployments[len(app.Spec.Deployments)-1].Version)
			if err == nil {
				healthcheckTimeout = time.After(maxWaitTimeDuration)
				healthcheckEvent := newAppDeploymentEvent(app, ketchv1.AppReconcileUpdate, fmt.Sprintf("waiting healthcheck on %d created units", specReplicas), processName, "")
				recorder.AnnotatedEventf(app, healthcheckEvent.Annotations, v1.EventTypeNormal, healthcheckEvent.Reason, healthcheckEvent.Description)
			}
		}

		readyUnits := wl.ReadyReplicas
		if oldReadyUnits != readyUnits && readyUnits >= 0 {
			unitsReadyEvent := newAppDeploymentEvent(app, ketchv1.AppReconcileUpdate, fmt.Sprintf("%d of %d new units ready", readyUnits, specReplicas), processName, "")
			recorder.AnnotatedEventf(app, unitsReadyEvent.Annotations, v1.EventTypeNormal, unitsReadyEvent.Reason, unitsReadyEvent.Description)
		}

		pendingTermination := wl.Replicas - wl.UpdatedReplicas
		if oldPendingTermination != pendingTermination && pendingTermination > 0 {
			pendingTerminationEvent := newAppDeploymentEvent(app, ketchv1.AppReconcileUpdate, fmt.Sprintf("%d old units pending termination", pendingTermination), processName, "")
			recorder.AnnotatedEventf(app, pendingTerminationEvent.Annotations, v1.EventTypeNormal, pendingTerminationEvent.Reason, pendingTerminationEvent.Description)
		}

		oldUpdatedReplicas = wl.UpdatedReplicas
		oldReadyUnits = readyUnits
		oldPendingTermination = pendingTermination
		if readyUnits == specReplicas &&
			wl.Replicas == specReplicas {
			break
		}

		select {
		case <-time.After(100 * time.Millisecond):
		case msg, isOpen := <-watchCh:
			if !isOpen {
				break
			}
			if isDeploymentEvent(msg, wl.Name) {
				appDeploymentEvent := appDeploymentEventFromWatchEvent(msg, app, processName)
				recorder.AnnotatedEventf(app, appDeploymentEvent.Annotations, v1.EventTypeNormal, ketchv1.AppReconcileUpdate, appDeploymentEvent.Description)
			}
		case <-healthcheckTimeout:
			podName, _ := checkPodStatus(r.Group, r.Client, app.Name, app.Spec.Deployments[len(app.Spec.Deployments)-1].Version)
			err = createDeployTimeoutError(ctx, cli.k8sClient, app, time.Since(now), cli.workloadNamespace, app.GroupVersionKind().Group, "healthcheck")
			healthcheckTimeoutEvent := newAppDeploymentEvent(app, ketchv1.AppReconcileError, fmt.Sprintf("error waiting for healthcheck: %s", err.Error()), processName, podName)
			recorder.AnnotatedEventf(app, healthcheckTimeoutEvent.Annotations, v1.EventTypeWarning, healthcheckTimeoutEvent.Reason, healthcheckTimeoutEvent.Description)
			return err
		case <-timeout:
			podName, _ := checkPodStatus(r.Group, r.Client, app.Name, app.Spec.Deployments[len(app.Spec.Deployments)-1].Version)
			err = createDeployTimeoutError(ctx, cli.k8sClient, app, time.Since(now), cli.workloadNamespace, app.GroupVersionKind().Group, "full rollout")
			timeoutEvent := newAppDeploymentEvent(app, ketchv1.AppReconcileError, fmt.Sprintf("deployment timeout: %s", err.Error()), processName, podName)
			recorder.AnnotatedEventf(app, timeoutEvent.Annotations, v1.EventTypeWarning, timeoutEvent.Reason, timeoutEvent.Description)
			return err
		case <-ctx.Done():
			return ctx.Err()
		}

		newWorkload, err := cli.Get(ctx)
		if isCanceledError(err) {
			// if the context is canceled,
			// probably, the app CR has been updated and controller-runtime is canceling our operation.
			// we don't want to emit an event in this case
			return err
		}
		if err != nil && !k8sErrors.IsNotFound(err) {
			podName, _ := checkPodStatus(r.Group, r.Client, app.Name, app.Spec.Deployments[len(app.Spec.Deployments)-1].Version)
			deploymentErrorEvent := newAppDeploymentEvent(app, ketchv1.AppReconcileError, fmt.Sprintf("error getting deployments: %s", err.Error()), processName, podName)
			recorder.AnnotatedEventf(app, deploymentErrorEvent.Annotations, v1.EventTypeWarning, deploymentErrorEvent.Reason, deploymentErrorEvent.Description)
			return err
		}
		if err == nil {
			wl = newWorkload
		}
	}

	outcome := ketchv1.AppReconcileOutcome{AppName: app.Name, DeploymentCount: wl.ReadyReplicas}
	outcomeEvent := newAppDeploymentEvent(app, ketchv1.AppReconcileComplete, outcome.String(), processName, "")
	recorder.AnnotatedEventf(app, outcomeEvent.Annotations, v1.EventTypeNormal, outcomeEvent.Reason, outcomeEvent.Description)
	return nil
}

// appDeploymentEventFromWatchEvent converts a watch.Event into an AppDeploymentEvent
func appDeploymentEventFromWatchEvent(watchEvent watch.Event, app *ketchv1.App, processName string) *ketchv1.AppDeploymentEvent {
	event, ok := watchEvent.Object.(*v1.Event)
	if !ok {
		return nil
	}
	var version int
	if len(app.Spec.Deployments) > 0 {
		version = int(app.Spec.Deployments[len(app.Spec.Deployments)-1].Version)
	}
	return &ketchv1.AppDeploymentEvent{
		Name:              app.Name,
		DeploymentVersion: version,
		Reason:            event.Reason,
		Description:       event.Message,
		ProcessName:       processName,
		Annotations: map[string]string{
			ketchv1.DeploymentAnnotationAppName:                 app.Name,
			ketchv1.DeploymentAnnotationDevelopmentVersion:      strconv.Itoa(version),
			ketchv1.DeploymentAnnotationEventName:               event.Reason,
			ketchv1.DeploymentAnnotationDescription:             event.Message,
			ketchv1.DeploymentAnnotationProcessName:             processName,
			ketchv1.DeploymentAnnotationInvolvedObjectName:      event.InvolvedObject.Name,
			ketchv1.DeploymentAnnotationInvolvedObjectFieldPath: event.InvolvedObject.FieldPath,
			ketchv1.DeploymentAnnotationSourceHost:              event.Source.Host,
			ketchv1.DeploymentAnnotationSourceComponent:         event.Source.Component,
		},
	}
}

// newAppDeploymentEvent creates a new AppDeploymentEvent, creating Annotations for use when emitting App Events.
func newAppDeploymentEvent(app *ketchv1.App, reason, desc, processName, podName string) *ketchv1.AppDeploymentEvent {
	var version int
	if len(app.Spec.Deployments) > 0 {
		version = int(app.Spec.Deployments[len(app.Spec.Deployments)-1].Version)
	}
	return &ketchv1.AppDeploymentEvent{
		Name:              app.Name,
		DeploymentVersion: version,
		Reason:            reason,
		Description:       desc,
		ProcessName:       processName,
		Annotations: map[string]string{
			ketchv1.DeploymentAnnotationAppName:            app.Name,
			ketchv1.DeploymentAnnotationDevelopmentVersion: strconv.Itoa(version),
			ketchv1.DeploymentAnnotationEventName:          reason,
			ketchv1.DeploymentAnnotationDescription:        desc,
			ketchv1.DeploymentAnnotationProcessName:        processName,
			ketchv1.DeploymentAnnotationPodErrorName:       podName,
		},
	}
}

// isDeploymentEvent returns true if the watchEvent is an Event type and matches the deployment.Name
func isDeploymentEvent(msg watch.Event, name string) bool {
	evt, ok := msg.Object.(*v1.Event)
	return ok && strings.HasPrefix(evt.Name, name)
}

// createDeployTimeoutError gets pods that are not status == ready aggregates and returns the pod phase errors
func createDeployTimeoutError(ctx context.Context, cli kubernetes.Interface, app *ketchv1.App, timeout time.Duration, namespace, group, label string) error {
	var deploymentVersion int
	if len(app.Spec.Deployments) > 0 {
		deploymentVersion = int(app.Spec.Deployments[len(app.Spec.Deployments)-1].Version)
	}
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s/app-name=%s,%s/app-deployment-version=%d", group, app.Name, group, deploymentVersion),
	}
	pods, err := cli.CoreV1().Pods(app.GetNamespace()).List(ctx, opts)
	if err != nil {
		return err
	}
	var podsForEvts []*v1.Pod
podsLoop:
	for i, pod := range pods.Items {
		for _, cond := range pod.Status.Conditions {
			if cond.Type == v1.PodReady && cond.Status != v1.ConditionTrue {
				podsForEvts = append(podsForEvts, &pods.Items[i])
				continue podsLoop
			}
		}
	}
	var messages []string
	for _, pod := range podsForEvts {
		err = newInvalidPodPhaseError(ctx, cli, pod, namespace)
		messages = append(messages, fmt.Sprintf("Pod %s: %v", pod.Name, err))
	}
	var msgErrorPart string
	if len(messages) > 0 {
		msgErrorPart += fmt.Sprintf(": %s", strings.Join(messages, ", "))
	}
	return errors.Errorf("timeout waiting %s after %v waiting for units%s", label, timeout, msgErrorPart)
}

// newInvalidPodPhaseError returns an error formatted with pod.Status.Phase details and the latest event message
func newInvalidPodPhaseError(ctx context.Context, cli kubernetes.Interface, pod *v1.Pod, namespace string) error {
	phaseWithMsg := fmt.Sprintf("%q", pod.Status.Phase)
	if pod.Status.Message != "" {
		phaseWithMsg = fmt.Sprintf("%s(%q)", phaseWithMsg, pod.Status.Message)
	}
	retErr := errors.Errorf("invalid pod phase %s", phaseWithMsg)
	eventsInterface := cli.CoreV1().Events(namespace)
	selector := eventsInterface.GetFieldSelector(&pod.Name, &namespace, nil, nil)
	options := metav1.ListOptions{FieldSelector: selector.String()}
	events, err := eventsInterface.List(ctx, options)
	if err == nil && len(events.Items) > 0 {
		lastEvt := events.Items[len(events.Items)-1]
		retErr = errors.Errorf("%v - last event: %s", retErr, lastEvt.Message)
	}
	return retErr
}

// check if timeout has expired
func timeoutExpired(t *metav1.Time, now time.Time) bool {
	return t.Add(reconcileTimeout).Before(now)
}

// checkPodStatus checks whether all pods for a deployment are running or not.
// If a Pod is found in a non-healthy state, it's name is returned
func checkPodStatus(group string, c client.Client, appName string, depVersion ketchv1.DeploymentVersion) (string, error) {
	if c == nil {
		return "", errors.New("client must be non-nil")
	}

	if len(appName) == 0 || depVersion <= 0 {
		return "", errors.New("invalid app specifications")
	}

	// podList contains list of Pods matching the specified labels below
	podList := &v1.PodList{}
	listOpts := []client.ListOption{
		// The specified labels below matches with the required deployment pods of the app.
		client.MatchingLabels(map[string]string{
			group + "/app-name":               appName,
			group + "/app-deployment-version": depVersion.String()}),
	}

	if err := c.List(context.Background(), podList, listOpts...); err != nil {
		return "", err
	}

	// check if all pods are running for the deployment
	for _, pod := range podList.Items {
		// check if pod have voluntarily terminated with a container exit code of 0
		if pod.Status.Phase == v1.PodSucceeded {
			return "", nil
		}

		if pod.Status.Phase != v1.PodRunning {
			return pod.Name, errors.New("all pods are not running")
		}

		for _, c := range pod.Status.Conditions {
			if c.Status != v1.ConditionTrue {
				return pod.Name, errors.New("all pods are not in healthy state")
			}
		}
	}
	return "", nil
}

func (r *AppReconciler) deleteChart(ctx context.Context, app *ketchv1.App) error {
	frameworks := ketchv1.FrameworkList{}
	err := r.Client.List(ctx, &frameworks)
	if err != nil {
		return err
	}
	for _, framework := range frameworks.Items {
		if !framework.HasApp(app.Name) {
			continue
		}

		if uninstallHelmChart(r.Group, app.Annotations) {
			helmClient, err := r.HelmFactoryFn(framework.Spec.NamespaceName)
			if err != nil {
				return err
			}
			if err = helmClient.DeleteChart(app.Name); err != nil {
				return err
			}
		}

		patchedFramework := framework

		patchedFramework.Status.Apps = make([]string, 0, len(patchedFramework.Status.Apps))
		for _, name := range framework.Status.Apps {
			if name == app.Name {
				continue
			}
			patchedFramework.Status.Apps = append(patchedFramework.Status.Apps, name)
		}
		mergePatch := client.MergeFrom(&framework)
		if err := r.Status().Patch(ctx, &patchedFramework, mergePatch); err != nil {
			return err
		}
		break
	}

	controllerutil.RemoveFinalizer(app, ketchv1.KetchFinalizer)
	if err := r.Update(ctx, app); err != nil {
		return err
	}
	return nil

}

func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// to avoid re-queueing when app.status is changed
	pred := predicate.GenerationChangedPredicate{}
	return ctrl.NewControllerManagedBy(mgr).
		For(&ketchv1.App{}).
		WithEventFilter(pred).
		Complete(r)
}
