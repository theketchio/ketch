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

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

// PoolReconciler reconciles a Pool object.
type PoolReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=theketch.io,resources=pools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=theketch.io,resources=pools/status,verbs=get;update;patch

func (r *PoolReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("pool", req.NamespacedName)

	pool := ketchv1.Pool{}
	if err := r.Get(ctx, req.NamespacedName, &pool); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	status := r.reconcile(ctx, &pool)
	pool.Status = status

	err := r.Status().Update(ctx, &pool)
	return ctrl.Result{}, err
}

func (r *PoolReconciler) reconcile(ctx context.Context, pool *ketchv1.Pool) ketchv1.PoolStatus {
	pools := ketchv1.PoolList{}
	err := r.List(ctx, &pools)
	if err != nil {
		return ketchv1.PoolStatus{
			Phase:     ketchv1.PoolFailed,
			Message:   "failed to get a list of pools",
			Apps:      pool.Status.Apps,
			Namespace: pool.Status.Namespace,
		}
	}
	namespace := v1.Namespace{}
	failures := 0
	for {
		err := r.Get(ctx, types.NamespacedName{Name: pool.Spec.NamespaceName}, &namespace)
		if err == nil {
			break
		}
		if failures > 10 {
			return ketchv1.PoolStatus{
				Phase:     ketchv1.PoolFailed,
				Message:   fmt.Sprintf("failed to get %s namespace", pool.Spec.NamespaceName),
				Apps:      pool.Status.Apps,
				Namespace: pool.Status.Namespace,
			}
		}
		if errors.IsNotFound(err) {
			n := &v1.Namespace{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: pool.Spec.NamespaceName,
				},
				Spec:   v1.NamespaceSpec{},
				Status: v1.NamespaceStatus{},
			}
			err = r.Create(ctx, n)
			if err != nil {
				return ketchv1.PoolStatus{
					Phase:     ketchv1.PoolFailed,
					Message:   fmt.Sprintf("failed to create %s namespace", pool.Spec.NamespaceName),
					Apps:      pool.Status.Apps,
					Namespace: pool.Status.Namespace,
				}
			}
			failures += 1
			time.Sleep(1 * time.Second)
		}
	}
	// we rely on istio automatic sidecar injection
	// https://istio.io/latest/docs/setup/additional-setup/sidecar-injection/#automatic-sidecar-injection
	istioInjectionValue := "disabled"
	if pool.Spec.IngressController.IngressType == ketchv1.IstioIngressControllerType {
		istioInjectionValue = "enabled"
	}
	if namespace.Labels == nil {
		namespace.Labels = map[string]string{}
	}
	namespace.Labels["istio-injection"] = istioInjectionValue

	err = r.Update(ctx, &namespace)
	if err != nil {
		return ketchv1.PoolStatus{
			Phase:     ketchv1.PoolFailed,
			Message:   fmt.Sprintf("failed to update namespace annotations: %v", err),
			Apps:      pool.Status.Apps,
			Namespace: pool.Status.Namespace,
		}
	}

	ref, err := reference.GetReference(r.Scheme, &namespace)
	if err != nil {
		return ketchv1.PoolStatus{
			Phase:     ketchv1.PoolFailed,
			Message:   fmt.Sprintf("failed to get a reference to %s namespace", pool.Spec.NamespaceName),
			Apps:      pool.Status.Apps,
			Namespace: pool.Status.Namespace,
		}
	}
	for _, p := range pools.Items {
		// It shouldn't happen because we have a web hook that checks this.
		if p.Status.Namespace == nil {
			continue
		}
		if p.Status.Namespace.UID == ref.UID && p.Name != pool.Name {
			return ketchv1.PoolStatus{
				Phase:     ketchv1.PoolFailed,
				Message:   "Target namespace is already used by another pool",
				Apps:      pool.Status.Apps,
				Namespace: pool.Status.Namespace,
			}
		}
	}
	return ketchv1.PoolStatus{
		Namespace: ref,
		Phase:     ketchv1.PoolCreated,
		Apps:      pool.Status.Apps,
	}
}

func (r *PoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ketchv1.Pool{}).
		Complete(r)
}
