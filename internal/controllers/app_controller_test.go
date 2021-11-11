package controllers

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/release"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"

	clientFake "k8s.io/client-go/kubernetes/fake"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlFake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"
	"github.com/theketchio/ketch/internal/chart"
	"github.com/theketchio/ketch/internal/templates"
	"github.com/theketchio/ketch/internal/utils/conversions"
)

type templateReader struct {
	templatesErrors map[string]error
}

func (t *templateReader) Get(name string) (*templates.Templates, error) {
	err := t.templatesErrors[name]
	if err != nil {
		return nil, err
	}
	return &templates.Templates{}, nil
}

type helm struct {
	updateChartResults map[string]error
	deleteChartCalled  []string
}

func (h *helm) UpdateChart(tv chart.TemplateValuer, config chart.ChartConfig, opts ...chart.InstallOption) (*release.Release, error) {
	return nil, h.updateChartResults[tv.GetName()]
}

func (h *helm) DeleteChart(appName string) error {
	h.deleteChartCalled = append(h.deleteChartCalled, appName)
	return nil
}

func TestAppReconciler_Reconcile(t *testing.T) {

	defaultObjects := []client.Object{
		&ketchv1.Framework{
			ObjectMeta: metav1.ObjectMeta{
				Name: "working-framework",
			},
			Spec: ketchv1.FrameworkSpec{
				NamespaceName: "hello",
				AppQuotaLimit: conversions.IntPtr(100),
				IngressController: ketchv1.IngressControllerSpec{
					IngressType: ketchv1.IstioIngressControllerType,
				},
			},
		},
		&ketchv1.Framework{
			ObjectMeta: metav1.ObjectMeta{
				Name: "second-framework",
			},
			Spec: ketchv1.FrameworkSpec{
				NamespaceName: "second-namespace",
				AppQuotaLimit: conversions.IntPtr(1),
				IngressController: ketchv1.IngressControllerSpec{
					IngressType: ketchv1.IstioIngressControllerType,
				},
			},
		},
		&ketchv1.App{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default-app",
			},
			Spec: ketchv1.AppSpec{
				Deployments: []ketchv1.AppDeploymentSpec{},
				Framework:   "second-framework",
			},
		},
	}
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
		app                  ketchv1.App
		wantConditionStatus  v1.ConditionStatus
		wantConditionMessage string
	}{
		{
			name: "app linked to nonexisting framework",
			app: ketchv1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-1",
				},
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
					Framework:   "non-existing-framework",
				},
			},
			wantConditionStatus:  v1.ConditionFalse,
			wantConditionMessage: `framework "non-existing-framework" is not found`,
		},
		{
			name: "running application",
			app: ketchv1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-running",
				},
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
					Framework:   "working-framework",
				},
			},
			wantConditionStatus: v1.ConditionTrue,
		},
		{
			name: "create an app linked to a framework without available slots to run the app",
			app: ketchv1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-3",
				},
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
					Framework:   "second-framework",
				},
			},
			wantConditionStatus:  v1.ConditionFalse,
			wantConditionMessage: "you have reached the limit of apps",
		},
		{
			name: "app with update-chart-error",
			app: ketchv1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-update-chart-failed",
				},
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
					Framework:   "working-framework",
				},
			},
			wantConditionStatus:  v1.ConditionFalse,
			wantConditionMessage: "failed to update helm chart: render error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ctx.k8sClient.Create(context.TODO(), &tt.app)
			assert.Nil(t, err)

			resultApp := ketchv1.App{}
			for {
				time.Sleep(250 * time.Millisecond)
				err = ctx.k8sClient.Get(context.TODO(), types.NamespacedName{Name: tt.app.Name}, &resultApp)
				assert.Nil(t, err)
				if len(resultApp.Status.Conditions) > 0 {
					break
				}
			}
			condition := resultApp.Status.Condition(ketchv1.Scheduled)
			require.NotNil(t, condition)
			require.Equal(t, tt.wantConditionStatus, condition.Status)
			assert.Equal(t, tt.wantConditionMessage, condition.Message)

			if condition.Status == v1.ConditionTrue {
				err = ctx.k8sClient.Delete(context.TODO(), &resultApp)
			}
		})
	}
	assert.Equal(t, []string{"app-running"}, helmMock.deleteChartCalled)
}

func TestWatchDeployEvents(t *testing.T) {
	process := &ketchv1.ProcessSpec{}

	app := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: ketchv1.AppSpec{
			Deployments: []ketchv1.AppDeploymentSpec{
				{
					Image:     "gcr.io/test",
					Version:   1,
					Processes: []ketchv1.ProcessSpec{*process},
				},
			},
			Framework: "test",
		},
	}
	namespace := "ketch-test"
	replicas := int32(1)

	// depStart is the Deployment in it's initial state
	depStart := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "ketch-test",
		},
		Status: appsv1.DeploymentStatus{
			UpdatedReplicas:     1,
			UnavailableReplicas: 1,
			Replicas:            1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}

	// depFetch is the Deployment as returned via Get() in the function's loop
	depFetch := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "ketch-test",
		},
		Status: appsv1.DeploymentStatus{
			UpdatedReplicas:     1,
			UnavailableReplicas: 0,
			Replicas:            1,
			ReadyReplicas:       1,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}

	recorder := record.NewFakeRecorder(1024)
	watcher := watch.NewFake()
	cli := clientFake.NewSimpleClientset(depFetch)
	timeout := time.After(DefaultPodRunningTimeout)
	r := AppReconciler{}
	ctx := context.Background()

	var events []string
	go func() {
		for ev := range recorder.Events {
			events = append(events, ev)
		}
	}()

	err := r.watchFunc(ctx, app, namespace, depStart, process, recorder, watcher, cli, timeout, func() {})
	require.Nil(t, err)

	time.Sleep(time.Millisecond * 100)

	expectedEvents := []string{
		"Normal AppReconcileStarted Updating units []",
		"Normal AppReconcileUpdate 1 of 1 new units created",
		"Normal AppReconcileUpdate 0 of 1 new units ready",
		"Normal AppReconcileUpdate 1 of 1 new units ready",
		"Normal AppReconcileComplete app test 1 reconcile success",
	}

EXPECTED:
	for _, expected := range expectedEvents {
		for _, ev := range events {
			if ev == expected {
				continue EXPECTED
			}
		}
		t.Errorf("expected event %s, but it was not found", expected)
	}
}

func Test_checkPodStatus(t *testing.T) {
	createPod := func(group, appName, version string, status v1.PodStatus) *v1.Pod {
		return &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%v-%v", appName, version),
				Namespace: "default",
				Labels: map[string]string{
					group + "/app-name":               appName,
					group + "/app-deployment-version": version,
				},
			},
			Status: status,
		}
	}
	tests := []struct {
		name       string
		pods       []runtime.Object
		appName    string
		depVersion ketchv1.DeploymentVersion
		group      string
		wantErr    string
	}{
		{
			name:       "pod in Pending state",
			appName:    "my-app",
			depVersion: 5,
			group:      "theketch.io",
			pods: []runtime.Object{
				createPod("theketch.io", "my-app", "5", v1.PodStatus{Phase: v1.PodPending}),
			},
			wantErr: `all pods are not running`,
		},
		{
			name:       "pod in Pending state but group doesn't match",
			appName:    "my-app",
			depVersion: 5,
			group:      "ketch.io",
			pods: []runtime.Object{
				createPod("theketch.io", "my-app", "5", v1.PodStatus{Phase: v1.PodPending}),
			},
			wantErr: ``,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := ctrlFake.NewClientBuilder().WithScheme(clientgoscheme.Scheme).WithRuntimeObjects(tt.pods...).Build()
			err := checkPodStatus(tt.group, cli, tt.appName, tt.depVersion)
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.Nil(t, err)
		})
	}
}

func TestAppDeloymentEventFromWatchEvent(t *testing.T) {
	app := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default-app",
		},
		Spec: ketchv1.AppSpec{
			Deployments: []ketchv1.AppDeploymentSpec{
				{
					Version: 2,
				},
			},
		},
	}
	tests := []struct {
		description string
		obj         watch.Event
		expected    *AppDeploymentEvent
	}{
		{
			description: "success",
			obj: watch.Event{
				Object: &v1.Event{
					Reason:  "test reason",
					Message: "test message",
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-event",
					},
					InvolvedObject: v1.ObjectReference{
						Name:      "test name",
						FieldPath: "test/fieldpath",
					},
					Source: v1.EventSource{
						Host:      "testhost",
						Component: "testcomponent",
					},
				},
			},
			expected: &AppDeploymentEvent{
				Name:              app.Name,
				DeploymentVersion: 2,
				Reason:            "test reason",
				Description:       "test message",
				Annotations: map[string]string{
					DeploymentAnnotationAppName:                 app.Name,
					DeploymentAnnotationDevelopmentVersion:      "2",
					DeploymentAnnotationEventName:               "test reason",
					DeploymentAnnotationDescription:             "test message",
					DeploymentAnnotationInvolvedObjectName:      "test name",
					DeploymentAnnotationInvolvedObjectFieldPath: "test/fieldpath",
					DeploymentAnnotationSourceHost:              "testhost",
					DeploymentAnnotationSourceComponent:         "testcomponent",
				},
			},
		},
		{
			description: "no event",
			obj: watch.Event{
				Object: &v1.Pod{},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			ev := appDeploymentEventFromWatchEvent(tc.obj, app)
			require.Equal(t, tc.expected, ev)
		})
	}
}

func TestAppDeloymentEvent(t *testing.T) {
	app := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default-app",
		},
		Spec: ketchv1.AppSpec{
			Deployments: []ketchv1.AppDeploymentSpec{
				{
					Version: 2,
				},
			},
		},
	}
	tests := []struct {
		reason   string
		desc     string
		expected *AppDeploymentEvent
	}{
		{
			reason: "test reason",
			desc:   "test message",
			expected: &AppDeploymentEvent{
				Name:              app.Name,
				DeploymentVersion: 2,
				Reason:            "test reason",
				Description:       "test message",
				Annotations: map[string]string{
					DeploymentAnnotationAppName:            app.Name,
					DeploymentAnnotationDevelopmentVersion: "2",
					DeploymentAnnotationEventName:          "test reason",
					DeploymentAnnotationDescription:        "test message",
				},
			},
		},
	}
	for i, tc := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			ev := newAppDeploymentEvent(app, tc.reason, tc.desc)
			require.Equal(t, tc.expected, ev)
		})
	}
}

func TestIsDeploymentEvent(t *testing.T) {
	tests := []struct {
		msg      watch.Event
		expected bool
	}{
		{
			msg: watch.Event{
				Object: &v1.Event{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
			expected: true,
		},
		{
			msg: watch.Event{
				Object: &v1.Event{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bad-name",
					},
				},
			},
			expected: false,
		},
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			res := isDeploymentEvent(tc.msg, dep)
			require.Equal(t, tc.expected, res)
		})
	}
}
