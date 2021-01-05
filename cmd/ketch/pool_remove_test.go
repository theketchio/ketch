package main

import (
	"bytes"
	"context"
	"testing"

	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/mocks"
)

func TestPoolRemove(t *testing.T) {
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
				return
			}

			assert.NilError(t, err)

			var pools ketchv1.PoolList
			if err := tt.cfg.Client().List(context.Background(), &pools); err != nil {
				t.Errorf("failed to list test pool: %s", err.Error())
				return
			}

			assert.Equal(t, 0, len(pools.Items))
		})
	}
}
