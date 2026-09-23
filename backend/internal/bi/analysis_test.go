package bi

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAnalysisPeriodsUseShanghaiCompletedDays(t *testing.T) {
	for _, tc := range []struct {
		now, period, start, end, previousStart, previousEnd string
		days                                                int
	}{
		{"2026-09-23T00:01:00Z", "current", "2026-09-21", "2026-09-23", "2026-09-14", "2026-09-16", 2},
		{"2026-09-20T16:01:00Z", "current", "2026-09-21", "2026-09-21", "2026-09-14", "2026-09-14", 0},
		{"2026-09-23T00:01:00Z", "week", "2026-09-14", "2026-09-21", "2026-09-07", "2026-09-14", 7},
		{"2024-03-10T00:00:00Z", "month", "2024-02-01", "2024-03-01", "2024-01-01", "2024-02-01", 29},
	} {
		now, err := time.Parse(time.RFC3339, tc.now)
		require.NoError(t, err)
		period, previous, err := resolvePeriod(tc.period, now)
		require.NoError(t, err)
		require.Equal(t, tc.start, period.StartDate)
		require.Equal(t, tc.end, period.EndDateExclusive)
		require.Equal(t, tc.days, period.CompleteDays)
		require.Equal(t, tc.previousStart, previous.StartDate)
		require.Equal(t, tc.previousEnd, previous.EndDateExclusive)
	}
}

func TestAnalysisTokenTotalsPreserveUnknownAndBigIntegers(t *testing.T) {
	var acc tokenAccumulator
	acc.add(usageFact{UsageRecord: UsageRecord{ActorType: "human"}, Normalized: NormalizedTokens{Input: ptr("9007199254740993"), Output: ptr("2"), CacheRead: ptr("1"), CacheWrite: ptr("0")}}, true)
	acc.add(usageFact{UsageRecord: UsageRecord{ActorType: "automatic"}}, false)
	result := acc.totals(true, true)
	require.NoError(t, schemaOutput("TokenTotals", result))
	require.Equal(t, "9007199254740995", *result.Total)
	require.Equal(t, *result.Total, *result.Human)
	require.EqualValues(t, 1, result.MeasuredRequests)
	require.EqualValues(t, 1, result.UnmeasuredRequests)
	var empty tokenAccumulator
	require.Nil(t, empty.totals(false, false).Total)
	require.Equal(t, "0", *empty.totals(true, true).Total)
	empty.add(usageFact{}, false)
	require.Nil(t, empty.totals(true, true).Total)
}

func schemaOutput(name string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return validateSchema(name, raw)
}

func TestRetentionCannotObserveCallsOutsideGrantedTeams(t *testing.T) {
	at := time.Date(2026, 9, 8, 4, 0, 0, 0, time.UTC)
	f := &analysisFrame{state: analysisState{Scope: OrganizationScope{Organization: Organization{AllTeams: false, TeamIDs: []string{"team-a"}}}}, records: map[string]map[string]*record{}, facts: []usageFact{
		{UsageRecord: UsageRecord{MemberID: ptr("member"), TeamID: ptr("team-b"), ActorType: "human", Outcome: "succeeded", OccurredAt: at}},
	}}
	period := dateRange(weekStart(at), weekStart(at).AddDate(0, 0, 7), "")
	require.False(t, f.observeMembers([]string{"member"}, period))
}

func TestRetentionUsesStableEventIDForSimultaneousFirstCalls(t *testing.T) {
	f := metricAcceptanceFrame(t, 1)
	addMetricUsage(t, f, 0, "2026-08-25", "human", "team-b", "model", "1")
	f.facts[0].ID = "event-z"
	addMetricUsage(t, f, 0, "2026-08-25", "human", "team-a", "model", "1")
	f.facts[1].ID = "event-a"
	require.Len(t, f.retention(analysisSelection{TeamID: "team-a"}), 1)
	require.Empty(t, f.retention(analysisSelection{TeamID: "team-b"}))
	f.facts[0], f.facts[1] = f.facts[1], f.facts[0]
	require.Len(t, f.retention(analysisSelection{TeamID: "team-a"}), 1)
}
