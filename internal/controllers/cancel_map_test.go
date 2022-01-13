package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCancelMap_replaceAndCancelPrevious(t *testing.T) {
	m := NewCancelMap()

	ctx, cancel := context.WithCancel(context.Background())
	cleanup := m.replaceAndCancelPrevious("myapp-web-1", cancel)
	cleanup()

	require.Equal(t, context.Canceled, ctx.Err())
	require.Equal(t, 0, len(m.cancelGeneration))
	require.Equal(t, 0, len(m.cancelMap))
}

func TestCancelMap_replaceAndCancelPrevious_TwoDifferentDeployments(t *testing.T) {
	m := NewCancelMap()

	ctxA, cancelA := context.WithCancel(context.Background())
	cleanupA := m.replaceAndCancelPrevious("myapp-web-1", cancelA)

	ctxB, cancelB := context.WithCancel(context.Background())
	cleanupB := m.replaceAndCancelPrevious("myapp-web-2", cancelB)

	require.Equal(t, 2, len(m.cancelGeneration))
	require.Equal(t, 2, len(m.cancelMap))

	cleanupA()

	require.Equal(t, context.Canceled, ctxA.Err())
	require.Equal(t, 1, len(m.cancelGeneration))
	require.Equal(t, 1, len(m.cancelMap))

	cleanupB()

	require.Equal(t, context.Canceled, ctxB.Err())
	require.Equal(t, 0, len(m.cancelGeneration))
	require.Equal(t, 0, len(m.cancelMap))
}

func TestCancelMap_replaceAndCancelPrevious_TwoCleanupFunctionsForOneDeployment(t *testing.T) {
	m := NewCancelMap()

	ctx1, cancel1 := context.WithCancel(context.Background())
	cleanup1 := m.replaceAndCancelPrevious("myapp-web-1", cancel1)

	ctx2, cancel2 := context.WithCancel(context.Background())
	cleanup2 := m.replaceAndCancelPrevious("myapp-web-1", cancel2)

	cleanup1()

	require.Equal(t, context.Canceled, ctx1.Err())
	require.Equal(t, 1, len(m.cancelGeneration))
	require.Equal(t, 1, len(m.cancelMap))

	cleanup2()

	require.Equal(t, context.Canceled, ctx2.Err())
	require.Equal(t, 0, len(m.cancelGeneration))
	require.Equal(t, 0, len(m.cancelMap))
}
