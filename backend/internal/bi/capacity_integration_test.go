//go:build integration && bi_capacity

package bi

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// This opt-in baseline reports measured latency; it is not a production SLA.
func TestBICapacityBaseline(t *testing.T) {
	s, c, p, _ := analysisFixture(t)
	ctx := context.Background()
	checkpoint := "analytics-seed"
	const calls = 10000
	started := time.Now()
	for offset := 0; offset < calls; offset += 500 {
		records := make([]any, 0, 500)
		for i := offset; i < offset+500; i++ {
			records = append(records, analyticRecord("usage", map[string]any{
				"id": fmt.Sprintf("test:capacity:%05d", i), "occurred_at": "2026-09-15T04:00:00Z", "actor_type": "automatic", "member_id": nil, "team_id": "test:a",
				"application_id": "test:app", "application_version_id": "test:app:v1", "scene_id": "test:scene", "requested_model": fmt.Sprintf("model-%d", i%10), "outcome": "succeeded", "duration_ms": nil, "retry_of": nil,
				"tokens": map[string]any{"encoding": "exclusive_buckets", "input": "100", "output": "20", "cache_read": "40", "cache_write": "10"},
			}))
		}
		next := fmt.Sprintf("capacity-%d", offset)
		job := applyImport(t, s, c, completeBatch(t, checkpoint, next, records))
		require.Equal(t, "applied", job.Status, job.Errors)
		checkpoint = next
	}
	importDuration := time.Since(started)
	contextLatency, overviewLatency := []time.Duration{}, []time.Duration{}
	for i := 0; i < 20; i++ {
		started = time.Now()
		snapshot, err := s.CreateAnalysisContext(ctx, p, c.OrganizationID, Filters{Period: "week", TeamIDs: []string{"test:a"}})
		require.NoError(t, err)
		contextLatency = append(contextLatency, time.Since(started))
		state, err := s.analysisContext(ctx, p, c.OrganizationID, snapshot.ID, "analytics:read")
		require.NoError(t, err)
		started = time.Now()
		frame, err := s.loadAnalysis(ctx, state)
		require.NoError(t, err)
		overview, err := frame.overview()
		require.NoError(t, err)
		require.EqualValues(t, 10007, *overview.Stats.Requests)
		require.Equal(t, "1701500", *overview.Stats.Tokens.Total)
		_, err = json.Marshal(overview)
		require.NoError(t, err)
		overviewLatency = append(overviewLatency, time.Since(started))
	}
	percentile := func(values []time.Duration, index int) float64 {
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
		return float64(values[index]) / float64(time.Millisecond)
	}
	var groups int
	require.NoError(t, integrationDB.QueryRow(`WITH days AS (`+dayRevisionSQL+`) SELECT COUNT(*) FROM days JOIN bi_usage_daily d ON d.organization_id=$1 AND d.day=days.day AND d.data_revision=days.data_revision`, c.OrganizationID, int64(1<<62), "2026-09-14", "2026-09-21").Scan(&groups))
	t.Logf("calls=%d batches=20 import_seconds=%.3f daily_groups=%d context_p50_ms=%.3f context_p95_ms=%.3f overview_p50_ms=%.3f overview_p95_ms=%.3f", calls, importDuration.Seconds(), groups, percentile(contextLatency, 9), percentile(contextLatency, 18), percentile(overviewLatency, 9), percentile(overviewLatency, 18))
}
