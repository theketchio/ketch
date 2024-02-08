package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/release"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	clientFake "k8s.io/client-go/kubernetes/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clientTest "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"
	"github.com/theketchio/ketch/internal/chart"
	"github.com/theketchio/ketch/internal/templates"
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

type watchReactor struct {
	action  clientTest.Action
	watcher watch.Interface
	err     error
}

func (w *watchReactor) Handles(action clientTest.Action) bool {
	return true
}
func (w *watchReactor) React(action clientTest.Action) (bool, watch.Interface, error) {
	return true, w.watcher, w.err
}

func TestAppReconciler_Reconcile(t *testing.T) {

	defaultObjects := []client.Object{
		&ketchv1.App{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default-app",
			},
			Spec: ketchv1.AppSpec{
				Deployments: []ketchv1.AppDeploymentSpec{},
				Namespace:   "second-namespace",
				Ingress:     ketchv1.IngressSpec{Controller: ketchv1.IngressControllerSpec{IngressType: ketchv1.TraefikIngressControllerType}},
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
			name: "app not linked to a namespace",
			app: ketchv1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-1",
				},
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
					Ingress:     ketchv1.IngressSpec{Controller: ketchv1.IngressControllerSpec{IngressType: ketchv1.TraefikIngressControllerType}},
				},
			},
			wantConditionStatus:  v1.ConditionFalse,
			wantConditionMessage: `app "app-1" must be linked to a kubernetes namespace`,
		},
		{
			name: "running application",
			app: ketchv1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-running",
				},
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
					Namespace:   "working-namespace",
					Ingress:     ketchv1.IngressSpec{Controller: ketchv1.IngressControllerSpec{IngressType: ketchv1.TraefikIngressControllerType}},
				},
			},
			wantConditionStatus: v1.ConditionTrue,
		},
		{
			name: "running application, delete it but keep its helm chart",
			app: ketchv1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-running-with-dont-uninstall-annotation",
					Annotations: map[string]string{
						"theketch.io/dont-uninstall-helm-chart": "true",
					},
				},
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
					Namespace:   "working-namespace",
					Ingress:     ketchv1.IngressSpec{Controller: ketchv1.IngressControllerSpec{IngressType: ketchv1.TraefikIngressControllerType}},
				},
			},
			wantConditionStatus: v1.ConditionTrue,
		},
		{
			name: "app with update-chart-error",
			app: ketchv1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-update-chart-failed",
				},
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
					Namespace:   "working-namespace",
					Ingress:     ketchv1.IngressSpec{Controller: ketchv1.IngressControllerSpec{IngressType: ketchv1.TraefikIngressControllerType}},
				},
			},
			wantConditionStatus:  v1.ConditionFalse,
			wantConditionMessage: "failed to update helm chart: render error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ctx.k8sClient.Create(context.Background(), &tt.app)
			require.Nil(t, err)

			resultApp := ketchv1.App{}
			for {
				time.Sleep(250 * time.Millisecond)
				err = ctx.k8sClient.Get(context.Background(), types.NamespacedName{Name: tt.app.Name}, &resultApp)
				require.Nil(t, err)
				if len(resultApp.Status.Conditions) > 0 {
					break
				}
			}
			condition := resultApp.Status.Condition(ketchv1.Scheduled)
			require.NotNil(t, condition)
			//require.Equal(t, tt.wantConditionStatus, condition.Status)
			require.Equal(t, tt.wantConditionMessage, condition.Message)
			require.True(t, controllerutil.ContainsFinalizer(&resultApp, ketchv1.KetchFinalizer))

			if condition.Status == v1.ConditionTrue {
				err = ctx.k8sClient.Delete(context.Background(), &resultApp)
				require.Nil(t, err)
			}
		})
	}
	require.Equal(t, []string{"app-running"}, helmMock.deleteChartCalled)
}

func TestWatchDeployEventsError(t *testing.T) {
	process := &ketchv1.ProcessSpec{
		Name: "test",
	}

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
			Namespace: "test",
		},
	}
	cli := clientFake.NewSimpleClientset()
	watchReaction := &watchReactor{
		err: fmt.Errorf("unknown (get events)"),
	}
	cli.WatchReactionChain = []clientTest.WatchReactor{watchReaction}
	wl := &workload{
		Name:            "test",
		UpdatedReplicas: 1,
		ReadyReplicas:   0,
		Replicas:        1,
	}
	wc := &workloadClient{
		k8sClient: cli,
	}
	recorder := record.NewFakeRecorder(1024)
	r := AppReconciler{
		CancelMap: NewCancelMap(),
	}
	var events []string
	go func() {
		for ev := range recorder.Events {
			events = append(events, ev)
		}
	}()
	err := r.watchDeployEvents(context.Background(), app, wc, wl, process, recorder)
	require.EqualError(t, err, "assure clusterrole 'manager-role' has 'watch' permissions on event resources: unknown (get events)")
	time.Sleep(time.Millisecond * 100) // give events time to propagate
	require.Equal(t, 1, len(events))
	require.Contains(t, events[0], "Warning AppReconcileError error watching deployments for workload test: assure clusterrole 'manager-role' has 'watch' permissions on event resources: unknown (get events)")
}

func TestWatchDeployEvents(t *testing.T) {
	tt := []struct {
		name          string
		appType       ketchv1.AppType
		expectedError string
	}{
		{
			name:    "watch deploy events - deployment",
			appType: "Deployment",
		},
		{
			name:    "watch deploy events - statefulset",
			appType: "StatefulSet",
		},
		{
			name:          "unknown type",
			appType:       "TypeThatDoesNotExist",
			expectedError: "unknown workload type",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			process := &ketchv1.ProcessSpec{
				Name: "test",
			}

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
					Namespace: "test",
				},
			}
			namespace := "ketch-test"
			replicas := int32(1)

			// wl initial state
			wl := workload{
				Name:            "test",
				UpdatedReplicas: 1,
				ReadyReplicas:   0,
				Replicas:        1,
			}

			var cli *clientFake.Clientset
			// details returned via Get() in the function's loop
			if tc.appType == "Deployment" {
				cli = clientFake.NewSimpleClientset(&appsv1.Deployment{
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
				})
			} else {
				cli = clientFake.NewSimpleClientset(&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "ketch-test",
					},
					Status: appsv1.StatefulSetStatus{
						UpdatedReplicas: 1,
						Replicas:        1,
						ReadyReplicas:   1,
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: &replicas,
					},
				})
			}

			recorder := record.NewFakeRecorder(1024)
			watcher := watch.NewFake()
			timeout := time.After(DefaultPodRunningTimeout)
			r := AppReconciler{}
			ctx := context.Background()

			var events []string
			go func() {
				for ev := range recorder.Events {
					events = append(events, ev)
				}
			}()

			cleanupIsCalled := false
			cleanupFn := func() {
				cleanupIsCalled = true
			}

			wc := workloadClient{
				k8sClient:         cli,
				workloadNamespace: namespace,
				workloadType:      tc.appType,
				workloadName:      "test",
			}

			err := r.watchFunc(ctx, cleanupFn, app, process.Name, recorder, watcher, &wc, &wl, timeout)
			if tc.expectedError != "" {
				require.Equal(t, tc.expectedError, err.Error())
				return
			}
			require.Nil(t, err)

			time.Sleep(time.Millisecond * 100)

			expectedEvents := []string{
				"Normal AppReconcileUpdate 1 of 1 new units created",
				"Normal AppReconcileUpdate 0 of 1 new units ready",
				"Normal AppReconcileUpdate 1 of 1 new units ready",
				"Normal AppReconcileComplete app test 1 reconcile success",
			}

		EXPECTED:
			for _, expected := range expectedEvents {
				for _, ev := range events {
					if strings.HasPrefix(ev, expected) {
						continue EXPECTED
					}
				}
				t.Errorf("expected event %s, but it was not found", expected)
			}

			require.True(t, cleanupIsCalled)
		})
	}
}

func TestCancelWatchDeployEvents(t *testing.T) {
	tt := []struct {
		name    string
		appType ketchv1.AppType
	}{
		{
			name:    "cancel watch deploy events - deployment",
			appType: "Deployment",
		},
		{
			name:    "cancel watch deploy events - statefulset",
			appType: "StatefulSet",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			process := &ketchv1.ProcessSpec{
				Name: "test",
			}

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
					Namespace: "test",
				},
			}
			namespace := "ketch-test"
			replicas := int32(1)

			// wl initial state
			wl := workload{
				Name:            "test",
				UpdatedReplicas: 1,
				ReadyReplicas:   0,
				Replicas:        1,
			}

			var cli *clientFake.Clientset
			// details returned via Get() in the function's loop
			if tc.appType == "Deployment" {
				cli = clientFake.NewSimpleClientset(&appsv1.Deployment{
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
				})
			} else {
				cli = clientFake.NewSimpleClientset(&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "ketch-test",
					},
					Status: appsv1.StatefulSetStatus{
						UpdatedReplicas: 1,
						Replicas:        1,
						ReadyReplicas:   1,
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: &replicas,
					},
				})
			}

			recorder := record.NewFakeRecorder(1024)
			watcher := watch.NewFake()
			timeout := time.After(DefaultPodRunningTimeout)
			r := AppReconciler{}

			ctx, cancel := context.WithCancel(context.Background())

			var events []string
			go func() {
				for ev := range recorder.Events {
					cancel() // cancel context after first event received
					events = append(events, ev)
				}
			}()

			wc := workloadClient{
				k8sClient:         cli,
				workloadNamespace: namespace,
				workloadType:      tc.appType,
				workloadName:      "test",
			}

			err := r.watchFunc(ctx, func() {}, app, process.Name, recorder, watcher, &wc, &wl, timeout)
			require.EqualError(t, err, "context canceled")

			// assert that watchFunc() ended early via context cancelation and that not all events were processed.
			allPossibleEvents := []string{
				"Normal AppReconcileUpdate 1 of 1 new units created",
				"Normal AppReconcileUpdate 0 of 1 new units ready",
				"Normal AppReconcileUpdate 1 of 1 new units ready",
				"Normal AppReconcileComplete app test 1 reconcile success",
			}
			require.True(t, len(events) < len(allPossibleEvents))
		})
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
		name        string
		pods        []runtime.Object
		appName     string
		depVersion  ketchv1.DeploymentVersion
		group       string
		expectedPod string
		wantErr     string
	}{
		{
			name:       "pod in Pending state",
			appName:    "my-app",
			depVersion: 5,
			group:      "theketch.io",
			pods: []runtime.Object{
				createPod("theketch.io", "my-app", "5", v1.PodStatus{Phase: v1.PodPending}),
			},
			expectedPod: "my-app-5",
			wantErr:     `not all pods are running`,
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
			podName, err := checkPodStatus(tt.group, cli, tt.appName, tt.depVersion)
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.Equal(t, podName, tt.expectedPod)
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
		processName string
		expected    *ketchv1.AppDeploymentEvent
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
			processName: "test process",
			expected: &ketchv1.AppDeploymentEvent{
				Name:              app.Name,
				DeploymentVersion: 2,
				Reason:            "test reason",
				Description:       "test message",
				ProcessName:       "test process",
				Annotations: map[string]string{
					ketchv1.DeploymentAnnotationAppName:                 app.Name,
					ketchv1.DeploymentAnnotationDevelopmentVersion:      "2",
					ketchv1.DeploymentAnnotationEventName:               "test reason",
					ketchv1.DeploymentAnnotationDescription:             "test message",
					ketchv1.DeploymentAnnotationProcessName:             "test process",
					ketchv1.DeploymentAnnotationInvolvedObjectName:      "test name",
					ketchv1.DeploymentAnnotationInvolvedObjectFieldPath: "test/fieldpath",
					ketchv1.DeploymentAnnotationSourceHost:              "testhost",
					ketchv1.DeploymentAnnotationSourceComponent:         "testcomponent",
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
			ev := appDeploymentEventFromWatchEvent(tc.obj, app, tc.processName)
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
		reason      string
		desc        string
		processName string
		podName     string
		expected    *ketchv1.AppDeploymentEvent
	}{
		{
			reason:      "test reason",
			desc:        "test message",
			processName: "test process",
			podName:     "test-pod",
			expected: &ketchv1.AppDeploymentEvent{
				Name:              app.Name,
				DeploymentVersion: 2,
				Reason:            "test reason",
				Description:       "test message",
				ProcessName:       "test process",
				Annotations: map[string]string{
					ketchv1.DeploymentAnnotationAppName:            app.Name,
					ketchv1.DeploymentAnnotationDevelopmentVersion: "2",
					ketchv1.DeploymentAnnotationEventName:          "test reason",
					ketchv1.DeploymentAnnotationDescription:        "test message",
					ketchv1.DeploymentAnnotationProcessName:        "test process",
					ketchv1.DeploymentAnnotationPodErrorName:       "test-pod",
				},
			},
		},
	}
	for i, tc := range tests {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			ev := newAppDeploymentEvent(app, tc.reason, tc.desc, tc.processName, tc.podName)
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
			res := isDeploymentEvent(tc.msg, dep.Name)
			require.Equal(t, tc.expected, res)
		})
	}
}

func TestIsHPATarget(t *testing.T) {
	hpaList := autoscalingv2.HorizontalPodAutoscalerList{
		Items: []autoscalingv2.HorizontalPodAutoscaler{
			{
				Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{},
				},
			},
		},
	}
	app := ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app",
		},
		Spec: ketchv1.AppSpec{
			Deployments: []ketchv1.AppDeploymentSpec{
				{
					Version: ketchv1.DeploymentVersion(2),
					Processes: []ketchv1.ProcessSpec{
						{
							Name: "web",
						},
						{
							Name: "worker",
						},
					},
				},
			},
		},
	}
	tests := []struct {
		name              string
		hpaScaleTargetRef autoscalingv2.CrossVersionObjectReference
		expected          map[string]autoscalingv2.HorizontalPodAutoscaler
	}{
		{
			name: "is target",
			hpaScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				Name:       "app-worker-2",
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			expected: map[string]autoscalingv2.HorizontalPodAutoscaler{"app-worker-2": hpaList.Items[0]},
		},
		{
			name: "not target",
			hpaScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				Name:       "target",
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			expected: map[string]autoscalingv2.HorizontalPodAutoscaler{},
		},
		{
			name: "mismatched apiVersion/Kind",
			hpaScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				Name:       "app-worker-2",
				APIVersion: "fake/v3",
				Kind:       "TestKind",
			},
			expected: map[string]autoscalingv2.HorizontalPodAutoscaler{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hpaList.Items[0].Spec.ScaleTargetRef = tc.hpaScaleTargetRef
			got := hpaTargetMap(&app, hpaList)
			require.Equal(t, len(tc.expected), len(got))
			for k := range tc.expected {
				_, ok := got[k]
				require.True(t, ok)
			}
		})
	}
}

func Test_appReconcileResult_isConflictError(t *testing.T) {
	resource := schema.GroupResource{Group: "theketch.io", Resource: "App"}
	conflictErr := k8serrors.NewConflict(resource, "app-web-1", fmt.Errorf("some err"))
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "some err",
			err:  fmt.Errorf("some err"),
			want: false,
		},
		{
			name: "conflict error",
			err:  conflictErr,
			want: true,
		},
		{
			name: "conflict error",
			err:  fmt.Errorf("failed %w", conflictErr),
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := appReconcileResult{
				err: tt.err,
			}
			isConflict := r.isConflictError()
			require.Equal(t, tt.want, isConflict)
		})
	}
}

func TestUpdateNamespaceLabelsForIngress(t *testing.T) {
	tests := []struct {
		description       string
		app               *ketchv1.App
		expectedAppLabels []ketchv1.MetadataItem
	}{
		{
			description: "istio",
			app: &ketchv1.App{
				Spec: ketchv1.AppSpec{
					Namespace: "test-1",
					Deployments: []ketchv1.AppDeploymentSpec{{
						Version: 1,
						Processes: []ketchv1.ProcessSpec{{
							Name: "process-1",
						}},
					}},
					Ingress: ketchv1.IngressSpec{Controller: ketchv1.IngressControllerSpec{IngressType: ketchv1.IstioIngressControllerType}},
				},
				ObjectMeta: metav1.ObjectMeta{Name: "app-1", Namespace: "test-1"},
			},
			expectedAppLabels: []ketchv1.MetadataItem{{
				Apply:             map[string]string{"sidecar.istio.io/inject": "true"},
				DeploymentVersion: 1,
				Target:            ketchv1.Target{Kind: "Pod", APIVersion: "v1"},
				ProcessName:       "process-1",
			}},
		},
		{
			description: "non-istio",
			app: &ketchv1.App{
				Spec: ketchv1.AppSpec{
					Namespace: "test-2",
					Deployments: []ketchv1.AppDeploymentSpec{{
						Version: 1,
						Processes: []ketchv1.ProcessSpec{{
							Name: "process-1",
						}},
					}},
					Ingress: ketchv1.IngressSpec{Controller: ketchv1.IngressControllerSpec{IngressType: ketchv1.TraefikIngressControllerType}},
				},
				ObjectMeta: metav1.ObjectMeta{Name: "app-2", Namespace: "test-2"},
			},
			expectedAppLabels: []ketchv1.MetadataItem{{
				Apply:             map[string]string{"sidecar.istio.io/inject": "false"},
				DeploymentVersion: 1,
				Target:            ketchv1.Target{Kind: "Pod", APIVersion: "v1"},
				ProcessName:       "process-1",
			}},
		},
		{
			description: "non-istio - clean up old labels",
			app: &ketchv1.App{
				Spec: ketchv1.AppSpec{
					Namespace: "test-3",
					Deployments: []ketchv1.AppDeploymentSpec{{
						Version: 1,
						Processes: []ketchv1.ProcessSpec{{
							Name: "process-1",
						}},
					}},
					Ingress: ketchv1.IngressSpec{Controller: ketchv1.IngressControllerSpec{IngressType: ketchv1.TraefikIngressControllerType}},
					Labels: []ketchv1.MetadataItem{{
						Target: ketchv1.Target{Kind: "Pod", APIVersion: "v1"},
						Apply:  map[string]string{"sidecar.istio.io/inject": "true"},
					}},
				},
				ObjectMeta: metav1.ObjectMeta{Name: "app-3", Namespace: "test-3"},
			},
			expectedAppLabels: []ketchv1.MetadataItem{{
				Apply:             map[string]string{"sidecar.istio.io/inject": "false"},
				DeploymentVersion: 1,
				Target:            ketchv1.Target{Kind: "Pod", APIVersion: "v1"},
				ProcessName:       "process-1",
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			UpdateAppLabelsForIngress(tc.app)
			require.Equal(t, tc.expectedAppLabels, tc.app.Spec.Labels)
		})
	}
}
