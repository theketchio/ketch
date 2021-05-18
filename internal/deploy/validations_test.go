package deploy

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestErrors(t *testing.T) {
	require.True(t, isMissing(NewMissingError("oops")))
	require.False(t, isValid(NewInvalidError("oops")))
	require.True(t, isMissing(fmt.Errorf("some error %w", NewMissingError("oops"))))
	require.False(t, isValid(fmt.Errorf("some error %w", NewInvalidError("oops"))))
}
