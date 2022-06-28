package templates

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIngressConfigMapName(t *testing.T) {
	tests := []struct {
		name        string
		ingressType string

		want string
	}{
		{
			name:        "istio configmap",
			ingressType: "istio",
			want:        "ingress-istio-templates",
		},
		{
			name:        "traefik configmap",
			ingressType: "traefik",
			want:        "ingress-traefik-templates",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configMapName := IngressConfigMapName(tt.ingressType)
			require.Equal(t, tt.want, configMapName)
		})
	}
}

func TestJobConfigMapName(t *testing.T) {
	require.Equal(t, "job-templates", JobConfigMapName())
}

func TestCronJobConfigMapName(t *testing.T) {
	require.Equal(t, "cronjob-templates", CronJobConfigMapName())
}
