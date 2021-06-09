package main

import (
	"reflect"
	"testing"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/mocks"
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

func Test_autoCompleteFrameworkNames(t *testing.T) {
	frameworkA := &ketchv1.Framework{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "framework-a",
		},
	}
	frameworkB := &ketchv1.Framework{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "framework-b",
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
				CtrlClientObjects: []runtime.Object{frameworkA, frameworkB},
			},

			want:         []string{"framework-a", "framework-b"},
			wantFallback: cobra.ShellCompDirectiveNoSpace,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			names, fallback := autoCompleteFrameworkNames(tt.cfg)
			if fallback != tt.wantFallback {
				t.Errorf("autoCompleteFrameworkNames() fallback = %v, wantFallback %v", fallback, tt.wantFallback)
				return
			}
			if !reflect.DeepEqual(names, tt.want) {
				t.Errorf("autoCompleteFrameworkNames() got = \n%v\n, want \n%v\n", names, tt.want)
			}
		})
	}
}
