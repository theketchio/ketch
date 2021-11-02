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
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/release"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/chart"
	"github.com/shipa-corp/ketch/internal/templates"
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
	Group string
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
	replicaDepRevision            = "deployment.kubernetes.io/revision"
	DeploymentProgressing         = "Progressing"
	deadlineExeceededProgressCond = "ProgressDeadlineExceeded"
	DefaultPodRunningTimeout      = 10 * time.Minute                 // TODO should this be configurable?
	maxWaitTimeDuration           = time.Duration(120) * time.Second // TODO should this be configurable?
	maxConcurrentReconciles       = 10

	appReconcileStarted  = "AppReconcileStarted"
	appReconcileComplete = "AppReconcileComplete"
	appReconcileUpdate   = "AppReconcileUpdate"
	appReconcileError    = "AppReconcileError"
)

// +kubebuilder:rbac:groups=theketch.io,resources=apps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=theketch.io,resources=apps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=theketch.io,resources=frameworks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=theketch.io,resources=frameworks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="networking.k8s.io",resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="networking.istio.io",resources=gateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="networking.istio.io",resources=virtualservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="networking.istio.io",resources=destinationrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="cert-manager.io",resources=certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="extensions",resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="extensions",resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="traefik.containo.us",resources=ingressroutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="traefik.containo.us",resources=ingressroutes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="traefik.containo.us",resources=traefikservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="traefik.containo.us",resources=traefikservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch;update;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;create;update

func (r *AppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("app", req.NamespacedName)

	app := ketchv1.App{}
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		if apierrors.IsNotFound(err) {
			err := r.deleteChart(ctx, req.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var (
		err    error
		result ctrl.Result
	)
	scheduleResult := r.reconcile(ctx, &app, logger)
	if scheduleResult.status == v1.ConditionFalse {
		// we have to return an error to run reconcile again.
		err = fmt.Errorf(scheduleResult.message)
		outcome := ketchv1.AppReconcileOutcome{AppName: app.Name, DeploymentCount: app.Spec.DeploymentsCount}
		r.Recorder.Event(&app, v1.EventTypeWarning, ketchv1.AppReconcileOutcomeReason, outcome.String(err))
	} else {
		app.Status.Framework = scheduleResult.framework
		outcome := ketchv1.AppReconcileOutcome{AppName: app.Name, DeploymentCount: app.Spec.DeploymentsCount}
		r.Recorder.Event(&app, v1.EventTypeNormal, ketchv1.AppReconcileOutcomeReason, outcome.String())
	}
	app.SetCondition(ketchv1.Scheduled, scheduleResult.status, scheduleResult.message, metav1.NewTime(time.Now()))
	if err := r.Status().Update(context.Background(), &app); err != nil {
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

type reconcileResult struct {
	status     v1.ConditionStatus
	message    string
	framework  *v1.ObjectReference
	useTimeout bool
}

func (r *AppReconciler) reconcile(ctx context.Context, app *ketchv1.App, logger logr.Logger) reconcileResult {
	framework := ketchv1.Framework{}
	if err := r.Get(ctx, types.NamespacedName{Name: app.Spec.Framework}, &framework); err != nil {
		return reconcileResult{
			status:  v1.ConditionFalse,
			message: fmt.Sprintf(`framework "%s" is not found`, app.Spec.Framework),
		}
	}
	ref, err := reference.GetReference(r.Scheme, &framework)
	if err != nil {
		return reconcileResult{
			status:  v1.ConditionFalse,
			message: err.Error(),
		}
	}
	if framework.Status.Namespace == nil {
		return reconcileResult{
			status:  v1.ConditionFalse,
			message: fmt.Sprintf(`framework "%s" is not linked to a kubernetes namespace`, framework.Name),
		}
	}
	tpls, err := r.TemplateReader.Get(templates.IngressConfigMapName(framework.Spec.IngressController.IngressType.String()))
	if err != nil {
		return reconcileResult{
			status:  v1.ConditionFalse,
			message: fmt.Sprintf(`failed to read configmap with the app's chart templates: %v`, err),
		}
	}
	if !framework.HasApp(app.Name) && framework.Spec.AppQuotaLimit != nil && len(framework.Status.Apps) >= *framework.Spec.AppQuotaLimit && *framework.Spec.AppQuotaLimit != -1 {
		return reconcileResult{
			status:  v1.ConditionFalse,
			message: fmt.Sprintf(`you have reached the limit of apps`),
		}
	}

	options := []chart.Option{
		chart.WithExposedPorts(app.ExposedPorts()),
		chart.WithTemplates(*tpls),
	}

	appChrt, err := chart.New(app, &framework, options...)
	if err != nil {
		return reconcileResult{
			status:  v1.ConditionFalse,
			message: err.Error(),
		}
	}
	patchedFramework := framework
	if !patchedFramework.HasApp(app.Name) {
		patchedFramework.Status.Apps = append(patchedFramework.Status.Apps, app.Name)
		mergePatch := client.MergeFrom(&framework)
		if err := r.Status().Patch(ctx, &patchedFramework, mergePatch); err != nil {
			return reconcileResult{
				status:  v1.ConditionFalse,
				message: fmt.Sprintf("failed to update framework status: %v", err),
			}
		}
	}
	targetNamespace := framework.Status.Namespace.Name
	helmClient, err := r.HelmFactoryFn(targetNamespace)
	if err != nil {
		return reconcileResult{
			status:  v1.ConditionFalse,
			message: err.Error(),
		}
	}

	// check for canary deployment
	if app.Spec.Canary.Active {
		// ensures that the canary deployment exists
		if len(app.Spec.Deployments) <= 1 {
			// reset canary specs
			app.Spec.Canary = ketchv1.CanarySpec{}

			return reconcileResult{
				status:     v1.ConditionFalse,
				message:    "no canary deployment found",
				useTimeout: true,
			}
		}

		// retry until all pods for canary deployment comes to running state.
		if err := checkPodStatus(r.Group, r.Client, app.Name, app.Spec.Deployments[1].Version); err != nil {

			if !timeoutExpired(app.Spec.Canary.Started, r.Now()) {
				return reconcileResult{
					status:     v1.ConditionFalse,
					message:    fmt.Sprintf("canary update failed: %v", err),
					useTimeout: true,
				}
			}

			// Do rollback if timeout expired
			app.DoRollback()
			if e := r.Update(ctx, app); err != nil {
				return reconcileResult{
					status:     v1.ConditionFalse,
					message:    fmt.Sprintf("failed to update app crd: %v", e),
					useTimeout: true,
				}
			}
		}

		// Once all pods are running then Perform canary deployment.
		if err = app.DoCanary(metav1.NewTime(r.Now()), logger, r.Recorder); err != nil {
			return reconcileResult{
				status:  v1.ConditionFalse,
				message: fmt.Sprintf("canary update failed: %v", err),
			}
		}
		if err := r.Update(ctx, app); err != nil {
			return reconcileResult{
				status:  v1.ConditionFalse,
				message: fmt.Sprintf("canary update failed: %v", err),
			}
		}
	}

	_, err = helmClient.UpdateChart(*appChrt, chart.NewChartConfig(*app))
	if err != nil {
		return reconcileResult{
			status:  v1.ConditionFalse,
			message: fmt.Sprintf("failed to update helm chart: %v", err),
		}
	}

	if len(app.Spec.Deployments) > 0 {
		// use latest deployment and watch events for each process
		latestDeployment := app.Spec.Deployments[len(app.Spec.Deployments)-1]
		for _, process := range latestDeployment.Processes {
			var dep appsv1.Deployment
			if err := r.Get(ctx, client.ObjectKey{
				Namespace: framework.Spec.NamespaceName,
				Name:      fmt.Sprintf("%s-%s-%d", app.GetName(), process.Name, latestDeployment.Version),
			}, &dep); err != nil {
				return reconcileResult{
					status:  v1.ConditionFalse,
					message: fmt.Sprintf("failed to get deployment: %v", err),
				}
			}
			err = r.watchDeployEvents(ctx, app, framework.Spec.NamespaceName, &dep, &process, r.Recorder)
			if err != nil {
				return reconcileResult{
					status:  v1.ConditionFalse,
					message: fmt.Sprintf("failed to get deploy events: %v", err),
				}
			}
		}
	}

	return reconcileResult{
		framework: ref,
		status:    v1.ConditionTrue,
	}
}

// watchDeployEvents watches a namespace for events and, after a deployment has started updating, records events
// with updated deployment status and/or healthcheck and timeout failures
func (r *AppReconciler) watchDeployEvents(ctx context.Context, app *ketchv1.App, namespace string, dep *appsv1.Deployment, process *ketchv1.ProcessSpec, recorder record.EventRecorder) error {
	config, err := GetRESTConfig()
	if err != nil {
		return err
	}
	cli, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	opts := metav1.ListOptions{
		FieldSelector: "involvedObject.kind=Pod",
		Watch:         true,
	}
	watcher, err := cli.CoreV1().Events(namespace).Watch(ctx, opts) // requires "watch" permission on events in clusterrole
	if err != nil {
		return err
	}
	defer watcher.Stop()

	return r.watchFunc(ctx, app, namespace, dep, process, recorder, watcher, cli)
}

func (r *AppReconciler) watchFunc(ctx context.Context, app *ketchv1.App, namespace string, dep *appsv1.Deployment, process *ketchv1.ProcessSpec, recorder record.EventRecorder, watcher watch.Interface, cli *kubernetes.Clientset) error {
	var err error
	watchCh := watcher.ResultChan()
	recorder.Eventf(app, v1.EventTypeNormal, appReconcileStarted, "Updating units [%s]", process.Name)

	timeout := time.After(DefaultPodRunningTimeout)
	for dep.Status.ObservedGeneration < dep.Generation {
		dep, err = cli.AppsV1().Deployments(namespace).Get(ctx, dep.Name, metav1.GetOptions{})
		if err != nil {
			recorder.Eventf(app, v1.EventTypeWarning, appReconcileError, "error getting deployments: %s", err.Error())
			return err
		}
		select {
		case <-time.After(100 * time.Millisecond):
		case <-timeout:
			recorder.Event(app, v1.EventTypeWarning, appReconcileError, "timeout waiting for deployment generation to update")
			return errors.Errorf("timeout waiting for deployment generation to update")
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	var specReplicas int32
	if dep.Spec.Replicas != nil {
		specReplicas = *dep.Spec.Replicas
	}
	oldUpdatedReplicas := int32(-1)
	oldReadyUnits := int32(-1)
	oldPendingTermination := int32(-1)
	now := time.Now()
	var healthcheckTimeout <-chan time.Time

	for {
		for i := range dep.Status.Conditions {
			c := dep.Status.Conditions[i]
			if c.Type == DeploymentProgressing && c.Reason == deadlineExeceededProgressCond {
				recorder.Eventf(app, v1.EventTypeWarning, appReconcileError, "deployment %q exceeded its progress deadline", dep.Name)
				return errors.Errorf("deployment %q exceeded its progress deadline", dep.Name)
			}
		}
		if oldUpdatedReplicas != dep.Status.UpdatedReplicas {
			recorder.Eventf(app, v1.EventTypeNormal, appReconcileUpdate, "%d of %d new units created", dep.Status.UpdatedReplicas, specReplicas)
		}

		if healthcheckTimeout == nil && dep.Status.UpdatedReplicas == specReplicas {
			err := checkPodStatus(r.Group, r.Client, app.Name, app.Spec.Deployments[len(app.Spec.Deployments)-1].Version)
			if err == nil {
				healthcheckTimeout = time.After(maxWaitTimeDuration)
				recorder.Eventf(app, v1.EventTypeNormal, appReconcileUpdate, "waiting healthcheck on %d created units", specReplicas)
			}
		}

		readyUnits := dep.Status.UpdatedReplicas - dep.Status.UnavailableReplicas
		if oldReadyUnits != readyUnits && readyUnits >= 0 {
			recorder.Eventf(app, v1.EventTypeNormal, appReconcileUpdate, "%d of %d new units ready", readyUnits, specReplicas)
		}

		pendingTermination := dep.Status.Replicas - dep.Status.UpdatedReplicas
		if oldPendingTermination != pendingTermination && pendingTermination > 0 {
			recorder.Eventf(app, v1.EventTypeNormal, appReconcileUpdate, "%d old units pending termination", pendingTermination)
		}

		oldUpdatedReplicas = dep.Status.UpdatedReplicas
		oldReadyUnits = readyUnits
		oldPendingTermination = pendingTermination
		if readyUnits == specReplicas &&
			dep.Status.Replicas == specReplicas {
			break
		}

		select {
		case <-time.After(100 * time.Millisecond):
		case msg, isOpen := <-watchCh:
			if !isOpen {
				break
			}
			if isDeploymentEvent(msg, dep) {
				recorder.Eventf(app, v1.EventTypeNormal, appReconcileUpdate, "%s", stringifyEvent(msg))
			}
		case <-healthcheckTimeout:
			err = createDeployTimeoutError(ctx, cli, app, time.Since(now), namespace, "healthcheck")
			recorder.Eventf(app, v1.EventTypeWarning, appReconcileError, "error waiting for healthcheck: %s", err.Error())
			return err
		case <-timeout:
			err = createDeployTimeoutError(ctx, cli, app, time.Since(now), namespace, "full rollout")
			recorder.Eventf(app, v1.EventTypeWarning, appReconcileError, "deployment timeout: %s", err.Error())
			return err
		case <-ctx.Done():
			return ctx.Err()
		}

		dep, err = cli.AppsV1().Deployments(namespace).Get(context.TODO(), dep.Name, metav1.GetOptions{})
		if err != nil {
			recorder.Eventf(app, v1.EventTypeWarning, appReconcileError, "error getting deployments: %s", err.Error())
			return err
		}
	}

	outcome := ketchv1.AppReconcileOutcome{AppName: app.Name, DeploymentCount: int(dep.Status.ReadyReplicas)}
	recorder.Event(app, v1.EventTypeNormal, appReconcileUpdate, outcome.String())
	return nil
}

// stringifyEvent accepts an event and returns relevant details as a string
func stringifyEvent(watchEvent watch.Event) string {
	event, ok := watchEvent.Object.(*v1.Event)
	if !ok {
		return ""
	}
	var subStr string
	if event.InvolvedObject.FieldPath != "" {
		subStr = fmt.Sprintf(" - %s", event.InvolvedObject.FieldPath)
	}
	component := []string{event.Source.Component}
	if event.Source.Host != "" {
		component = append(component, event.Source.Host)
	}
	message := fmt.Sprintf("%s%s - %s [%s]",
		event.InvolvedObject.Name,
		subStr,
		event.Message,
		strings.Join(component, ", "),
	)
	return message
}

// isDeploymentEvent returns true if the watchEvnet is an Event type and matches the deployment.Name
func isDeploymentEvent(msg watch.Event, dep *appsv1.Deployment) bool {
	evt, ok := msg.Object.(*v1.Event)
	return ok && strings.HasPrefix(evt.Name, dep.Name)
}

// createDeployTimeoutError gets pods that are not status == ready aggregates and returns the pod phase errors
func createDeployTimeoutError(ctx context.Context, cli *kubernetes.Clientset, app *ketchv1.App, timeout time.Duration, namespace, label string) error {
	opts := metav1.ListOptions{
		FieldSelector: "involvedObject.kind=Pod",
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
func newInvalidPodPhaseError(ctx context.Context, cli *kubernetes.Clientset, pod *v1.Pod, namespace string) error {
	phaseWithMsg := fmt.Sprintf("%q", pod.Status.Phase)
	if pod.Status.Message != "" {
		phaseWithMsg = fmt.Sprintf("%s(%q)", phaseWithMsg, pod.Status.Message)
	}
	retErr := errors.Errorf("invalid pod phase %s", phaseWithMsg)
	eventsInterface := cli.CoreV1().Events(namespace)
	selector := eventsInterface.GetFieldSelector(&pod.Name, &namespace, nil, nil)
	options := metav1.ListOptions{FieldSelector: selector.String()}
	events, err := eventsInterface.List(context.TODO(), options)
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
func checkPodStatus(group string, c client.Client, appName string, depVersion ketchv1.DeploymentVersion) error {
	if c == nil {
		return errors.New("client must be non-nil")
	}

	if len(appName) == 0 || depVersion <= 0 {
		return errors.New("invalid app specifications")
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
		return err
	}

	// check if all pods are running for the deployment
	for _, pod := range podList.Items {
		// check if pod have voluntarily terminated with a container exit code of 0
		if pod.Status.Phase == v1.PodSucceeded {
			return nil
		}

		if pod.Status.Phase != v1.PodRunning {
			return errors.New("all pods are not running")
		}

		for _, c := range pod.Status.Conditions {
			if c.Status != v1.ConditionTrue {
				return errors.New("all pods are not in healthy state")
			}
		}
	}
	return nil
}

func (r *AppReconciler) deleteChart(ctx context.Context, appName string) error {
	frameworks := ketchv1.FrameworkList{}
	err := r.Client.List(ctx, &frameworks)
	if err != nil {
		return err
	}
	for _, framework := range frameworks.Items {
		if !framework.HasApp(appName) {
			continue
		}

		helmClient, err := r.HelmFactoryFn(framework.Spec.NamespaceName)
		if err != nil {
			return err
		}
		err = helmClient.DeleteChart(appName)
		if err != nil {
			return err
		}
		patchedFramework := framework

		patchedFramework.Status.Apps = make([]string, 0, len(patchedFramework.Status.Apps))
		for _, name := range framework.Status.Apps {
			if name == appName {
				continue
			}
			patchedFramework.Status.Apps = append(patchedFramework.Status.Apps, name)
		}
		mergePatch := client.MergeFrom(&framework)
		return r.Status().Patch(ctx, &patchedFramework, mergePatch)
	}
	return nil

}

func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ketchv1.App{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}).
		Complete(r)
}
