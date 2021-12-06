package inform

import (
	"context"
	"fmt"
	"strconv"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"
)

// InformerManager contains fields used by Informers
type InformerManager struct {
	mgr                  ctrl.Manager
	kubernetesClient     *kubernetes.Clientset
	recorder             record.EventRecorder
	sharedIndexInformers map[sharedIndexInformer]chan struct{} // informer: stop chan
}

// sharedIndexInformer defines the methods from cache.SharedIndexInformer that InformerManager relies on. Makes testing easier.
type sharedIndexInformer interface {
	Run(stop <-chan struct{})
}

// NewInformerManager generates an InformerManager
func NewInformerManager(mgr ctrl.Manager, recorder record.EventRecorder) (*InformerManager, error) {
	kubernetesClient, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return nil, err
	}

	return &InformerManager{
		kubernetesClient:     kubernetesClient,
		mgr:                  mgr,
		recorder:             recorder,
		sharedIndexInformers: make(map[sharedIndexInformer]chan struct{}),
	}, nil
}

// Stop sends a struct on the stop chan for all added informers
func (i *InformerManager) Stop() {
	for _, stop := range i.sharedIndexInformers {
		stop <- struct{}{}
	}
}

// AddInformer adds an informer to the InformerManager's informer map
func (i *InformerManager) AddInformer(informer sharedIndexInformer) {
	i.sharedIndexInformers[informer] = make(chan struct{})
}

// RunInformers calls Run on all added Informers
func (i *InformerManager) RunInformers() {
	defer runtime.HandleCrash()

	for informer, stop := range i.sharedIndexInformers {
		go informer.Run(stop)
	}
}

/* Autoscaler Informer */

// AutoscalerInformer returns an informer that is designed to issue App Events based on HorizontalPodAutoscaler updates
func (i *InformerManager) AutoscalerInformer() cache.SharedIndexInformer {
	factory := informers.NewSharedInformerFactory(i.kubernetesClient, 0)
	informer := factory.Autoscaling().V1().HorizontalPodAutoscalers().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: i.onDeploymentEventUpdate,
	})

	return informer
}

func (i *InformerManager) onDeploymentEventUpdate(o, n interface{}) {
	newHPA, ok := n.(*autoscalingv1.HorizontalPodAutoscaler)
	if !ok {
		return
	}

	app, err := i.getApp(newHPA)
	if err != nil {
		i.recorder.Eventf(newHPA, v1.EventTypeWarning, ketchv1.AppReconcileError, "error getting App that owns HorizontalPodAutoscaler: %s", err.Error())
		return
	}

	appEvent := newAppDeploymentAutoscalingEvent(newHPA, app)
	i.recorder.AnnotatedEventf(app, appEvent.Annotations, v1.EventTypeNormal, appEvent.Reason, appEvent.Description)
}

func (i *InformerManager) getApp(hpa *autoscalingv1.HorizontalPodAutoscaler) (*ketchv1.App, error) {
	ownerReferences := hpa.GetOwnerReferences()
	if len(ownerReferences) != 1 {
		return nil, fmt.Errorf("expected HorizontalPodAutoscaler %s to have 1 owner, but %d are present", hpa.Name, len(ownerReferences))
	}
	cli := i.mgr.GetClient() // type client.Client

	var app ketchv1.App
	ctx := context.Background()

	if err := cli.Get(ctx, client.ObjectKey{
		Name:      ownerReferences[0].Name,
		Namespace: hpa.Namespace,
	}, &app); err != nil {
		return nil, err
	}
	return &app, nil
}

func newAppDeploymentAutoscalingEvent(hpa *autoscalingv1.HorizontalPodAutoscaler, app *ketchv1.App) *ketchv1.AppDeploymentEvent {
	var version int
	if len(app.Spec.Deployments) > 0 {
		version = int(app.Spec.Deployments[len(app.Spec.Deployments)-1].Version)
	}
	description := fmt.Sprintf("scaling App %s from current: %d to desired: %d", app.Name, hpa.Status.CurrentReplicas, hpa.Status.DesiredReplicas)
	event := &ketchv1.AppDeploymentEvent{
		Reason:      "HorizontalPodAutoscaler",
		Description: description,
		Annotations: map[string]string{
			ketchv1.DeploymentAnnotationAppName:            app.Name,
			ketchv1.DeploymentAnnotationDevelopmentVersion: strconv.Itoa(version),
			ketchv1.DeploymentAnnotationEventName:          "HorizontalPodAutoscaler",
			ketchv1.DeploymentAnnotationDescription:        description,
		},
	}
	return event
}
