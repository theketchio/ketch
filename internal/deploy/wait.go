package deploy

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/controllers"
	"github.com/shipa-corp/ketch/internal/errors"
	"github.com/shipa-corp/ketch/internal/utils"
)

type WaitFn func(ctx context.Context, svc *Params, app *ketchv1.App, timeout time.Duration) error

func WaitForDeployment(ctx context.Context, svc *Params, app *ketchv1.App, timeout time.Duration) error {
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	watcher, err := watchAppReconcileEvent(ctx, svc.KubeClient, app)
	if err != nil {
		return err
	}
	defer watcher.Stop()

	for {
		select {
		case msg, ok := <-watcher.ResultChan():
			if !ok {
				return errors.New("wait for deployment channel closed")
			}
			if evt, ok := msg.Object.(*corev1.Event); ok {
				reason, err := controllers.ParseAppReconcileMessage(evt.Reason)
				if err != nil {
					return err
				}
				if reason.DeploymentCount == app.Spec.DeploymentsCount {
					switch evt.Type {
					case corev1.EventTypeNormal:
						fmt.Fprintln(svc.Writer, "successfully deployed!")
						return nil
					case corev1.EventTypeWarning:
						return errors.New(evt.Message)
					}
				}
			}
		case <-tctx.Done():
			return fmt.Errorf("deployment timed out")
		}
	}
}

func watchAppReconcileEvent(ctx context.Context, kubeClient kubernetes.Interface, app *ketchv1.App) (watch.Interface, error) {
	reason := controllers.AppReconcileReason{AppName: app.Name, DeploymentCount: app.Spec.DeploymentsCount}
	selector := fields.Set(map[string]string{
		"involvedObject.apiVersion": utils.V1betaPrefix,
		"involvedObject.kind":       "App",
		"involvedObject.name":       app.Name,
		"reason":                    reason.String(),
	}).AsSelector()
	return kubeClient.CoreV1().
		Events(app.Namespace).Watch(ctx, metav1.ListOptions{FieldSelector: selector.String()})
}
