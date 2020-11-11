package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

// NewPodStatusReporter returns an PodStatusReporter instance.
func NewPodStatusReporter(config *rest.Config, client client.Client, logger logr.Logger) (*PodStatusReporter, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &PodStatusReporter{
		ctrlClient: client,
		clientset:  clientset,
		logger:     logger,
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ketch-app-status-updater"),
	}, nil
}

// PodStatusReporter implements Watch() method. It looks for changes related to ketch applications' pods and sets
// AppContainersReady condition on a corresponing App CRD.
type PodStatusReporter struct {
	ctrlClient client.Client
	clientset  *kubernetes.Clientset
	logger     logr.Logger
	queue      workqueue.RateLimitingInterface
}

// Watch runs kubernetes informer and looks for changes related to applications' pods.
func (r *PodStatusReporter) Watch() {
	factory := informers.NewSharedInformerFactory(r.clientset, 0)
	informer := factory.Core().V1().Pods().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    r.onAdd,
		UpdateFunc: r.onUpdate,
		DeleteFunc: r.onDelete,
	})

	stopper := make(chan struct{})
	defer close(stopper)

	// running forever
	go informer.Run(stopper)

	// running forever
	wait.Until(r.worker, 1*time.Second, stopper)
}

func (r *PodStatusReporter) onAdd(obj interface{}) {
	pod, _ := obj.(*v1.Pod)
	w := podWrapper{pod: pod}
	if !w.isKetchApplicationPod() {
		return
	}
	r.reportStatus(pod.Namespace, w.AppName())
}

func (r *PodStatusReporter) onDelete(obj interface{}) {
	pod, _ := obj.(*v1.Pod)
	w := podWrapper{pod: pod}
	if !w.isKetchApplicationPod() {
		return
	}
	r.reportStatus(pod.Namespace, w.AppName())
}

func (r *PodStatusReporter) onUpdate(_, newObj interface{}) {
	pod, _ := newObj.(*v1.Pod)
	w := podWrapper{pod: pod}
	if !w.isKetchApplicationPod() {
		return
	}
	r.reportStatus(pod.Namespace, w.AppName())
}

func (r *PodStatusReporter) reportStatus(namespace string, appName string) {
	r.queue.AddRateLimited(queueItem{AppName: appName, Namespace: namespace})
}

type queueItem struct {
	AppName   string
	Namespace string
}

func (r *PodStatusReporter) worker() {
	for r.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it.
func (r *PodStatusReporter) processNextWorkItem() bool {
	obj, shutdown := r.queue.Get()
	if shutdown {
		// Stop working
		return false
	}

	// We call Done here so the workqueue knows we have finished
	// processing this item. We also must remember to call Forget if we
	// do not want this work item being re-queued. For example, we do
	// not call Forget if a transient error occurs, instead the item is
	// put back on the workqueue and attempted again after a back-off
	// period.
	defer r.queue.Done(obj)

	i := obj.(queueItem)

	updated := r.updateAppStatus(i)
	if updated {
		r.queue.Forget(obj)
	}
	return true
}

func (r *PodStatusReporter) updateAppStatus(item queueItem) bool {
	selector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			labelAppName: item.AppName,
		},
	}
	s, _ := metav1.LabelSelectorAsSelector(selector)
	options := metav1.ListOptions{
		LabelSelector: s.String(),
	}
	pods, err := r.clientset.CoreV1().Pods(item.Namespace).List(context.Background(), options)
	if err != nil {
		r.logger.Error(err, "Failed to list pods")
		return false
	}
	status := v1.ConditionFalse
	totalContainers := 0
	for _, pod := range pods.Items {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			totalContainers += 1
			if containerStatus.State.Running != nil {
				// at least one container should be running.
				status = v1.ConditionTrue
			}
		}
	}
	// we don't report an error on our side if there are no containers.
	if totalContainers == 0 {
		status = v1.ConditionTrue
	}

	app := &ketchv1.App{}
	if err = r.ctrlClient.Get(context.Background(), types.NamespacedName{Name: item.AppName}, app); err != nil {
		r.logger.Error(err, "Failed to get app")
		return false
	}
	app.SetCondition(ketchv1.AppContainersReady, status, "", metav1.NewTime(time.Now()))
	if err := r.ctrlClient.Status().Update(context.Background(), app); err != nil {
		r.logger.Error(err, "Failed to update app's status")
		return false
	}
	return true
}
