//go:build integration && bi_capacity

package bi

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
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
	contextLatency, overviewLatency, fullLatency := []time.Duration{}, []time.Duration{}, []time.Duration{}
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
		started = time.Now()
		full, err := s.loadMetadata(ctx, state)
		require.NoError(t, err)
		full.facts, err = s.readUsageFacts(ctx, state, nil, nil)
		require.NoError(t, err)
		full.factsLoaded = true
		before, err := full.overview()
		require.NoError(t, err)
		_, err = json.Marshal(before)
		require.NoError(t, err)
		fullLatency = append(fullLatency, time.Since(started))
		require.Equal(t, before, overview)
	}
	percentile := func(values []time.Duration, index int) float64 {
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
		return float64(values[index]) / float64(time.Millisecond)
	}
	var groups int
	require.NoError(t, integrationDB.QueryRow(`WITH days AS (`+dayRevisionSQL+`) SELECT COUNT(*) FROM days JOIN bi_usage_daily d ON d.organization_id=$1 AND d.day=days.day AND d.data_revision=days.data_revision`, c.OrganizationID, int64(1<<62), "2026-09-14", "2026-09-21").Scan(&groups))
	t.Logf("calls=%d batches=20 import_seconds=%.3f daily_groups=%d context_p50_ms=%.3f context_p95_ms=%.3f overview_p50_ms=%.3f overview_p95_ms=%.3f full_p50_ms=%.3f full_p95_ms=%.3f", calls, importDuration.Seconds(), groups, percentile(contextLatency, 9), percentile(contextLatency, 18), percentile(overviewLatency, 9), percentile(overviewLatency, 18), percentile(fullLatency, 9), percentile(fullLatency, 18))
}

func TestBIHistoricalReadBaseline(t *testing.T) {
	s, c, p, _ := analysisFixture(t)
	ctx := context.Background()
	base := acceptancePayload(t, s, c, "usage", "test:one:1")
	checkpoint := "analytics-seed"
	const calls = 10000
	for offset := 0; offset < calls; offset += 500 {
		records := make([]any, 0, 500)
		for i := offset; i < offset+500; i++ {
			usage := maps.Clone(base)
			usage["id"], usage["occurred_at"] = fmt.Sprintf("test:history-volume:%05d", i), "2026-08-25T04:00:00Z"
			if i%2 == 1 {
				usage["actor_type"], usage["member_id"] = "automatic", nil
			}
			records = append(records, analyticRecord("usage", usage))
		}
		next := fmt.Sprintf("history-volume-%d", offset)
		job := applyImport(t, s, c, completeBatch(t, checkpoint, next, records))
		require.Equal(t, "applied", job.Status, job.Errors)
		checkpoint = next
	}
	snapshot, err := s.CreateAnalysisContext(ctx, p, c.OrganizationID, Filters{Period: "week", TeamIDs: []string{"test:a"}})
	require.NoError(t, err)
	state, err := s.analysisContext(ctx, p, c.OrganizationID, snapshot.ID, "analytics:read")
	require.NoError(t, err)
	measure := func(compact bool) (time.Duration, int, Overview) {
		started := time.Now()
		var frame *analysisFrame
		var err error
		if compact {
			frame, err = s.loadAnalysis(ctx, state)
		} else {
			frame, err = s.loadMetadata(ctx, state)
			require.NoError(t, err)
			frame.facts, err = s.readUsageFacts(ctx, state, nil, nil)
			frame.factsLoaded = true
		}
		require.NoError(t, err)
		result, err := frame.overview()
		require.NoError(t, err)
		return time.Since(started), len(frame.facts), result
	}
	_, fullRows, expected := measure(false)
	_, compactRows, actual := measure(true)
	require.Equal(t, expected, actual)
	require.Less(t, compactRows, fullRows/100)
	fullLatency, compactLatency := []time.Duration{}, []time.Duration{}
	// Alternate order after warming both paths to reduce cache-order bias.
	for i := 0; i < 20; i++ {
		for _, compact := range []bool{i%2 == 0, i%2 != 0} {
			elapsed, _, result := measure(compact)
			require.Equal(t, expected, result)
			if compact {
				compactLatency = append(compactLatency, elapsed)
			} else {
				fullLatency = append(fullLatency, elapsed)
			}
		}
	}
	percentile := func(values []time.Duration, index int) float64 {
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
		return float64(values[index]) / float64(time.Millisecond)
	}
	t.Logf("historical_calls=%d full_rows=%d compact_rows=%d full_p50_ms=%.3f full_p95_ms=%.3f compact_p50_ms=%.3f compact_p95_ms=%.3f", calls, fullRows, compactRows,
		percentile(fullLatency, 9), percentile(fullLatency, 18), percentile(compactLatency, 9), percentile(compactLatency, 18))
}
