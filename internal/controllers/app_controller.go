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

// Package controllers contains App and Pool reconcilers to be used with controller-runtime.
package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/release"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
}

type helmFactoryFn func(namespace string) (Helm, error)

// Helm has methods to update/delete helm charts.
type Helm interface {
	UpdateChart(appChrt chart.ApplicationChart, config chart.ChartConfig, opts ...chart.InstallOption) (*release.Release, error)
	DeleteChart(appName string) error
}

// +kubebuilder:rbac:groups=theketch.io,resources=apps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=theketch.io,resources=apps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=theketch.io,resources=pools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=theketch.io,resources=pools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="networking.k8s.io",resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="networking.istio.io",resources=gateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="networking.istio.io",resources=virtualservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="cert-manager.io",resources=certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="extensions",resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="extensions",resources=ingresses,verbs=get;list;watch;create;update;patch;delete

func (r *AppReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("app", req.NamespacedName)

	app := ketchv1.App{}
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		if apierrors.IsNotFound(err) {
			err := r.deleteChart(ctx, req.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	app.Status = r.reconcile(ctx, &app)

	var (
		err    error
		result ctrl.Result
	)

	switch app.Status.Phase {
	case ketchv1.AppPending:
		result = ctrl.Result{
			Requeue:      true,
			RequeueAfter: time.Minute,
		}
		err = errors.New(app.Status.Message)
	case ketchv1.AppFailed:
		err = errors.New(app.Status.Message)
	case ketchv1.AppRunning:
	}

	if err := r.Status().Update(ctx, &app); err != nil {
		return result, err
	}
	return result, err
}

func (r *AppReconciler) reconcile(ctx context.Context, app *ketchv1.App) ketchv1.AppStatus {
	pool := ketchv1.Pool{}
	if err := r.Get(ctx, types.NamespacedName{Name: app.Spec.Pool}, &pool); err != nil {
		return ketchv1.AppStatus{
			Phase:   ketchv1.AppFailed,
			Message: fmt.Sprintf(`pool "%s" is not found`, app.Spec.Pool),
		}
	}
	ref, err := reference.GetReference(r.Scheme, &pool)
	if err != nil {
		return ketchv1.AppStatus{
			Phase:   ketchv1.AppFailed,
			Message: err.Error(),
		}
	}
	if pool.Status.Namespace == nil {
		return ketchv1.AppStatus{
			Phase:   ketchv1.AppFailed,
			Message: fmt.Sprintf(`pool "%s" is not linked to a kubernetes namespace`, pool.Name),
		}
	}
	tpls, err := r.TemplateReader.Get(app.TemplatesConfigMapName(pool.Spec.IngressController.IngressType))
	if err != nil {
		return ketchv1.AppStatus{
			Phase:   ketchv1.AppFailed,
			Message: fmt.Sprintf(`failed to read configmap with the app's chart templates: %v`, err),
		}
	}
	if !pool.HasApp(app.Name) && len(pool.Status.Apps) >= pool.Spec.AppQuotaLimit && pool.Spec.AppQuotaLimit != -1 {
		return ketchv1.AppStatus{
			Phase:   ketchv1.AppFailed,
			Message: fmt.Sprintf(`you have reached the limit of apps`),
		}
	}
	options := []chart.Option{
		chart.WithExposedPorts(app.ExposedPorts()),
		chart.WithTemplates(*tpls),
	}
	appChrt, err := chart.New(app, &pool, options...)
	if err != nil {
		return ketchv1.AppStatus{
			Phase:   ketchv1.AppFailed,
			Message: err.Error(),
		}
	}
	patchedPool := pool
	if !patchedPool.HasApp(app.Name) {
		patchedPool.Status.Apps = append(patchedPool.Status.Apps, app.Name)
		mergePatch := client.MergeFrom(&pool)
		if err := r.Status().Patch(ctx, &patchedPool, mergePatch); err != nil {
			return ketchv1.AppStatus{
				Phase:   ketchv1.AppPending,
				Message: fmt.Sprintf("failed to update pool status: %v", err),
			}
		}
	}
	version := fmt.Sprintf("v%v", app.ObjectMeta.Generation)
	chartVersion := fmt.Sprintf("v0.0.%v", app.ObjectMeta.Generation)
	if app.Spec.Version != nil {
		version = *app.Spec.Version
	}
	chartConfig := chart.ChartConfig{
		ChartVersion: chartVersion,
		AppName:      app.Name,
		AppVersion:   version,
	}

	targetNamespace := pool.Status.Namespace.Name
	helmClient, err := r.HelmFactoryFn(targetNamespace)
	if err != nil {
		return ketchv1.AppStatus{
			Phase:   ketchv1.AppPending,
			Message: err.Error(),
		}
	}
	_, err = helmClient.UpdateChart(*appChrt, chartConfig)
	if err != nil {
		return ketchv1.AppStatus{
			Phase:   ketchv1.AppPending,
			Message: fmt.Sprintf("failed to update helm chart: %v", err),
		}
	}
	return ketchv1.AppStatus{
		Pool:  ref,
		Phase: ketchv1.AppRunning,
	}
}

func (r *AppReconciler) deleteChart(ctx context.Context, appName string) error {
	pools := ketchv1.PoolList{}
	err := r.Client.List(ctx, &pools)
	if err != nil {
		return err
	}
	for _, pool := range pools.Items {
		if !pool.HasApp(appName) {
			continue
		}

		helmClient, err := r.HelmFactoryFn(pool.Spec.NamespaceName)
		if err != nil {
			return err
		}
		err = helmClient.DeleteChart(appName)
		if err != nil {
			return err
		}
		patchedPool := pool

		patchedPool.Status.Apps = make([]string, 0, len(patchedPool.Status.Apps))
		for _, name := range pool.Status.Apps {
			if name == appName {
				continue
			}
			patchedPool.Status.Apps = append(patchedPool.Status.Apps, name)
		}
		mergePatch := client.MergeFrom(&pool)
		return r.Status().Patch(ctx, &patchedPool, mergePatch)
	}
	return nil

}

func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ketchv1.App{}).
		Complete(r)
}
