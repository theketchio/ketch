package main

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"
	"github.com/theketchio/ketch/internal/mocks"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func Test_appList(t *testing.T) {
	appA := &ketchv1.App{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "app-a",
		},
		Spec: ketchv1.AppSpec{
			Description: "my app-a",
			Framework:   "fw1",
			Ingress: ketchv1.IngressSpec{
				GenerateDefaultCname: false,
				Cnames:               ketchv1.CnameList{{Name: "app-a-cname1"}},
			},
			Builder: "",
		},
	}
	appB := &ketchv1.App{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "app-b",
		},
		Spec: ketchv1.AppSpec{
			Description: "my app-b",
			Framework:   "fw1",
			Ingress: ketchv1.IngressSpec{
				GenerateDefaultCname: false,
				Cnames:               ketchv1.CnameList{{Name: "app-b-cname1"}},
			},
			Builder: "",
		},
	}

	tests := []struct {
		name string
		cfg  config

		wantOut string
		wantErr bool
	}{
		{
			name: "update service endpoint",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{appA, appB},
			},
			wantOut: `NAME     FRAMEWORK    STATE      ADDRESSES              BUILDER    DESCRIPTION
app-a    fw1          created    http://app-a-cname1               my app-a
app-b    fw1          created    http://app-b-cname1               my app-b
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			err := appList(context.Background(), tt.cfg, out)
			if (err != nil) != tt.wantErr {
				t.Errorf("frameworkList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotOut := out.String(); gotOut != tt.wantOut {
				t.Errorf("frameworkList() gotOut = \n%v\n, want \n%v\n", gotOut, tt.wantOut)
			}
		})
	}
}

func Test_appListNames(t *testing.T) {
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

		filter  []string
		want    []string
		wantErr bool
	}{
		{
			name: "no filter show all",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{appA, appB},
			},

			filter:  []string{},
			want:    []string{"app-a", "app-b"},
			wantErr: false,
		},
		{
			name: "filtered, show app-a only",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{appA, appB},
			},

			filter:  []string{"-a"},
			want:    []string{"app-a"},
			wantErr: false,
		},
		{
			name: "no result, random filter",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{appA, appB},
			},

			filter:  []string{"foo"},
			want:    []string{},
			wantErr: false,
		},
		{
			name: "filtered, random filter and app-a filter",
			cfg: &mocks.Configuration{
				CtrlClientObjects: []runtime.Object{appA, appB},
			},

			filter:  []string{"foo", "-a"},
			want:    []string{"app-a"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			names, err := appListNames(tt.cfg, tt.filter...)
			if (err != nil) != tt.wantErr {
				t.Errorf("appListNames() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(names, tt.want) {
				t.Errorf("appListNames() got = \n%v\n, want \n%v\n", names, tt.want)
			}
		})
	}
}
