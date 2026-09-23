//go:build integration

package bi

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPeriodReadUsesLatestEventTimeWithoutResurrectingOldVersions(t *testing.T) {
	s, c, p, oldContext := analysisFixture(t)
	ctx := context.Background()
	changes := []any{}
	for _, change := range []struct{ id, at string }{{"test:one:1", "2026-09-08T04:00:00Z"}, {"test:history:two:2", "2026-09-16T04:00:00Z"}} {
		original, err := recordAt(ctx, s.db, c.OrganizationID, "usage", change.id, -1)
		require.NoError(t, err)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(original.Raw, &payload))
		payload["occurred_at"] = change.at
		changes = append(changes, map[string]any{"kind": "usage", "revision": 2, "payload": payload})
	}
	job := applyImport(t, s, c, completeBatch(t, "analytics-seed", "correct-event-times", changes))
	require.Equal(t, "applied", job.Status, job.Errors)
	newContext, err := s.CreateAnalysisContext(ctx, p, c.OrganizationID, Filters{Period: "week", TeamIDs: []string{"test:a"}})
	require.NoError(t, err)
	for _, tc := range []struct {
		snapshot      AnalysisContext
		first, second bool
	}{{oldContext, true, false}, {newContext, false, true}} {
		state, err := s.analysisContext(ctx, p, c.OrganizationID, tc.snapshot.ID, "analytics:read")
		require.NoError(t, err)
		frame, err := s.loadMetadata(ctx, state)
		require.NoError(t, err)
		facts, err := frame.periodFacts(analysisSelection{})
		require.NoError(t, err)
		seen := map[string]bool{}
		for _, fact := range facts {
			seen[fact.ID] = true
		}
		require.Equal(t, tc.first, seen["test:one:1"])
		require.Equal(t, tc.second, seen["test:history:two:2"])
	}
}
