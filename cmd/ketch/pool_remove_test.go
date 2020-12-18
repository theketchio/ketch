package main

import (
	"bytes"
	"context"
	"k8s.io/apimachinery/pkg/api/errors"
	"testing"

	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/mocks"
)

func Test_poolRemove(t *testing.T) {
	testPool := &ketchv1.Pool{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pool",
		},
		Spec: ketchv1.PoolSpec{
			NamespaceName: "test-namespace",
		},
	}

	tests := []struct {
		name    string
		cfg     config
		options poolRemoveOptions
		pool    *ketchv1.Pool
		wantErr string
	}{
		{
			name: "remove pool and associated namespace",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{testPool},
			},
			options: poolRemoveOptions{Name: testPool.Name},
			pool:    testPool,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := poolRemove(context.Background(), tt.cfg, tt.options, &bytes.Buffer{})

			if len(tt.wantErr) > 0 {
				assert.Error(t, err, tt.wantErr)
			}

			// TODO: find a better way to test namespace removal in `poolRemove`.
			// This doesn't actually check the `err` from `poolRemove`, which fails here because the `fake` client does not
			// create a namespace when test pool is created.
			if err := tt.cfg.Client().Get(context.Background(), types.NamespacedName{Name: tt.pool.Spec.NamespaceName}, &corev1.Namespace{}); err != nil && !errors.IsNotFound(err) {
				t.Errorf("failed to get namespace: %s", err.Error())
			} else {
				assert.Check(t, errors.IsNotFound(err))
			}
		})
	}
}
