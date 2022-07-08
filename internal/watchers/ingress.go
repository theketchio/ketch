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

package watchers

import (
	"context"
	"fmt"
	"time"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"

	"github.com/avast/retry-go"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IngressWatcher contains fields relevant to watching the ingress configmap
type IngressWatcher struct {
	clientSet           kubernetes.Interface
	client              client.Client
	sharedIndexInformer cache.SharedIndexInformer
	logger              logr.Logger

	retries    uint
	retryDelay time.Duration
}

var (
	informerResyncPeriod = time.Minute * 5
)

// NewIngressWatcher creates a IngressWatcher instance
func NewIngressWatcher(clientSet kubernetes.Interface, client client.Client, logger logr.Logger) *IngressWatcher {
	fieldSelector := labels.Set(map[string]string{"metadata.name": ketchv1.IngressConfigmapName}).AsSelector()
	return &IngressWatcher{
		clientSet:  clientSet,
		client:     client,
		logger:     logger,
		retries:    5,
		retryDelay: time.Second,
		sharedIndexInformer: cache.NewSharedIndexInformer(&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				options.FieldSelector = fieldSelector.String()
				return clientSet.CoreV1().ConfigMaps(ketchv1.IngressConfigmapNamespace).List(context.Background(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.FieldSelector = fieldSelector.String()
				return clientSet.CoreV1().ConfigMaps(ketchv1.IngressConfigmapNamespace).Watch(context.Background(), options)
			},
		},
			&v1.ConfigMap{},
			informerResyncPeriod,
			cache.Indexers{},
		),
	}
}

// Inform runs the IngressWatcher informer. When the ingress configmap is altered, handleAddUpdateIngressConfigmapFn
// is called.
func (i *IngressWatcher) Inform(ctx context.Context) error {
	stop := make(chan struct{})
	go func() {
		for range ctx.Done() {
			stop <- struct{}{}
		}
	}()
	i.sharedIndexInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			i.handleAddUpdateIngressConfigmap(ctx, obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			i.handleAddUpdateIngressConfigmap(ctx, newObj)
		},
	})
	go i.sharedIndexInformer.Run(stop)
	if !cache.WaitForCacheSync(stop, i.sharedIndexInformer.HasSynced) {
		return fmt.Errorf("Failed to sync")
	}
	return nil
}

func (i *IngressWatcher) handleAddUpdateIngressConfigmap(ctx context.Context, obj interface{}) {
	configmap, ok := obj.(*v1.ConfigMap)
	if !ok { // not configmap
		return
	}
	if configmap.Name != ketchv1.IngressConfigmapName { // not ingress configmap
		return
	}
	ingressControllerSpec := ketchv1.NewIngressControllerSpec(*configmap)
	err := retry.Do(func() error {
		return i.updateAppsIngress(ctx, *ingressControllerSpec)
	}, retry.Attempts(i.retries), retry.Delay(i.retryDelay))
	if err != nil {
		i.logger.Error(err, "error updating app's ingresses")
	}
}

func (i *IngressWatcher) updateAppsIngress(ctx context.Context, ingressControllerSpec ketchv1.IngressControllerSpec) error {
	var appList ketchv1.AppList
	if err := i.client.List(ctx, &appList); err != nil {
		i.logger.Error(err, "error listing apps during ingress update")
		return err
	}

	for _, app := range appList.Items {
		app.Spec.Ingress.Controller = ingressControllerSpec
		i.logger.Info("updating app ingress controller", "app", app.Name, "ingress controller", ingressControllerSpec)
		if err := i.client.Update(ctx, &app); err != nil {
			i.logger.Error(err, "error updating ingress", "app", app.Name)
			return err
		}
	}
	return nil
}
