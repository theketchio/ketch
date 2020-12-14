package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/mocks"
)

func Test_appCreateFailsWithInvalidPool(t *testing.T) {
	app := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testapp",
		},
		Spec: ketchv1.AppSpec{
			Pool: "invalid-pool",
		},
	}

	tests := []struct {
		name    string
		cfg     config
		options appCreateOptions
		wantErr string
	}{
		{
			name: "error - no pool",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{app},
			},
			options: appCreateOptions{
				name: app.Name,
				pool: app.Spec.Pool,
			},
			wantErr: `failed to get pool instance: pools.theketch.io "invalid-pool" not found`,
		},
	}

	for _, tt := range tests {
		out := &bytes.Buffer{}
		err := appCreate(context.Background(), tt.cfg, tt.options, out)
		wantErr := len(tt.wantErr) > 0
		if (err != nil) != wantErr {
			t.Errorf("appCreate() error = %v, wantErr %v", err, tt.wantErr)
			return
		}
		if wantErr {
			assert.Equal(t, tt.wantErr, err.Error())
			return
		}
	}
}
