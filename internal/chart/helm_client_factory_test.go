package chart

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/action"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestHelmClientFactory_NewHelmClient_VerifyCacheIsUsed(t *testing.T) {

	getActionConfigIsCalled := map[string]int{}

	factory := &HelmClientFactory{
		configurations:              map[string]*action.Configuration{},
		configurationsLastUsedTimes: map[string]time.Time{},
		getActionConfig: func(namespace string) (*action.Configuration, error) {
			getActionConfigIsCalled[namespace] += 1
			return &action.Configuration{}, nil
		},
	}
	now := time.Now()
	cli, err := factory.NewHelmClient("my-namespace", nil, log.NullLogger{})
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.True(t, factory.configurationsLastUsedTimes["my-namespace"].After(now))
	require.True(t, factory.configurationsLastUsedTimes["my-namespace"].Before(time.Now()))
	require.Equal(t, map[string]int{"my-namespace": 1}, getActionConfigIsCalled)

	now = time.Now()
	cli, err = factory.NewHelmClient("my-namespace", nil, log.NullLogger{})
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, map[string]int{"my-namespace": 1}, getActionConfigIsCalled)
	require.True(t, factory.configurationsLastUsedTimes["my-namespace"].After(now))
	require.True(t, factory.configurationsLastUsedTimes["my-namespace"].Before(time.Now()))

	cli, err = factory.NewHelmClient("another-namespace", nil, log.NullLogger{})
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, map[string]int{"my-namespace": 1, "another-namespace": 1}, getActionConfigIsCalled)
}

func TestHelmClientFactory_cleanup(t *testing.T) {
	now := time.Now()
	factory := &HelmClientFactory{
		configurations: map[string]*action.Configuration{
			"namespace-1": {},
			"namespace-2": {},
			"namespace-3": {},
			"namespace-4": {},
		},
		configurationsLastUsedTimes: map[string]time.Time{
			"namespace-1": now.Add(-20 * time.Minute),
			"namespace-2": now.Add(-5 * time.Minute),
			"namespace-3": now.Add(-20 * time.Minute),
			"namespace-4": now.Add(-5 * time.Minute),
		},
	}

	factory.lastCleanupTime = now.Add(-1 * time.Minute)
	factory.cleanup()
	require.Equal(t, 4, len(factory.configurations))
	require.Equal(t, 4, len(factory.configurationsLastUsedTimes))
	require.Equal(t, factory.lastCleanupTime, now.Add(-1*time.Minute))

	factory.lastCleanupTime = now.Add(-16 * time.Minute)
	factory.cleanup()
	require.Equal(t, map[string]*action.Configuration{
		"namespace-2": {},
		"namespace-4": {},
	}, factory.configurations)
	require.Equal(t, map[string]time.Time{
		"namespace-2": now.Add(-5 * time.Minute),
		"namespace-4": now.Add(-5 * time.Minute),
	}, factory.configurationsLastUsedTimes)
	require.True(t, factory.lastCleanupTime.After(now))
}
