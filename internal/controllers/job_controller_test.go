package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1"
)

func TestJobReconciler_Reconcile(t *testing.T) {
	defaultObjects := []client.Object{}
	helmMock := &helm{
		updateChartResults: map[string]error{
			"app-update-chart-failed": errors.New("render error"),
		},
	}
	readerMock := &templateReader{
		templatesErrors: map[string]error{
			"templates-failed": errors.New("no templates"),
		},
	}
	ctx, err := setup(readerMock, helmMock, defaultObjects)
	require.Nil(t, err)
	require.NotNil(t, ctx)
	defer teardown(ctx)

	tests := []struct {
		name                 string
		want                 ctrl.Result
		job                  ketchv1.Job
		wantConditionStatus  v1.ConditionStatus
		wantConditionMessage string
	}{
		{
			name: "running job",
			job: ketchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job",
					Namespace: "default",
				},
				Spec: ketchv1.JobSpec{
					Namespace: "working-namespace",
				},
			},
			wantConditionStatus: v1.ConditionTrue,
		},
		{
			name: "cronjob",
			job: ketchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cron-job",
					Namespace: "default",
				},
				Spec: ketchv1.JobSpec{
					Namespace: "default",
					Schedule:  "* * * * *",
				},
			},
			wantConditionStatus: v1.ConditionTrue,
		},
		{
			name: "running job, delete it but keep its helm chart",
			job: ketchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job-2",
					Namespace: "default",
					Annotations: map[string]string{
						"theketch.io/dont-uninstall-helm-chart": "true",
					},
				},
				Spec: ketchv1.JobSpec{
					Namespace: "working-namespace",
				},
			},
			wantConditionStatus: v1.ConditionTrue,
		},
		{
			name: "job not linked to namespace",
			job: ketchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-namespace",
					Namespace: "default",
				},
				Spec: ketchv1.JobSpec{},
			},
			wantConditionStatus:  v1.ConditionFalse,
			wantConditionMessage: `job "no-namespace" is not linked to a kubernetes namespace`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ctx.k8sClient.Create(context.Background(), &tt.job)
			require.Nil(t, err)
			resultJob := ketchv1.Job{}
			for {
				time.Sleep(250 * time.Millisecond)
				err = ctx.k8sClient.Get(context.Background(), types.NamespacedName{Name: tt.job.Name, Namespace: tt.job.Namespace}, &resultJob)
				require.Nil(t, err)
				if len(resultJob.Status.Conditions) > 0 {
					break
				}
			}
			condition := resultJob.Status.Condition(ketchv1.Scheduled)
			require.Equal(t, tt.wantConditionStatus, condition.Status)
			require.Equal(t, tt.wantConditionMessage, condition.Message)
			require.True(t, controllerutil.ContainsFinalizer(&resultJob, ketchv1.KetchFinalizer))
			require.Equal(t, tt.job.Spec.Schedule, resultJob.Spec.Schedule)
			if condition.Status == v1.ConditionTrue {
				err = ctx.k8sClient.Delete(context.Background(), &resultJob)
				require.Nil(t, err)
			}
		})
	}
	require.Equal(t, []string{"test-job", "test-cron-job"}, helmMock.deleteChartCalled)
}
