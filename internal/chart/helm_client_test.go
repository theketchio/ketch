package chart

import (
	"testing"
	"time"

	log "github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	helmTime "helm.sh/helm/v3/pkg/time"
)

func TestIsHelmChartStatusActionable(t *testing.T) {
	tests := []struct {
		description   string
		status        release.Status
		statusMap     map[release.Status]int // helmStatusActionMapUpdate or helmStatusActionMapDelete
		expected      bool
		expectedError string
	}{
		{
			description: "update - deployed",
			status:      release.StatusDeployed,
			statusMap:   helmStatusActionMapUpdate,
			expected:    true,
		},
		{
			description: "update - not found",
			status:      notFound,
			statusMap:   helmStatusActionMapUpdate,
			expected:    true,
		},
		{
			description:   "update - unknown",
			status:        release.StatusUnknown,
			statusMap:     helmStatusActionMapUpdate,
			expected:      false,
			expectedError: "helm chart for app testapp in non-actionable status unknown",
		},
		{
			description: "update - superseded",
			status:      release.StatusSuperseded,
			statusMap:   helmStatusActionMapUpdate,
			expected:    false,
		},
		{
			description: "delete - deployed",
			status:      release.StatusDeployed,
			statusMap:   helmStatusActionMapDelete,
			expected:    true,
		},
		{
			description: "delete - not found",
			status:      notFound,
			statusMap:   helmStatusActionMapDelete,
			expected:    false,
		},
		{
			description:   "delete - unknown",
			status:        release.StatusUnknown,
			statusMap:     helmStatusActionMapDelete,
			expected:      false,
			expectedError: "helm chart for app testapp in non-actionable status unknown",
		},
		{
			description: "delete - superseded",
			status:      release.StatusSuperseded,
			statusMap:   helmStatusActionMapDelete,
			expected:    false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			c := &HelmClient{
				log: log.Discard(),
			}
			mockStatusFunc := func(cfg *action.Configuration, appName string) (*release.Release, release.Status, error) {
				status := tc.status
				currentRelease := &release.Release{Info: &release.Info{FirstDeployed: helmTime.Time{Time: time.Now()}}}
				return currentRelease, status, nil
			}

			ok, err := c.isHelmChartStatusActionable(mockStatusFunc, "testapp", tc.statusMap)
			if tc.expectedError != "" {
				require.EqualError(t, err, tc.expectedError)
			} else {
				require.Nil(t, err)
			}
			require.Equal(t, tc.expected, ok)
		})
	}
}
