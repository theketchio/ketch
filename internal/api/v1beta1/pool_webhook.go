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

package v1beta1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var poollog = logf.Log.WithName("pool-resource")

type manager interface {
	GetClient() client.Client
}

var poolmgr manager = nil

func (r *Pool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	poolmgr = mgr
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-theketch-io-v1beta1-pool,mutating=true,failurePolicy=fail,groups=theketch.io,resources=pools,verbs=create;update,versions=v1beta1,name=mpool.kb.io

var _ webhook.Defaulter = &Pool{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Pool) Default() {
	poollog.Info("default", "name", r.Name)
}

// +kubebuilder:webhook:verbs=create;update;delete,path=/validate-theketch-io-v1beta1-pool,mutating=false,failurePolicy=fail,groups=theketch.io,resources=pools,versions=v1beta1,name=vpool.kb.io

var _ webhook.Validator = &Pool{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Pool) ValidateCreate() error {
	poollog.Info("validate create", "name", r.Name)
	client := poolmgr.GetClient()
	ctx := context.TODO()
	pools := PoolList{}
	if err := client.List(ctx, &pools); err != nil {
		return err
	}
	for _, pool := range pools.Items {
		if pool.Spec.NamespaceName == r.Spec.NamespaceName {
			return ErrNamespaceIsUsedByAnotherPool
		}
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Pool) ValidateUpdate(old runtime.Object) error {
	poollog.Info("validate update", "name", r.Name)

	oldPool, ok := old.(*Pool)
	if !ok {
		return fmt.Errorf("can't validate pool update")
	}

	c := poolmgr.GetClient()
	if oldPool.Spec.NamespaceName != r.Spec.NamespaceName {
		if len(r.Status.Apps) > 0 {
			return ErrChangeNamespaceWhenAppsRunning
		}
		pools := PoolList{}
		if err := c.List(context.Background(), &pools); err != nil {
			return err
		}
		for _, pool := range pools.Items {
			if pool.Spec.NamespaceName == r.Spec.NamespaceName && r.Name != pool.Name {
				return ErrNamespaceIsUsedByAnotherPool
			}
		}
	}
	if oldPool.Spec.AppQuotaLimit != r.Spec.AppQuotaLimit {
		if r.Spec.AppQuotaLimit < len(r.Status.Apps) && r.Spec.AppQuotaLimit != -1 {
			return ErrDecreaseQuota
		}
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Pool) ValidateDelete() error {
	poollog.Info("validate delete", "name", r.Name)
	if len(r.Status.Apps) > 0 {
		return ErrDeletePoolWithRunningApps
	}
	return nil
}
