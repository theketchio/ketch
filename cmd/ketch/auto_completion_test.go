package main

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1"
	"github.com/theketchio/ketch/internal/mocks"
)

func Test_autoCompleteAppNames(t *testing.T) {
	appA := &ketchv1.App{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "app-a",
		},
	}
	appB := &ketchv1.App{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "app-b",
		},
	}

	tests := []struct {
		name string
		cfg  config

		want         []string
		wantFallback cobra.ShellCompDirective
	}{
		{
			name: "show all, no error",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{appA, appB},
			},

			want:         []string{"app-a", "app-b"},
			wantFallback: cobra.ShellCompDirectiveNoSpace,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			names, fallback := autoCompleteAppNames(tt.cfg)
			if fallback != tt.wantFallback {
				t.Errorf("autoCompleteAppNames() fallback = %v, wantFallback %v", fallback, tt.wantFallback)
				return
			}
			if !reflect.DeepEqual(names, tt.want) {
				t.Errorf("appListNames() got = \n%v\n, want \n%v\n", names, tt.want)
			}
		})
	}
}

func Test_autoCompleteNamespaces(t *testing.T) {
	namespaceA := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespace-a",
		},
	}
	namespaceB := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "namespace-b",
		},
	}

	tests := []struct {
		name string
		cfg  config

		want         []string
		wantFallback cobra.ShellCompDirective
	}{
		{
			name: "show all, no error",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{namespaceA, namespaceB},
			},

			want:         []string{"namespace-a", "namespace-b"},
			wantFallback: cobra.ShellCompDirectiveNoSpace,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			names, fallback := autoCompleteNamespaces(tt.cfg)
			if fallback != tt.wantFallback {
				t.Errorf("autoCompleteNamespaces() fallback = %v, wantFallback %v", fallback, tt.wantFallback)
				return
			}
			if !reflect.DeepEqual(names, tt.want) {
				t.Errorf("autoCompleteNamespace() got = \n%v\n, want \n%v\n", names, tt.want)
			}
		})
	}
}
