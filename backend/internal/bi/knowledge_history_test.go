package bi

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestKnowledgeStatusHistoryLoadsOncePerFrameAndPreservesTimeOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	when := func(day int) time.Time { return time.Date(2026, 9, day, 0, 0, 0, 0, time.UTC) }
	frame := func(revision int64) *analysisFrame {
		return &analysisFrame{ctx: context.Background(), service: &Service{db: db}, state: analysisState{Revision: revision, Context: AnalysisContext{OrganizationID: "org"}}}
	}
	current := frame(3)
	mock.ExpectQuery("FROM bi_entity_versions").WithArgs("org", "knowledge", int64(3)).WillReturnRows(sqlmock.NewRows([]string{"effective_at", "status"}).
		AddRow(when(17), "valid").AddRow(when(17), "expired").AddRow(when(16), "expired").AddRow(when(1), "valid"))
	for _, tc := range []struct {
		at      time.Time
		expired bool
	}{{when(1).Add(-time.Nanosecond), false}, {when(15), false}, {when(16), true}, {when(16).Add(time.Hour), true}, {when(17), false}, {when(19), false}} {
		expired, err := current.expiredKnowledgeAt("knowledge", tc.at)
		require.NoError(t, err)
		require.Equal(t, tc.expired, expired, tc.at)
	}
	mock.ExpectQuery("FROM bi_entity_versions").WithArgs("org", "knowledge", int64(2)).WillReturnRows(sqlmock.NewRows([]string{"effective_at", "status"}).
		AddRow(when(16), "expired").AddRow(when(1), "valid"))
	expired, err := frame(2).expiredKnowledgeAt("knowledge", when(19))
	require.NoError(t, err)
	require.True(t, expired)
	mock.ExpectQuery("FROM bi_entity_versions").WithArgs("org", "missing", int64(3)).WillReturnRows(sqlmock.NewRows([]string{"effective_at", "status"}))
	for i := 0; i < 2; i++ {
		expired, err = current.expiredKnowledgeAt("missing", when(19))
		require.NoError(t, err)
		require.False(t, expired)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}
