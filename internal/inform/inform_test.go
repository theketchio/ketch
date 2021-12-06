package inform

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"
	"github.com/theketchio/ketch/internal/controllers"
)

type mockInformer struct {
	running bool
}

// Run implements sharedIndexInformer. It sets running true until a value is received on the stop channel.
func (m *mockInformer) Run(stop <-chan struct{}) {
	m.running = true
	<-stop
	m.running = false
	return
}

func TestInformerManager(t *testing.T) {
	informerOne := &mockInformer{}
	i := &InformerManager{
		sharedIndexInformers: make(map[sharedIndexInformer]chan struct{}),
	}

	i.AddInformer(informerOne)
	require.Equal(t, len(i.sharedIndexInformers), 1)
	i.RunInformers()
	time.Sleep(time.Millisecond * 100) // allow a moment until checking the mockInformer value
	require.True(t, informerOne.running)
	i.Stop()
	time.Sleep(time.Millisecond * 100) // allow a moment until checking the mockInformer value
	require.False(t, informerOne.running)
}

func TestOnDeploymentEventUpdate(t *testing.T) {
	app := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testapp",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "App",
			APIVersion: "theketch.io/v1beta1",
		},
		Spec: ketchv1.AppSpec{
			Deployments: []ketchv1.AppDeploymentSpec{},
			Framework:   "second-framework",
		},
	}
	defaultObjects := []client.Object{app}
	ctx, err := controllers.Setup(nil, nil, defaultObjects)
	require.Nil(t, err)
	require.NotNil(t, ctx)
	defer controllers.Teardown(ctx)

	informerManager := &InformerManager{
		mgr: ctx.Manager,
	}

	tests := []struct {
		description string
		hpa         *autoscalingv1.HorizontalPodAutoscaler
		event       string
	}{
		{
			description: "success",
			hpa: &autoscalingv1.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "testHPA",
					OwnerReferences: []metav1.OwnerReference{{Name: "testapp"}},
				},
				Status: autoscalingv1.HorizontalPodAutoscalerStatus{
					CurrentReplicas: 1,
					DesiredReplicas: 2,
				},
			},
			event: "Normal HorizontalPodAutoscaler scaling App testapp from current: 1 to desired: 2",
		},
		{
			description: "error - wrong number of object owners",
			hpa: &autoscalingv1.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "testHPA",
					OwnerReferences: []metav1.OwnerReference{},
				},
				Status: autoscalingv1.HorizontalPodAutoscalerStatus{
					CurrentReplicas: 1,
					DesiredReplicas: 2,
				},
			},
			event: "Warning AppReconcileError error getting App that owns HorizontalPodAutoscaler: expected HorizontalPodAutoscaler testHPA to have 1 owner, but 0 are present",
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			fakeRecorder := record.NewFakeRecorder(128)
			informerManager.recorder = fakeRecorder
			informerManager.onDeploymentEventUpdate(nil, tc.hpa)

			// assure expected event is received within 2 seconds
			done := make(chan error)
			time.AfterFunc(time.Second*2, func() {
				done <- fmt.Errorf("expected event %s was not received", tc.event)
			})
		EVENTS:
			for {
				select {
				case receivedEvent := <-fakeRecorder.Events:
					if receivedEvent == tc.event {
						break EVENTS
					}
				case err := <-done:
					t.Error(err)
					break EVENTS
				}
			}
		})
	}
}

func TestGetApp(t *testing.T) {
	app := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testapp",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "App",
			APIVersion: "theketch.io/v1beta1",
		},
		Spec: ketchv1.AppSpec{
			Deployments: []ketchv1.AppDeploymentSpec{},
			Framework:   "second-framework",
		},
	}
	defaultObjects := []client.Object{app}
	ctx, err := controllers.Setup(nil, nil, defaultObjects)
	require.Nil(t, err)
	require.NotNil(t, ctx)
	defer controllers.Teardown(ctx)

	informerManager := &InformerManager{
		mgr: ctx.Manager,
	}

	tests := []struct {
		description string
		hpa         *autoscalingv1.HorizontalPodAutoscaler
		expectedApp string
		err         string
	}{
		{
			description: "success",
			hpa: &autoscalingv1.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "testHPA",
					OwnerReferences: []metav1.OwnerReference{{Name: "testapp"}},
				},
			},
			expectedApp: "testapp",
		},
		{
			description: "error - wrong number of object owners",
			hpa: &autoscalingv1.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "testHPA",
					OwnerReferences: []metav1.OwnerReference{},
				},
			},
			expectedApp: "",
			err:         "expected HorizontalPodAutoscaler testHPA to have 1 owner, but 0 are present",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			res, err := informerManager.getApp(tc.hpa)
			if tc.err != "" {
				require.Nil(t, res)
				require.EqualError(t, err, tc.err)
			} else {
				require.Equal(t, tc.expectedApp, res.Name)
				require.Nil(t, err)
			}
		})
	}
}

func TestNewAppDeploymentAutoscalingEvent(t *testing.T) {
	app := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testapp",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "App",
			APIVersion: "theketch.io/v1beta1",
		},
		Spec: ketchv1.AppSpec{
			Deployments: []ketchv1.AppDeploymentSpec{},
			Framework:   "second-framework",
		},
	}

	tests := []struct {
		description string
		hpa         *autoscalingv1.HorizontalPodAutoscaler
		event       *ketchv1.AppDeploymentEvent
	}{
		{
			description: "success",
			hpa: &autoscalingv1.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "testHPA",
					OwnerReferences: []metav1.OwnerReference{{Name: "testapp"}},
				},
				Status: autoscalingv1.HorizontalPodAutoscalerStatus{
					CurrentReplicas: 1,
					DesiredReplicas: 2,
				},
			},
			event: &ketchv1.AppDeploymentEvent{
				Reason:      "HorizontalPodAutoscaler",
				Description: "scaling App testapp from current: 1 to desired: 2",
				Annotations: map[string]string{
					ketchv1.DeploymentAnnotationAppName:            "testapp",
					ketchv1.DeploymentAnnotationDevelopmentVersion: "0",
					ketchv1.DeploymentAnnotationEventName:          "HorizontalPodAutoscaler",
					ketchv1.DeploymentAnnotationDescription:        "scaling App testapp from current: 1 to desired: 2",
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			event := newAppDeploymentAutoscalingEvent(tc.hpa, app)
			require.Equal(t, tc.event, event)
		})
	}
}
