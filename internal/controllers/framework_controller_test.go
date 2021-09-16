package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/utils/conversions"
)

func TestFrameworkReconciler_Reconcile(t *testing.T) {
	defaultObjects := []client.Object{
		&ketchv1.Framework{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: "default-framework",
			},
			Spec: ketchv1.FrameworkSpec{
				NamespaceName: "default-namespace",
				AppQuotaLimit: conversions.IntPtr(100),
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
		framework                ketchv1.Framework
		wantStatusPhase          ketchv1.FrameworkPhase
		wantStatusMessage        string
		wantNamespaceAnnotations map[string]string
		wantNamespaceLabels      map[string]string
	}{
		{
			name: "namespace is used by another framework",
			framework: ketchv1.Framework{
				ObjectMeta: metav1.ObjectMeta{
					Name: "framework-2",
				},
				Spec: ketchv1.FrameworkSpec{
					AppQuotaLimit: conversions.IntPtr(1),
					NamespaceName: "default-namespace",
					IngressController: ketchv1.IngressControllerSpec{
						IngressType: ketchv1.IstioIngressControllerType,
					},
				},
			},
			wantStatusPhase:   ketchv1.FrameworkFailed,
			wantStatusMessage: "Target namespace is already used by another framework",
		},
		{
			name: "istio controller - everything is ok",
			framework: ketchv1.Framework{
				ObjectMeta: metav1.ObjectMeta{
					Name: "framework-3",
				},
				Spec: ketchv1.FrameworkSpec{
					AppQuotaLimit: conversions.IntPtr(1),
					NamespaceName: "another-namespace-3",
					IngressController: ketchv1.IngressControllerSpec{
						IngressType: ketchv1.IstioIngressControllerType,
					},
				},
			},
			wantStatusPhase: ketchv1.FrameworkCreated,
			wantNamespaceLabels: map[string]string{
				"istio-injection": "enabled",
			},
		},
		{
			name: "traefik controller - everything is ok",
			framework: ketchv1.Framework{
				ObjectMeta: metav1.ObjectMeta{
					Name: "framework-4",
				},
				Spec: ketchv1.FrameworkSpec{
					AppQuotaLimit: conversions.IntPtr(1),
					NamespaceName: "another-namespace-4",
					IngressController: ketchv1.IngressControllerSpec{
						IngressType: ketchv1.TraefikIngressControllerType,
					},
				},
			},
			wantStatusPhase: ketchv1.FrameworkCreated,
			wantNamespaceLabels: map[string]string{
				"istio-injection": "disabled",
			},
		},
		{
			name: "framework annotations and labels added to namespace",
			framework: ketchv1.Framework{
				ObjectMeta: metav1.ObjectMeta{
					Name: "framework-5",
				},
				Spec: ketchv1.FrameworkSpec{
					AppQuotaLimit: conversions.IntPtr(1),
					Annotations:   map[string]string{"test-annotation": "value"},
					Labels:        map[string]string{"test-label": "value"},
					NamespaceName: "another-namespace-5",
					IngressController: ketchv1.IngressControllerSpec{
						IngressType: ketchv1.IstioIngressControllerType,
					},
				},
			},
			wantStatusPhase: ketchv1.FrameworkCreated,
			wantNamespaceAnnotations: map[string]string{
				"test-annotation": "value",
			},
			wantNamespaceLabels: map[string]string{
				"test-label":      "value",
				"istio-injection": "enabled",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := ctx.k8sClient.Create(context.TODO(), &tt.framework)
			assert.Nil(t, err)

			resultFramework := ketchv1.Framework{}
			for {
				time.Sleep(250 * time.Millisecond)
				err = ctx.k8sClient.Get(context.TODO(), types.NamespacedName{Name: tt.framework.Name}, &resultFramework)
				assert.Nil(t, err)
				if len(resultFramework.Status.Phase) > 0 {
					break
				}
			}

			assert.Equal(t, tt.wantStatusPhase, resultFramework.Status.Phase)
			assert.Equal(t, tt.wantStatusMessage, resultFramework.Status.Message)

			if tt.wantStatusPhase == ketchv1.FrameworkCreated {
				assert.NotNil(t, resultFramework.Status.Namespace.Name)
				assert.NotNil(t, resultFramework.Status.Namespace.UID)

				gotNamespace := v1.Namespace{}
				err = ctx.k8sClient.Get(context.TODO(), types.NamespacedName{Name: tt.framework.Spec.NamespaceName}, &gotNamespace)
				assert.Equal(t, tt.wantNamespaceLabels, gotNamespace.Labels)
				assert.Equal(t, tt.wantNamespaceAnnotations, gotNamespace.Annotations)
			} else {
				assert.Nil(t, resultFramework.Status.Namespace)
			}
		})
	}
}

func TestFrameworkReconciler_ReconcileUpdate(t *testing.T) {
	defaultObjects := []client.Object{
		&ketchv1.Framework{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: "default-framework",
			},
			Spec: ketchv1.FrameworkSpec{
				Annotations: map[string]string{
					"remove-after-update": "value",
					"keep-after-update":   "value",
				},
				Labels: map[string]string{
					"remove-after-update": "value",
					"keep-after-update":   "value",
				},
				NamespaceName: "default-namespace",
				AppQuotaLimit: conversions.IntPtr(100),
				IngressController: ketchv1.IngressControllerSpec{
					IngressType: ketchv1.IstioIngressControllerType,
				},
			},
		},
	}
	ctx, err := setup(nil, nil, defaultObjects)
	assert.Nil(t, err)

	defer teardown(ctx)
	updatedAnnotations := map[string]string{
		"keep-after-update": "value",
	}
	updatedLabels := map[string]string{
		"keep-after-update": "value",
	}

	tests := []struct {
		name                  string
		frameworkName         string
		namespaceName         string
		wantStatusPhase       ketchv1.FrameworkPhase
		wantStatusMessage     string
		preUpdateAnnotations  map[string]string
		preUpdateLabels       map[string]string
		postUpdateAnnotations map[string]string
		postUpdateLabels      map[string]string
	}{
		{
			name:            "Updating framework annotations/labels is reflected in namespace",
			frameworkName:   "default-framework",
			namespaceName:   "default-namespace",
			wantStatusPhase: ketchv1.FrameworkCreated,
			preUpdateAnnotations: map[string]string{
				"remove-after-update": "value",
				"keep-after-update":   "value",
			},
			preUpdateLabels: map[string]string{
				"remove-after-update": "value",
				"keep-after-update":   "value",
				"istio-injection":     "enabled",
			},
			postUpdateAnnotations: map[string]string{
				"keep-after-update": "value",
			},
			postUpdateLabels: map[string]string{
				"keep-after-update": "value",
				"istio-injection":   "enabled",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			resultFramework := ketchv1.Framework{}
			for {
				time.Sleep(250 * time.Millisecond)
				err = ctx.k8sClient.Get(context.TODO(), types.NamespacedName{Name: tt.frameworkName}, &resultFramework)
				assert.Nil(t, err)
				if len(resultFramework.Status.Phase) > 0 {
					break
				}
			}
			gotNamespace := v1.Namespace{}
			err = ctx.k8sClient.Get(context.TODO(), types.NamespacedName{Name: tt.namespaceName}, &gotNamespace)
			assert.Equal(t, tt.preUpdateLabels, gotNamespace.Labels)
			assert.Equal(t, tt.preUpdateAnnotations, gotNamespace.Annotations)

			resultFramework.Spec.Labels = updatedLabels
			resultFramework.Spec.Annotations = updatedAnnotations

			err := ctx.k8sClient.Update(context.TODO(), &resultFramework)
			assert.Nil(t, err)

			resultFramework = ketchv1.Framework{}
			for {
				time.Sleep(250 * time.Millisecond)
				err = ctx.k8sClient.Get(context.TODO(), types.NamespacedName{Name: tt.frameworkName}, &resultFramework)
				assert.Nil(t, err)
				if len(resultFramework.Status.Phase) > 0 {
					break
				}
			}

			assert.Equal(t, tt.wantStatusPhase, resultFramework.Status.Phase)
			assert.Equal(t, tt.wantStatusMessage, resultFramework.Status.Message)

			assert.NotNil(t, resultFramework.Status.Namespace.Name)
			assert.NotNil(t, resultFramework.Status.Namespace.UID)

			gotNamespace = v1.Namespace{}
			err = ctx.k8sClient.Get(context.TODO(), types.NamespacedName{Name: tt.namespaceName}, &gotNamespace)
			assert.Equal(t, tt.postUpdateLabels, gotNamespace.Labels)
			assert.Equal(t, tt.postUpdateAnnotations, gotNamespace.Annotations)
		})
	}
}
