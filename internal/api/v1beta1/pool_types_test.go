package v1beta1

import (
	"testing"
)

func TestPool_HasApp(t *testing.T) {
	tests := []struct {
		name    string
		appName string
		apps    []string
		want    bool
	}{
		{
			name:    "pool has the app",
			apps:    []string{"ketch", "theketch"},
			appName: "ketch",
			want:    true,
		},
		{
			name:    "pool has no app",
			apps:    []string{"ketch", "theketch"},
			appName: "dashboard",
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pool{
				Status: PoolStatus{
					Apps: tt.apps,
				},
			}
			if got := p.HasApp(tt.appName); got != tt.want {
				t.Errorf("HasApp() = %v, want %v", got, tt.want)
			}
		})
	}
}
