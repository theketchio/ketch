package controllers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_uninstallHelmChart(t *testing.T) {
	tests := []struct {
		name          string
		group         string
		annotations   map[string]string
		wantUninstall bool
	}{
		{
			name:  "dont uninstall",
			group: "theketch.io",
			annotations: map[string]string{
				"theketch.io/dont-uninstall-helm-chart": "true",
			},
			wantUninstall: false,
		},
		{
			name:  "uninstall",
			group: "theketch.io",
			annotations: map[string]string{
				"theketch.io/dont-uninstall-helm-chart": "some-value",
			},
			wantUninstall: true,
		},
		{
			name:          "no annotation - uninstall",
			group:         "theketch.io",
			annotations:   map[string]string{},
			wantUninstall: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uninstall := uninstallHelmChart(tt.group, tt.annotations)
			require.Equal(t, tt.wantUninstall, uninstall)
		})
	}
}
