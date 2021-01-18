package docker


import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)


func TestNew(t *testing.T) {
	c, err := New(context.Background())
	require.Nil(t, err )
	require.NotNil(t, c )
}

