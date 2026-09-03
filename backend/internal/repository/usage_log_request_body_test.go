package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageLogRepositoryGetRequestBody(t *testing.T) {
	const query = "SELECT request_body FROM usage_logs WHERE id = \\$1"

	t.Run("found", func(t *testing.T) {
		db, mock := newSQLMock(t)
		repo := &usageLogRepository{sql: db}

		mock.ExpectQuery(query).
			WithArgs(int64(42)).
			WillReturnRows(sqlmock.NewRows([]string{"request_body"}).AddRow(`{"input":"hello"}`))

		body, err := repo.GetRequestBody(context.Background(), 42)
		require.NoError(t, err)
		require.JSONEq(t, `{"input":"hello"}`, body)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("null", func(t *testing.T) {
		db, mock := newSQLMock(t)
		repo := &usageLogRepository{sql: db}

		mock.ExpectQuery(query).
			WithArgs(int64(43)).
			WillReturnRows(sqlmock.NewRows([]string{"request_body"}).AddRow(nil))

		_, err := repo.GetRequestBody(context.Background(), 43)
		require.ErrorIs(t, err, service.ErrUsageLogRequestBodyNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("not_found", func(t *testing.T) {
		db, mock := newSQLMock(t)
		repo := &usageLogRepository{sql: db}

		mock.ExpectQuery(query).
			WithArgs(int64(44)).
			WillReturnRows(sqlmock.NewRows([]string{"request_body"}))

		_, err := repo.GetRequestBody(context.Background(), 44)
		require.ErrorIs(t, err, service.ErrUsageLogRequestBodyNotFound)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestPrepareUsageLogInsertRequestBody(t *testing.T) {
	body := `{"messages":[{"role":"user","content":"hello"}]}`
	log := &service.UsageLog{
		UserID:      1,
		APIKeyID:    2,
		AccountID:   3,
		RequestID:   "req-request-body",
		Model:       "claude-3",
		RequestBody: &body,
		CreatedAt:   time.Now().UTC(),
	}

	prepared := prepareUsageLogInsert(log)
	requestBodyArg, ok := prepared.args[len(prepared.args)-2].(sql.NullString)
	require.True(t, ok)
	require.True(t, requestBodyArg.Valid)
	require.Equal(t, body, requestBodyArg.String)

	log.RequestBody = nil
	prepared = prepareUsageLogInsert(log)
	requestBodyArg, ok = prepared.args[len(prepared.args)-2].(sql.NullString)
	require.True(t, ok)
	require.False(t, requestBodyArg.Valid)
}
