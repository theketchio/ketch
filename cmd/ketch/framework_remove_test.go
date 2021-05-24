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

func TestFrameworkRemove(t *testing.T) {
	testFramework := &ketchv1.Framework{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-framework",
		},
		Spec: ketchv1.FrameworkSpec{
			NamespaceName: "test-namespace",
		},
	}

	tests := []struct {
		name      string
		cfg       config
		options   frameworkRemoveOptions
		framework *ketchv1.Framework
		wantErr   string
	}{
		{
			name: "remove framework and associated namespace",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{testFramework},
			},
			options:   frameworkRemoveOptions{Name: testFramework.Name},
			framework: testFramework,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := frameworkRemove(context.Background(), tt.cfg, tt.options, &bytes.Buffer{})

			if len(tt.wantErr) > 0 {
				assert.Error(t, err, tt.wantErr)
				return
			}

			assert.NilError(t, err)

			var frameworks ketchv1.FrameworkList
			if err := tt.cfg.Client().List(context.Background(), &frameworks); err != nil {
				t.Errorf("failed to list test framework: %s", err.Error())
				return
			}

			assert.Equal(t, 0, len(frameworks.Items))
		})
	}
}
