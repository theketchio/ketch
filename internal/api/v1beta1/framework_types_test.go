package v1beta1

import (
	"testing"
)

func TestFramework_HasApp(t *testing.T) {
	tests := []struct {
		name    string
		appName string
		apps    []string
		want    bool
	}{
		{
			name:    "framework has the app",
			apps:    []string{"ketch", "theketch"},
			appName: "ketch",
			want:    true,
		},
		{
			name:    "framework has no app",
			apps:    []string{"ketch", "theketch"},
			appName: "dashboard",
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Framework{
				Status: FrameworkStatus{
					Apps: tt.apps,
				},
			}
			if got := p.HasApp(tt.appName); got != tt.want {
				t.Errorf("HasApp() = %v, want %v", got, tt.want)
			}
		})
	}
}
