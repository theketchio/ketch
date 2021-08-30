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
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// joblog is for logging in this package.
var joblog = logf.Log.WithName("job-resource")

var jobmgr manager = nil

func (r *Job) SetupWebhookWithManager(mgr ctrl.Manager) error {
	jobmgr = mgr
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update;delete,path=/validate-theketch-io-v1beta1-job,mutating=false,failurePolicy=fail,groups=theketch.io,resources=jobs,versions=v1beta1,name=vjob.kb.io,sideEffects=none,admissionReviewVersions=v1beta1

var _ webhook.Validator = &Job{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Job) ValidateCreate() error {
	joblog.Info("validate create", "name", r.Name)
	client := jobmgr.GetClient()
	jobs := JobList{}
	if err := client.List(context.Background(), &jobs); err != nil {
		return err
	}
	for _, job := range jobs.Items {
		if job.Spec.Name == r.Spec.Name {
			return ErrJobExists
		}
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Job) ValidateUpdate(old runtime.Object) error {
	joblog.Info("validate update", "name", r.Name)
	oldJob, ok := old.(*Job)
	if !ok {
		return fmt.Errorf("can't validate job update")
	}
	client := jobmgr.GetClient()
	jobs := JobList{}
	if err := client.List(context.Background(), &jobs); err != nil {
		return err
	}
	for _, job := range jobs.Items {
		if job.Spec.Name == oldJob.Spec.Name {
			return ErrJobExists
		}
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Job) ValidateDelete() error {
	return nil
}
