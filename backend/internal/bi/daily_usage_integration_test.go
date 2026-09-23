//go:build integration

package bi

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDailyUsageSnapshotsTrackCorrectionsAndEmptyDays(t *testing.T) {
	s, c, p, snapshot := analysisFixture(t)
	ctx := context.Background()
	state, err := s.analysisContext(ctx, p, c.OrganizationID, snapshot.ID, "analytics:read")
	require.NoError(t, err)
	total := func(revision int64, start, end string) (string, int64) {
		var tokens string
		var requests int64
		err := integrationDB.QueryRowContext(ctx, `WITH days AS (`+dayRevisionSQL+`) SELECT COALESCE(SUM(f.input_tokens+f.output_tokens),0)::text,COALESCE(SUM(f.requests),0)
			FROM days JOIN bi_usage_daily f ON f.organization_id=$1 AND f.day=days.day AND f.data_revision=days.data_revision WHERE f.team_id='test:a'`, c.OrganizationID, revision, start, end).Scan(&tokens, &requests)
		require.NoError(t, err)
		return tokens, requests
	}
	before, count := total(state.Revision, "2026-09-14", "2026-09-21")
	require.Equal(t, "1500", before)
	require.EqualValues(t, 7, count)
	original, err := recordAt(ctx, s.db, c.OrganizationID, "usage", "test:one:1", -1)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(original.Raw, &payload))
	payload["occurred_at"] = "2026-09-08T04:00:00Z"
	job := applyImport(t, s, c, completeBatch(t, "analytics-seed", "daily-correction", []any{
		map[string]any{"kind": "usage", "revision": 2, "payload": payload},
		map[string]any{"kind": "tombstone", "revision": 2, "entity_kind": "usage", "entity_id": "test:failed:1", "effective_at": "2026-09-22T00:00:00Z", "reason": "corrected"},
	}))
	require.Equal(t, "applied", job.Status, job.Errors)
	newContext, err := s.CreateAnalysisContext(ctx, p, c.OrganizationID, Filters{Period: "week", TeamIDs: []string{"test:a"}})
	require.NoError(t, err)
	newState, err := s.analysisContext(ctx, p, c.OrganizationID, newContext.ID, "analytics:read")
	require.NoError(t, err)
	after, count := total(newState.Revision, "2026-09-14", "2026-09-21")
	require.Equal(t, "1400", after)
	require.EqualValues(t, 5, count)
	unchanged, count := total(state.Revision, "2026-09-14", "2026-09-21")
	require.Equal(t, before, unchanged)
	require.EqualValues(t, 7, count)
	_, emptyCount := total(newState.Revision, "2026-09-19", "2026-09-20")
	require.Zero(t, emptyCount)
	previous, _ := total(newState.Revision, "2026-09-07", "2026-09-14")
	require.Equal(t, "120", previous)
}
