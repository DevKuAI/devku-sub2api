package bi

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestListAdminBindingsJoinsManagerAndUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM bi_wechat_bindings b")).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("FROM bi_wechat_bindings b JOIN bi_managers m ON m.id=b.manager_id LEFT JOIN users u ON u.id=m.user_id")).
		WithArgs(20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "display_name", "status", "created_at", "last_login_at", "revoked_at", "session_count"}).
			AddRow("binding-1", int64(7), "Alice", "active", now, nil, nil, int64(2)))
	mock.ExpectClose()

	items, total, err := (&Service{db: db}).listAdminBindings(context.Background(), 1, 20, "")
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, []AdminBinding{{ID: "binding-1", UserID: 7, DisplayName: "Alice", Status: "active", CreatedAt: now, SessionCount: 2}}, items)
	require.NoError(t, db.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}

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
