/*
Copyright 2021.

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

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"
	"github.com/theketchio/ketch/internal/chart"
	"github.com/theketchio/ketch/internal/templates"
)

// JobReconciler reconciles a Job object
type JobReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	HelmFactoryFn  helmFactoryFn
	Recorder       record.EventRecorder
	TemplateReader templates.Reader
}

// JobReconcileReason contains information about job reconcile
type JobReconcileReason struct {
	JobName string
}

// +kubebuilder:rbac:groups=resources.resources,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=resources.resources,resources=jobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=resources.resources,resources=jobs/finalizers,verbs=update
// +kubebuilder:rbac:groups=theketch.io,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=theketch.io,resources=jobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile fetches a Job by name and updates helm charts with differences
func (r *JobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("job", req.NamespacedName)

	var job ketchv1.Job
	if err := r.Get(ctx, req.NamespacedName, &job); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !controllerutil.ContainsFinalizer(&job, ketchv1.KetchFinalizer) {
		controllerutil.AddFinalizer(&job, ketchv1.KetchFinalizer)
		if err := r.Update(ctx, &job); err != nil {
			logger.Error(err, "failed to add ketch finalizer")
			return ctrl.Result{}, err
		}
	}

	if !job.ObjectMeta.DeletionTimestamp.IsZero() {
		err := r.deleteChart(ctx, &job)
		return ctrl.Result{}, err
	}

	var err error
	scheduleResult := r.reconcile(ctx, &job)
	if scheduleResult.status == v1.ConditionFalse {
		// we have to return an error to run reconcile again.
		err = fmt.Errorf(scheduleResult.message)
		reason := JobReconcileReason{JobName: job.Name}
		r.Recorder.Event(&job, v1.EventTypeWarning, reason.String(), err.Error())
	} else {
		job.Status.Framework = scheduleResult.framework
		reason := JobReconcileReason{JobName: job.Name}
		r.Recorder.Event(&job, v1.EventTypeNormal, reason.String(), "success")
	}
	job.SetCondition(ketchv1.Scheduled, scheduleResult.status, scheduleResult.message, metav1.NewTime(time.Now()))
	if err := r.Status().Update(context.Background(), &job); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *JobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ketchv1.Job{}).
		Complete(r)
}

type reconcileResult struct {
	status    v1.ConditionStatus
	message   string
	framework *v1.ObjectReference
}

func (r *JobReconciler) reconcile(ctx context.Context, job *ketchv1.Job) reconcileResult {
	framework := ketchv1.Framework{}
	if err := r.Get(ctx, types.NamespacedName{Name: job.Spec.Framework}, &framework); err != nil {
		return reconcileResult{
			status:  v1.ConditionFalse,
			message: fmt.Sprintf(`framework "%s" is not found`, job.Spec.Framework),
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
	tpls, err := r.TemplateReader.Get(templates.JobConfigMapName())
	if err != nil {
		return reconcileResult{
			status:  v1.ConditionFalse,
			message: fmt.Sprintf(`failed to read configmap with the app's chart templates: %v`, err),
		}
	}

	options := []chart.Option{
		chart.WithTemplates(*tpls),
	}

	jobChartConfig := chart.NewJobChartConfig(*job)
	jobChart := chart.NewJobChart(job, options...)

	patchedFramework := framework
	if !patchedFramework.HasJob(job.Name) {
		patchedFramework.Status.Jobs = append(patchedFramework.Status.Jobs, job.Name)
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

	_, err = helmClient.UpdateChart(jobChart, jobChartConfig)
	if err != nil {
		return reconcileResult{
			status:  v1.ConditionFalse,
			message: fmt.Sprintf("failed to update helm chart: %v", err),
		}
	}

	return reconcileResult{
		framework: ref,
		status:    v1.ConditionTrue,
	}
}

func (r *JobReconciler) deleteChart(ctx context.Context, job *ketchv1.Job) error {
	frameworks := ketchv1.FrameworkList{}
	err := r.Client.List(ctx, &frameworks)
	if err != nil {
		return err
	}
	for _, framework := range frameworks.Items {
		if !framework.HasJob(job.Name) {
			continue
		}

		if uninstallHelmChart(ketchv1.Group, job.Annotations) {
			helmClient, err := r.HelmFactoryFn(framework.Spec.NamespaceName)
			if err != nil {
				return err
			}
			err = helmClient.DeleteChart(job.Name)
			if err != nil {
				return err
			}
		}
		patchedFramework := framework

		patchedFramework.Status.Jobs = make([]string, 0, len(patchedFramework.Status.Jobs))
		for _, name := range framework.Status.Jobs {
			if name == job.Name {
				continue
			}
			patchedFramework.Status.Jobs = append(patchedFramework.Status.Jobs, name)
		}
		mergePatch := client.MergeFrom(&framework)
		if err := r.Status().Patch(ctx, &patchedFramework, mergePatch); err != nil {
			return err
		}
		break
	}
	controllerutil.RemoveFinalizer(job, ketchv1.KetchFinalizer)
	if err := r.Update(ctx, job); err != nil {
		return err
	}
	return nil
}

// String is a Stringer interface implementation
func (r *JobReconcileReason) String() string {
	return r.JobName
}
