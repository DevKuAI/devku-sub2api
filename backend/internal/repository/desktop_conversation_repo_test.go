package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDesktopConversationCommitFailureNeverReturnsReceipt(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "desktop_conversation_records"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit().WillReturnError(errors.New("commit failed"))
	repo := NewDesktopConversationRepository(client)
	receipt, err := repo.Create(context.Background(), 1, 2, &service.DesktopConversationInput{
		RecordID: uuid.NewString(), InstallationID: uuid.NewString(), Client: "workbuddy", SessionID: "source",
		StartedAt: time.Now(), StoppedAt: time.Now(), Prompts: []service.DesktopTextSegment{{Text: "private"}}, CaptureStatus: "response_missing",
	})
	require.ErrorIs(t, err, service.ErrDesktopConversationStorage)
	require.Nil(t, receipt)
	require.NoError(t, mock.ExpectationsWereMet())
}
