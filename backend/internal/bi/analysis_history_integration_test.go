//go:build integration

package bi

import (
	"context"
	"fmt"
	"maps"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func compareAnalysisWithFullHistory(t *testing.T, s *Service, state analysisState) (int, int) {
	t.Helper()
	ctx := context.Background()
	compact, err := s.loadAnalysis(ctx, state)
	require.NoError(t, err)
	full, err := s.loadMetadata(ctx, state)
	require.NoError(t, err)
	full.facts, err = s.readUsageFacts(ctx, state, nil, nil)
	require.NoError(t, err)
	full.factsLoaded = true
	for _, selection := range []analysisSelection{{}, {TeamID: "test:a"}, {TeamID: "test:b"}, {RoleID: "test:role"}, {RoleID: "test:role-new"}, {MemberID: "test:one"}, {ApplicationID: "test:app"}, {SceneID: "test:scene"}} {
		require.Equal(t, full.statistics(selection).stats, compact.statistics(selection).stats, selection)
		require.Equal(t, full.adoption(selection), compact.adoption(selection), selection)
		require.Equal(t, full.tokenAnalysis(selection), compact.tokenAnalysis(selection), selection)
		require.Equal(t, full.trend(selection, "day"), compact.trend(selection, "day"), selection)
		require.Equal(t, full.trend(selection, "seven_day"), compact.trend(selection, "seven_day"), selection)
	}
	return len(full.facts), len(compact.facts)
}

func TestAnalysisHistoryCompactionPreservesMetricsAndDirectoryChanges(t *testing.T) {
	s, c, p, _ := analysisFixture(t)
	ctx := context.Background()
	base := acceptancePayload(t, s, c, "usage", "test:one:1")
	records := []any{}
	for i := 0; i < 240; i++ {
		usage := maps.Clone(base)
		usage["id"] = fmt.Sprintf("test:compact:%03d", i)
		usage["occurred_at"] = time.Date(2026, 8, 25, 4, 0, i, 0, time.UTC).Format(time.RFC3339Nano)
		if i >= 80 && i < 160 {
			usage["actor_type"], usage["member_id"] = "automatic", nil
		} else if i >= 160 {
			usage["actor_type"], usage["member_id"] = "unknown", nil
		}
		records = append(records, analyticRecord("usage", usage))
	}
	afterChange := maps.Clone(base)
	afterChange["id"], afterChange["occurred_at"] = "test:after-role-change", "2026-08-25T06:00:00Z"
	outside := maps.Clone(base)
	outside["id"], outside["occurred_at"], outside["team_id"], outside["outcome"] = "test:outside-failure", "2026-09-01T06:00:00Z", "test:b", "failed"
	records = append(records, analyticRecord("usage", afterChange), analyticRecord("usage", outside))
	job := applyImport(t, s, c, completeBatch(t, "analytics-seed", "history-compaction", records))
	require.Equal(t, "applied", job.Status, job.Errors)
	contexts := []analysisState{}
	for _, period := range []string{"week", "current", "month"} {
		snapshot, err := s.CreateAnalysisContext(ctx, p, c.OrganizationID, Filters{Period: period, TeamIDs: []string{"test:a"}})
		require.NoError(t, err)
		state, err := s.analysisContext(ctx, p, c.OrganizationID, snapshot.ID, "analytics:read")
		require.NoError(t, err)
		contexts = append(contexts, state)
		full, compact := compareAnalysisWithFullHistory(t, s, state)
		if period == "week" {
			require.Less(t, compact, full/4)
		}
	}
	// Rebuild from the context's directory revision, including intra-day changes.
	membership := acceptancePayload(t, s, c, "membership", "test:membership:one")
	membership["valid_to"] = "2026-08-25T05:00:00.000000001Z"
	changes := []any{
		analyticRecord("role", map[string]any{"id": "test:role-new", "name": "New role", "status": "active"}),
		map[string]any{"kind": "membership", "revision": 2, "payload": membership},
		analyticRecord("membership", map[string]any{"id": "test:membership:one:new", "member_id": "test:one", "team_id": "test:a", "role_id": "test:role-new", "valid_from": "2026-08-25T05:00:00.000000001Z", "valid_to": nil, "primary": true}),
	}
	move := acceptancePayload(t, s, c, "usage", "test:compact:000")
	move["occurred_at"] = "2026-09-15T04:00:00Z"
	changes = append(changes, map[string]any{"kind": "usage", "revision": 2, "payload": move}, map[string]any{"kind": "tombstone", "entity_kind": "usage", "entity_id": "test:compact:001", "revision": 2, "effective_at": "2026-09-23T00:00:00Z", "reason": "corrected"})
	job = applyImport(t, s, c, completeBatch(t, "history-compaction", "history-corrected", changes))
	require.Equal(t, "applied", job.Status, job.Errors)
	for _, old := range contexts {
		compareAnalysisWithFullHistory(t, s, old)
		snapshot, err := s.CreateAnalysisContext(ctx, p, c.OrganizationID, old.Context.Filters)
		require.NoError(t, err)
		state, err := s.analysisContext(ctx, p, c.OrganizationID, snapshot.ID, "analytics:read")
		require.NoError(t, err)
		compareAnalysisWithFullHistory(t, s, state)
		state.Scope.AllTeams = false
		state.Scope.TeamIDs = []string{"test:a"}
		compareAnalysisWithFullHistory(t, s, state)
	}
}
