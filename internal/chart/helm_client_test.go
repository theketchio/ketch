package chart

import (
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
)

func TestWaitForActionableStatus(t *testing.T) {
	tests := []struct {
		description             string
		statusFuncMap           map[int]release.Status // sequence of responses returned by mockStatusFunc
		statusMap               map[release.Status]int // test update or delete
		expected                bool
		expectedStatusFuncCalls int
	}{
		{
			description:             "delete - deployed",
			statusFuncMap:           map[int]release.Status{0: release.StatusDeployed},
			statusMap:               helmStatusActionMapDelete,
			expected:                true,
			expectedStatusFuncCalls: 1,
		},
		{
			description:             "delete - eventual success",
			statusFuncMap:           map[int]release.Status{0: release.StatusUnknown, 1: release.StatusDeployed},
			statusMap:               helmStatusActionMapDelete,
			expected:                true,
			expectedStatusFuncCalls: 2,
		},
		{
			description:             "delete - not found",
			statusFuncMap:           map[int]release.Status{0: "not-found"},
			statusMap:               helmStatusActionMapDelete,
			expected:                false,
			expectedStatusFuncCalls: 1,
		},
		{
			description:             "delete - superseded",
			statusFuncMap:           map[int]release.Status{0: release.StatusSuperseded},
			statusMap:               helmStatusActionMapDelete,
			expected:                false,
			expectedStatusFuncCalls: 1,
		},
		{
			description:             "delete timeout",
			statusFuncMap:           map[int]release.Status{0: release.StatusPendingInstall, 1: release.StatusPendingInstall, 2: release.StatusPendingInstall, 3: release.StatusPendingInstall, 4: release.StatusPendingInstall},
			statusMap:               helmStatusActionMapDelete,
			expected:                true,
			expectedStatusFuncCalls: 5,
		},
		{
			description:             "update - deployed",
			statusFuncMap:           map[int]release.Status{0: release.StatusDeployed},
			statusMap:               helmStatusActionMapUpdate,
			expected:                true,
			expectedStatusFuncCalls: 1,
		},
		{
			description:             "update - eventual success",
			statusFuncMap:           map[int]release.Status{0: release.StatusUnknown, 1: release.StatusDeployed},
			statusMap:               helmStatusActionMapUpdate,
			expected:                true,
			expectedStatusFuncCalls: 2,
		},
		{
			description:             "update - not found",
			statusFuncMap:           map[int]release.Status{0: "not-found"},
			statusMap:               helmStatusActionMapUpdate,
			expected:                true,
			expectedStatusFuncCalls: 1,
		},
		{
			description:             "update - superseded",
			statusFuncMap:           map[int]release.Status{0: release.StatusSuperseded},
			statusMap:               helmStatusActionMapUpdate,
			expected:                false,
			expectedStatusFuncCalls: 1,
		},
		{
			description:             "update timeout",
			statusFuncMap:           map[int]release.Status{0: release.StatusPendingInstall, 1: release.StatusPendingInstall, 2: release.StatusPendingInstall, 3: release.StatusPendingInstall, 4: release.StatusPendingInstall},
			statusMap:               helmStatusActionMapUpdate,
			expected:                true,
			expectedStatusFuncCalls: 5,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			c := &HelmClient{
				log: log.NullLogger{},
			}
			// speed up retry, timeout
			statusRetryInterval = time.Millisecond * 100
			statusRetryTimeout = time.Millisecond * 500
			// mockStatusFunc and counter to track times called
			counter := 0
			mockStatusFunc := func(cfg *action.Configuration, appName string) (*release.Release, release.Status, error) {
				status := tc.statusFuncMap[counter]
				counter += 1
				mockRelease := &release.Release{Chart: &chart.Chart{}, Info: &release.Info{}}
				return mockRelease, status, nil
			}

			ok, err := c.waitForActionableStatus(mockStatusFunc, "testapp", tc.statusMap)
			require.Nil(t, err)
			require.Equal(t, tc.expected, ok)
			require.Equal(t, tc.expectedStatusFuncCalls, counter)
		})
	}
}
