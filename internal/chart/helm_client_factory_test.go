package chart

import (
	"testing"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/action"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestHelmClientFactory_NewHelmClient_VerifyCacheIsUsed(t *testing.T) {

	getActionConfigIsCalled := map[string]int{}

	factory := &HelmClientFactory{
		configurations: map[string]*action.Configuration{},
		getActionConfig: func(namespace string) (*action.Configuration, error) {
			getActionConfigIsCalled[namespace] += 1
			return &action.Configuration{}, nil
		},
	}
	cli, err := factory.NewHelmClient("my-namespace", nil, log.NullLogger{})
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, map[string]int{"my-namespace": 1}, getActionConfigIsCalled)

	cli, err = factory.NewHelmClient("my-namespace", nil, log.NullLogger{})
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, map[string]int{"my-namespace": 1}, getActionConfigIsCalled)

	cli, err = factory.NewHelmClient("another-namespace", nil, log.NullLogger{})
	require.Nil(t, err)
	require.NotNil(t, cli)
	require.Equal(t, map[string]int{"my-namespace": 1, "another-namespace": 1}, getActionConfigIsCalled)
}
