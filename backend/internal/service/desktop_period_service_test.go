package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

func TestDesktopPeriodCalendarClosedShanghaiPeriods(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	for _, tt := range []struct {
		kind, start, now, wantStart, wantEnd string
		invalid                              bool
	}{
		{"week", "", "2026-10-13T01:00:00", "2026-10-05", "2026-10-12", false},
		{"month", "", "2026-01-01T00:00:00", "2025-12-01", "2026-01-01", false},
		{"month", "2024-02-01", "2024-03-01T00:00:00", "2024-02-01", "2024-03-01", false},
		{"week", "2026-10-06", "2026-10-13T00:00:00", "", "", true},
		{"week", "2026-10-12", "2026-10-13T00:00:00", "", "", true},
		{"month", "2026-10-01", "2026-10-13T00:00:00", "", "", true},
	} {
		now, err := time.ParseInLocation("2006-01-02T15:04:05", tt.now, loc)
		require.NoError(t, err)
		first, end, err := resolveDesktopPeriod(tt.kind, tt.start, now)
		if tt.invalid {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		require.Equal(t, tt.wantStart, first.Format("2006-01-02"))
		require.Equal(t, tt.wantEnd, end.Format("2006-01-02"))
	}
}

type periodTestRepo struct {
	*reportTestRepo
	DesktopPeriodRepository
	sources   []DesktopPeriodSource
	coverage  DesktopPeriodCoverage
	saves     int
	leaseLost bool
}

func (r *periodTestRepo) PeriodSources(context.Context, int64, time.Time, time.Time) ([]DesktopPeriodSource, DesktopPeriodCoverage, error) {
	return r.sources, r.coverage, nil
}
func (r *periodTestRepo) SavePeriod(context.Context, *DesktopPeriodTask, time.Time, bool) error {
	r.saves++
	if r.leaseLost {
		return ErrDesktopReportLeaseLost
	}
	return nil
}
func (r *periodTestRepo) BeginPeriodExecution(ctx context.Context, _ *DesktopPeriodTask, e *DesktopReportExecution) error {
	return r.BeginExecution(ctx, nil, e)
}
func (r *periodTestRepo) FinishPeriodExecution(ctx context.Context, e *DesktopReportExecution) error {
	return r.FinishExecution(ctx, e)
}
func (r *periodTestRepo) RecordPeriodExecutionUsage(context.Context, int64, *UsageLog) error {
	return nil
}
func periodTask(s *DesktopReportService) *DesktopPeriodTask {
	start, end, _ := resolveDesktopPeriod("week", "", s.now())
	return &DesktopPeriodTask{OrganizationID: 1, Start: start, End: end, DesktopPeriodMetadata: DesktopPeriodMetadata{Kind: "week", Revision: 1, Range: DesktopNativeRange{Start: start.Format("2006-01-02"), End: end.AddDate(0, 0, -1).Format("2006-01-02")}}}
}

type periodTestAI struct {
	*reportTestAI
	inputs []string
}

func (a *periodTestAI) Generate(ctx context.Context, o *DesktopOrganization, k *APIKey, model, prompt, input, id string) (string, error) {
	a.inputs = append(a.inputs, input)
	return a.reportTestAI.Generate(ctx, o, k, model, prompt, input, id)
}
func TestDesktopPeriodUsesOnlyDailySummaryAndFreezesSources(t *testing.T) {
	s, daily, baseAI, _ := reportFixture()
	ai := &periodTestAI{reportTestAI: baseAI}
	s.ai = ai
	r := &periodTestRepo{reportTestRepo: daily, sources: []DesktopPeriodSource{{Date: "2026-09-28", Content: "DAILY_SUMMARY_ONLY", Revision: 2, Status: "completed"}}, coverage: DesktopPeriodCoverage{Available: []string{"2026-09-28"}, Missing: []string{"2026-09-29"}}}
	s.repo = r
	daily.members = []DesktopReportMember{{Content: "FORBIDDEN_MEMBER_BODY"}}
	daily.snapshots = []DesktopReportSnapshot{reportSnapshot("FORBIDDEN_CONVERSATION")}
	task := periodTask(s)
	_, err := s.processPeriod(context.Background(), r, task)
	require.NoError(t, err)
	require.Equal(t, "partial", task.Status)
	require.NotNil(t, task.Content)
	require.Len(t, ai.calls, 1)
	require.Contains(t, ai.inputs[0], "DAILY_SUMMARY_ONLY")
	require.NotContains(t, strings.Join(ai.inputs, ""), "FORBIDDEN")
	require.True(t, task.Configuration.Captured)
	require.Len(t, r.executions, 1)
	r.sources[0].Content = "NEW_SOURCE"
	_, err = s.processPeriod(context.Background(), r, task)
	require.NoError(t, err)
	require.Len(t, ai.calls, 1)
	require.Equal(t, "DAILY_SUMMARY_ONLY", task.Sources[0].Content)
}
func TestDesktopPeriodNoSourcesNeverCallsModelAndLeaseLossStops(t *testing.T) {
	s, daily, ai, _ := reportFixture()
	r := &periodTestRepo{reportTestRepo: daily, coverage: DesktopPeriodCoverage{Missing: []string{"2026-09-28"}}}
	s.repo = r
	task := periodTask(s)
	next, err := s.processPeriod(context.Background(), r, task)
	require.NoError(t, err)
	require.Equal(t, "waiting", task.Status)
	require.Equal(t, "missing_sources", task.Reason)
	require.True(t, next.After(s.now()))
	require.Empty(t, ai.calls)
	require.False(t, task.Configuration.Captured)
	r.sources = []DesktopPeriodSource{{Content: "daily"}}
	r.leaseLost = true
	_, err = s.processPeriod(context.Background(), r, task)
	require.ErrorIs(t, err, ErrDesktopReportLeaseLost)
	require.Empty(t, ai.calls)
}
func TestDesktopPeriodBudgetAndConfigurationBlockModel(t *testing.T) {
	s, daily, ai, o := reportFixture()
	r := &periodTestRepo{reportTestRepo: daily, sources: []DesktopPeriodSource{{Content: "daily"}}}
	s.repo = r
	task := periodTask(s)
	ai.blocked = NewDesktopReportUnavailable("analysis_budget_exhausted")
	_, err := s.processPeriod(context.Background(), r, task)
	require.Error(t, err)
	require.Empty(t, ai.calls)
	ai.blocked = nil
	task.Configuration = DesktopPeriodConfiguration{Captured: true, Model: "old", UserID: o.GatewayUserID, GroupID: o.GroupID}
	_, err = s.processPeriod(context.Background(), r, task)
	require.ErrorContains(t, err, "configuration_changed")
	require.Empty(t, ai.calls)
}
