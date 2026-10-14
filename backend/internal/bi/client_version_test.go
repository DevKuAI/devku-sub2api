package bi

import (
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBIClientVersionGate(t *testing.T) {
	require.NoError(t, requireClientVersion("0.2.0", "0.1.0"))
	require.NoError(t, requireClientVersion("0.2.0", "0.2.0"))
	for _, current := range []string{"0.1.9", "0.2.0-beta.1"} {
		var err *Error
		require.True(t, errors.As(requireClientVersion(current, "0.2.0"), &err))
		require.Equal(t, 426, err.Status)
		require.Equal(t, "0.2.0", err.MinClientVersion)
		require.False(t, err.Retryable)
	}
	require.Error(t, requireClientVersion("invalid", "0.2.0"))
}
