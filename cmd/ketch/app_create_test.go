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

func Test_appCreatePoolValidity(t *testing.T) {
	invalidPoolApp := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "invalid-pool-app",
		},
		Spec: ketchv1.AppSpec{
			Pool: "invalid-pool",
		},
	}

	validPoolApp := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "valid-pool-app",
		},
		Spec: ketchv1.AppSpec{
			Pool: "valid-pool",
		},
	}

	tests := []struct {
		name    string
		cfg     config
		options appCreateOptions
		wantErr string
	}{
		{
			name: "failing - invalid pool",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{invalidPoolApp},
			},
			options: appCreateOptions{
				name: invalidPoolApp.Name,
				pool: invalidPoolApp.Spec.Pool,
			},
			wantErr: `failed to get pool instance: pools.theketch.io "invalid-pool" not found`,
		},
		{
			name: "passing - valid pool",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{validPoolApp},
			},
			options: appCreateOptions{
				name: validPoolApp.Name,
				pool: validPoolApp.Spec.Pool,
			},
		},
	}

	// Create pool for testing happy path
	poolCfg := &mocks.Configuration{CtrlClientObjects: []runtime.Object{}}
	poolOpt := poolAddOptions{name: "valid-pool", ingressServiceEndpoint: "10.10.20.30", ingressType: traefik}
	if err := addPool(context.Background(), poolCfg, poolOpt, &bytes.Buffer{}); err != nil {
		t.Error(err)
		return
	}

	for _, tt := range tests {
		err := appCreate(context.Background(), tt.cfg, tt.options, &bytes.Buffer{})
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
