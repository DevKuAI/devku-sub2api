package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestUsageLogRepositoryGetDesktopMembersUsage(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	todayStart := time.Date(2026, 9, 2, 16, 0, 0, 0, time.UTC)
	last30DaysStart := time.Date(2026, 8, 4, 6, 30, 0, 0, time.UTC)
	endTime := time.Date(2026, 9, 3, 6, 30, 0, 0, time.UTC)

	mock.ExpectQuery(`(?s)SELECT\s+assignment\.member_id,.+FROM desktop_member_api_keys assignment.+LEFT JOIN usage_logs ul ON ul\.api_key_id = assignment\.api_key_id.+WHERE assignment\.member_id = ANY\(\$1\).+GROUP BY assignment\.member_id`).
		WithArgs(sqlmock.AnyArg(), todayStart, last30DaysStart, endTime).
		WillReturnRows(sqlmock.NewRows([]string{
			"member_id", "today_tokens", "last_30_days_tokens", "total_tokens",
			"today_actual_cost", "last_30_days_actual_cost", "total_actual_cost",
		}).
			AddRow(7, 100, 900, 1200, 0.1, 0.9, 1.2).
			AddRow(9, 999, 999, 999, 9.9, 9.9, 9.9))

	result, err := repo.GetDesktopMembersUsage(
		context.Background(),
		[]int64{7, 8, 7, -1},
		todayStart,
		last30DaysStart,
		endTime,
	)

	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, int64(100), result[7].TodayTokens)
	require.Equal(t, int64(900), result[7].Last30DaysTokens)
	require.Equal(t, int64(1200), result[7].TotalTokens)
	require.InDelta(t, 0.1, result[7].TodayActualCost, 0.0000001)
	require.InDelta(t, 0.9, result[7].Last30DaysActualCost, 0.0000001)
	require.InDelta(t, 1.2, result[7].TotalActualCost, 0.0000001)
	require.NotNil(t, result[8])
	require.Zero(t, result[8].TotalTokens)
	require.NotContains(t, result, int64(9))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetDesktopMembersUsageEmpty(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	result, err := repo.GetDesktopMembersUsage(context.Background(), nil, time.Time{}, time.Time{}, time.Time{})

	require.NoError(t, err)
	require.Empty(t, result)
	require.NoError(t, mock.ExpectationsWereMet())
}
