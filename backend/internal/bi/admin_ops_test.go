package bi

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestImportStatusReportsExpiredLeasesAsFailed(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	lease := now.Add(-time.Minute)

	require.Equal(t, "failed", importStatus("validating", &lease, now, 1))
	require.Equal(t, "failed", importStatus("queued", &lease, now, 2))
	require.Equal(t, "processing", importStatus("validating", nil, now, 0))
	require.Equal(t, "rejected", importStatus("rejected", nil, now, 1))
}

func TestReportStatusReportsExpiredRunningLeaseAsTimedOut(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	lease := now.Add(-time.Minute)

	require.Equal(t, "lease_timeout", reportStatus("running", &lease, now))
	require.Equal(t, "running", reportStatus("running", nil, now))
	require.Equal(t, "failed", reportStatus("failed", &lease, now))
}

func TestRetryableImportStatusIncludesFailedAndExcludesTerminalStates(t *testing.T) {
	for _, status := range []string{"failed", "rejected", "validating", "queued"} {
		require.True(t, retryableImportStatus(status), status)
	}
	for _, status := range []string{"applied", "running"} {
		require.False(t, retryableImportStatus(status), status)
	}
}
