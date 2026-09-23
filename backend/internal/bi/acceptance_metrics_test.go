package bi

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func metricAcceptanceFrame(t *testing.T, members int) *analysisFrame {
	t.Helper()
	now := time.Date(2026, 9, 23, 8, 0, 0, 0, time.UTC)
	period, previous, err := resolvePeriod("week", now)
	require.NoError(t, err)
	f := &analysisFrame{state: analysisState{Scope: OrganizationScope{Organization: Organization{ID: "org", AllTeams: true, TeamIDs: []string{}, Capabilities: []string{"analytics:read"}}}, Context: AnalysisContext{ID: "ctx_fixture", OrganizationID: "org", Filters: Filters{Period: "week", TeamIDs: []string{}}, Range: period, ComparisonRange: previous}, Sources: []sourceState{{ID: "source", Kinds: append([]string{}, RecordKinds...), Applied: true, BackfillComplete: true, HistoryStartDate: ptr("2026-08-01"), CompleteThrough: &now}}}, records: map[string]map[string]*record{}, facts: []usageFact{}, factsLoaded: true, organizationStart: time.Date(2026, 8, 1, 0, 0, 0, 0, shanghai)}
	add := func(kind string, payload map[string]any) {
		raw, err := json.Marshal(payload)
		require.NoError(t, err)
		r, err := decodeRecord(ImportRecord{Kind: kind, Revision: json.Number("1"), Payload: raw}, 0)
		require.NoError(t, err)
		if f.records[kind] == nil {
			f.records[kind] = map[string]*record{}
		}
		f.records[kind][r.ID] = r
	}
	add("team", map[string]any{"id": "team-a", "name": "Team A", "status": "active"})
	add("team", map[string]any{"id": "team-b", "name": "Team B", "status": "active"})
	add("role", map[string]any{"id": "role", "name": "Engineer", "status": "active"})
	for i := 0; i < members; i++ {
		id := fmt.Sprintf("member-%02d", i)
		add("member", map[string]any{"id": id, "name": id, "status": "active", "enabled_at": "2026-08-01T00:00:00Z", "disabled_at": nil})
		add("membership", map[string]any{"id": "membership-" + id, "member_id": id, "team_id": "team-a", "role_id": "role", "valid_from": "2026-08-01T00:00:00Z", "valid_to": nil, "primary": true})
		add("eligibility", map[string]any{"id": "eligibility-" + id, "member_id": id, "application_id": nil, "scene_id": nil, "valid_from": "2026-08-01T00:00:00Z", "valid_to": nil})
	}
	return f
}

func addMetricUsage(t *testing.T, f *analysisFrame, member int, date, actor, team, model, tokens string) {
	t.Helper()
	at, err := time.Parse(time.RFC3339, date+"T04:00:00Z")
	require.NoError(t, err)
	u := usageFact{UsageRecord: UsageRecord{ID: fmt.Sprintf("usage-%d", len(f.facts)), OccurredAt: at, ActorType: actor, TeamID: ptr(team), RequestedModel: model, Outcome: "succeeded"}}
	if member >= 0 {
		u.MemberID = ptr(fmt.Sprintf("member-%02d", member))
	}
	if tokens != "" {
		u.Normalized = NormalizedTokens{Input: ptr(tokens), Output: ptr("0"), CacheRead: ptr("0"), CacheWrite: ptr("0")}
	}
	f.facts = append(f.facts, u)
}

func TestAcceptanceA01AutomaticGrowthDoesNotBecomeAdoptionGrowth(t *testing.T) {
	f := metricAcceptanceFrame(t, 25)
	for i := 0; i < 23; i++ {
		addMetricUsage(t, f, i, "2026-09-08", "human", "team-a", "model", "1")
	}
	for i := 0; i < 21; i++ {
		addMetricUsage(t, f, i, "2026-09-15", "human", "team-a", "model", "1")
	}
	addMetricUsage(t, f, -1, "2026-09-08", "automatic", "team-a", "model", "10")
	addMetricUsage(t, f, -1, "2026-09-15", "automatic", "team-a", "model", "1000")
	stats := f.statistics(analysisSelection{}).stats
	previous := f.calculate(f.state.Context.ComparisonRange, analysisSelection{}).stats
	require.EqualValues(t, 23, *previous.ActiveMembers.Numerator)
	require.EqualValues(t, 21, *stats.ActiveMembers.Numerator)
	require.InDelta(t, -0.08, *stats.AdoptionComparison.RateDifference, 1e-12)
	require.Equal(t, "1000", *stats.Tokens.Automatic)
	tokens := f.tokenAnalysis(analysisSelection{})
	require.Greater(t, *tokens.RelativeChange, 0.0)
	overview, err := f.overview()
	require.NoError(t, err)
	for _, finding := range overview.Findings {
		require.Equal(t, "none", finding.InterpretationStatus)
		require.NotContains(t, finding.Fact, "采用提升")
	}
}

func TestAcceptanceA02GrowingFirstUseCohortsCanHaveLowerRetention(t *testing.T) {
	f := metricAcceptanceFrame(t, 30)
	for i := 0; i < 10; i++ {
		addMetricUsage(t, f, i, "2026-09-01", "human", "team-a", "model", "1")
		if i < 8 {
			addMetricUsage(t, f, i, "2026-09-08", "human", "team-a", "model", "1")
		}
	}
	for i := 10; i < 30; i++ {
		addMetricUsage(t, f, i, "2026-09-08", "human", "team-a", "model", "1")
		if i < 16 {
			addMetricUsage(t, f, i, "2026-09-15", "human", "team-a", "model", "1")
		}
	}
	cohorts := f.retention(analysisSelection{})
	require.Len(t, cohorts, 2)
	require.EqualValues(t, 10, cohorts[0].Size)
	require.EqualValues(t, 20, cohorts[1].Size)
	require.InDelta(t, 0.8, *cohorts[0].Cells[0].Rate.Value, 1e-12)
	require.InDelta(t, 0.3, *cohorts[1].Cells[0].Rate.Value, 1e-12)
	require.Nil(t, cohorts[1].Cells[1].Retained)
	require.Equal(t, "period_incomplete", *cohorts[1].Cells[1].Rate.Reason)
}

func TestAcceptanceA03LowVolumePreservesCoverageAndRepeatUse(t *testing.T) {
	f := metricAcceptanceFrame(t, 2)
	for i := 0; i < 2; i++ {
		addMetricUsage(t, f, i, "2026-09-15", "human", "team-a", "model", "1")
		addMetricUsage(t, f, i, "2026-09-16", "human", "team-a", "model", "1")
	}
	stats := f.statistics(analysisSelection{}).stats
	require.Equal(t, "4", *stats.Tokens.Total)
	require.EqualValues(t, 2, *stats.EligibleMembers.Numerator)
	require.Equal(t, 1.0, *stats.AdoptionRate.Value)
	require.EqualValues(t, 2, *stats.RepeatMembers.Numerator)
	overview, err := f.overview()
	require.NoError(t, err)
	raw, err := json.Marshal(overview)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "绩效")
}

func TestAcceptanceA13IncompleteFirstUseHistoryStaysUnknown(t *testing.T) {
	f := metricAcceptanceFrame(t, 1)
	f.state.Sources[0].HistoryStartDate = ptr("2026-09-14")
	addMetricUsage(t, f, 0, "2026-09-15", "human", "team-a", "model", "1")
	stats := f.statistics(analysisSelection{}).stats
	require.EqualValues(t, 1, *stats.ActiveMembers.Numerator)
	require.Nil(t, stats.ActivatedMembers.Value)
	require.Equal(t, "history_incomplete", *stats.ActivatedMembers.Reason)
	require.Nil(t, stats.StableMembers.Value)
	require.Empty(t, f.retention(analysisSelection{}))
}

func TestAcceptanceA15TokenBreakdownsReconcileExactly(t *testing.T) {
	f := metricAcceptanceFrame(t, 2)
	addMetricUsage(t, f, 0, "2026-09-15", "human", "team-a", "model-a", "9007199254740993")
	addMetricUsage(t, f, 1, "2026-09-16", "human", "team-b", "model-b", "7")
	addMetricUsage(t, f, -1, "2026-09-17", "automatic", "team-a", "model-a", "11")
	addMetricUsage(t, f, -1, "2026-09-18", "unknown", "team-b", "model-b", "13")
	addMetricUsage(t, f, -1, "2026-09-19", "automatic", "team-a", "media", "")
	stats := f.statistics(analysisSelection{}).stats
	sum := func(values ...*string) string {
		total := new(big.Int)
		for _, value := range values {
			if value != nil {
				n, ok := new(big.Int).SetString(*value, 10)
				require.True(t, ok)
				total.Add(total, n)
			}
		}
		return total.String()
	}
	require.Equal(t, *stats.Tokens.Total, sum(stats.Tokens.Human, stats.Tokens.Automatic, stats.Tokens.Unknown))
	models := f.tokenAnalysis(analysisSelection{})
	modelValues := []*string{}
	for _, row := range models.Models {
		modelValues = append(modelValues, row.Tokens)
	}
	require.Equal(t, *stats.Tokens.Total, sum(modelValues...))
	trend := f.trend(analysisSelection{}, "day")
	daily := []*string{}
	for _, point := range trend.Points {
		daily = append(daily, point.Tokens)
	}
	require.Equal(t, *stats.Tokens.Total, sum(daily...))
	a, b := f.statistics(analysisSelection{TeamID: "team-a"}), f.statistics(analysisSelection{TeamID: "team-b"})
	require.Equal(t, *stats.Tokens.Total, sum(a.stats.Tokens.Total, b.stats.Tokens.Total))
	require.EqualValues(t, 1, stats.Tokens.UnmeasuredRequests)
	require.Nil(t, models.RelativeChange)
}

func TestAcceptanceA16ComparisonUsesEachPeriodsDenominator(t *testing.T) {
	f := metricAcceptanceFrame(t, 4)
	for _, id := range []string{"member-02", "member-03"} {
		r := f.records["member"][id]
		r.Payload["disabled_at"] = json.RawMessage(`"2026-09-14T00:00:00Z"`)
	}
	addMetricUsage(t, f, 0, "2026-09-08", "human", "team-a", "model", "1")
	addMetricUsage(t, f, 1, "2026-09-08", "human", "team-a", "model", "1")
	addMetricUsage(t, f, 0, "2026-09-15", "human", "team-a", "model", "1")
	stats := f.statistics(analysisSelection{}).stats
	require.EqualValues(t, 2, *stats.AdoptionRate.Denominator)
	require.Equal(t, 0.5, *stats.AdoptionRate.Value)
	require.Equal(t, 0.5, *stats.AdoptionComparison.Previous)
	require.Equal(t, 0.0, *stats.AdoptionComparison.RateDifference)
	zero := metricAcceptanceFrame(t, 1)
	addMetricUsage(t, zero, 0, "2026-09-15", "human", "team-a", "model", strings.Repeat("9", 20))
	comparison := zero.tokenAnalysis(analysisSelection{})
	require.Nil(t, comparison.RelativeChange)
	require.Equal(t, "no_baseline", *comparison.ComparisonReason)
}
