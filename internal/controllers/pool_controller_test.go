package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

func TestPoolReconciler_Reconcile(t *testing.T) {
	defaultObjects := []runtime.Object{
		&ketchv1.Pool{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: "default-pool",
			},
			Spec: ketchv1.PoolSpec{
				NamespaceName: "default-namespace",
				AppQuotaLimit: 100,
				IngressController: ketchv1.IngressControllerSpec{
					IngressType: ketchv1.IstioIngressControllerType,
				},
			},
		},
	}
	ctx, err := setup(nil, nil, defaultObjects)
	assert.Nil(t, err)

	defer teardown(ctx)
	tests := []struct {
		name                     string
		pool                     ketchv1.Pool
		wantStatusPhase          ketchv1.PoolPhase
		wantStatusMessage        string
		wantNamespaceAnnotations map[string]string
	}{
		{
			name: "namespace is used by another pool",
			pool: ketchv1.Pool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pool-2",
				},
				Spec: ketchv1.PoolSpec{
					NamespaceName: "default-namespace",
					IngressController: ketchv1.IngressControllerSpec{
						IngressType: ketchv1.IstioIngressControllerType,
					},
				},
			},
			wantStatusPhase:   ketchv1.PoolFailed,
			wantStatusMessage: "Target namespace is already used by another pool",
		},
		{
			name: "istio controller - everything is ok",
			pool: ketchv1.Pool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pool-3",
				},
				Spec: ketchv1.PoolSpec{
					NamespaceName: "another-namespace-3",
					IngressController: ketchv1.IngressControllerSpec{
						IngressType: ketchv1.IstioIngressControllerType,
					},
				},
			},
			wantStatusPhase: ketchv1.PoolCreated,
			wantNamespaceAnnotations: map[string]string{
				"istio-injection": "enabled",
			},
		},
		{
			name: "traefik controller - everything is ok",
			pool: ketchv1.Pool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pool-4",
				},
				Spec: ketchv1.PoolSpec{
					NamespaceName: "another-namespace-4",
					IngressController: ketchv1.IngressControllerSpec{
						IngressType: ketchv1.TraefikIngressControllerType,
					},
				},
			},
			wantStatusPhase: ketchv1.PoolCreated,
			wantNamespaceAnnotations: map[string]string{
				"istio-injection": "disabled",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := ctx.k8sClient.Create(context.TODO(), &tt.pool)
			assert.Nil(t, err)

			resultPool := ketchv1.Pool{}
			for {
				time.Sleep(250 * time.Millisecond)
				err = ctx.k8sClient.Get(context.TODO(), types.NamespacedName{Name: tt.pool.Name}, &resultPool)
				assert.Nil(t, err)
				if len(resultPool.Status.Phase) > 0 {
					break
				}
			}

			assert.Equal(t, tt.wantStatusPhase, resultPool.Status.Phase)
			assert.Equal(t, tt.wantStatusMessage, resultPool.Status.Message)

			if tt.wantStatusPhase == ketchv1.PoolCreated {
				assert.NotNil(t, resultPool.Status.Namespace.Name)
				assert.NotNil(t, resultPool.Status.Namespace.UID)

				gotNamespace := v1.Namespace{}
				err = ctx.k8sClient.Get(context.TODO(), types.NamespacedName{Name: tt.pool.Spec.NamespaceName}, &gotNamespace)
				assert.Equal(t, tt.wantNamespaceAnnotations, gotNamespace.Labels)
			} else {
				assert.Nil(t, resultPool.Status.Namespace)
			}
		})
	}
}
