package chart

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func strRef(s string) *string {
	return &s
}

func Test_newIngress(t *testing.T) {
	tests := []struct {
		name         string
		appName      string
		cnames       []string
		defaultCname *string

		want ingress
	}{
		{
			name:    "custom cnames + default cname",
			appName: "ketch",
			cnames: []string{
				"theketch.io", "theketch.cloud",
			},
			defaultCname: strRef("ketch.10.10.10.10.shipa.cloud"),
			want: ingress{
				Https: []httpsEndpoint{
					{Cname: "theketch.io", SecretName: "ketch-cname-7698da46d42bea3603f2"},
					{Cname: "theketch.cloud", SecretName: "ketch-cname-3913eadb7576bef699fc"},
				},
				Http: []string{
					"ketch.10.10.10.10.shipa.cloud",
				},
			},
		},
		{
			name:    "custom cnames",
			appName: "ketch",
			cnames: []string{
				"theketch.io", "theketch.cloud",
			},
			want: ingress{
				Https: []httpsEndpoint{
					{Cname: "theketch.io", SecretName: "ketch-cname-7698da46d42bea3603f2"},
					{Cname: "theketch.cloud", SecretName: "ketch-cname-3913eadb7576bef699fc"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newIngress(tt.appName, tt.cnames, tt.defaultCname)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("newIngress() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
