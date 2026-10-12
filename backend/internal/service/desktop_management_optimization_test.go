package service

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
	"math"
	"strings"
	"testing"
	"time"
)

func TestDesktopBudgetRejectsInvalidAmounts(t *testing.T) {
	for _, amount := range []float64{-1, math.NaN(), math.Inf(1)} {
		require.Error(t, (DesktopMemberBudget{Quota: amount}).Validate())
		require.Error(t, (DesktopManagementPolicy{AnalysisLimit1d: amount}).Validate())
	}
	require.Error(t, (DesktopManagementPolicy{ConversationRetentionDays: -1}).Validate())
	require.Error(t, (DesktopManagementPolicy{ReportRetentionDays: 3651}).Validate())
	require.NoError(t, (DesktopManagementPolicy{}).Validate())
	key := &APIKey{QuotaUsed: 4, Usage1d: 2}
	(DesktopMemberBudget{Quota: 10, Limit1d: 3, Limit7d: 20}).Apply(key)
	require.Equal(t, 4.0, key.QuotaUsed)
	require.Equal(t, 2.0, key.Usage1d)
}

func TestDesktopReportChunksPreserveSourceAndText(t *testing.T) {
	text := strings.Repeat("中文正文与 code \"quoted\"\n", 900)
	source := DesktopReportSource{RecordID: "record-one", Client: "workbuddy", SessionID: "original-session", CaptureStatus: "response_missing", Prompts: []DesktopTextSegment{{Text: text, Truncated: true}}}
	inputs := reportSourceInputs([]DesktopReportSource{source}, "gpt-4o")
	require.Greater(t, len(inputs), 1)
	codec, err := openAIInputTokensCodecForModel("gpt-4o")
	require.NoError(t, err)
	var rebuilt strings.Builder
	for _, input := range inputs {
		require.True(t, reportInputFits(input, codec))
		var fragments []struct {
			RecordID  string `json:"record_id"`
			Kind      string `json:"kind"`
			Text      string `json:"text"`
			Truncated bool   `json:"truncated"`
		}
		require.NoError(t, json.Unmarshal([]byte(input), &fragments))
		for _, fragment := range fragments {
			require.Equal(t, "record-one", fragment.RecordID)
			require.Equal(t, "prompt", fragment.Kind)
			require.True(t, fragment.Truncated)
			_, _ = rebuilt.WriteString(fragment.Text)
		}
	}
	require.Equal(t, text, rebuilt.String())
}

func TestDesktopReportSmallRecordsStayWhole(t *testing.T) {
	sources := []DesktopReportSource{{RecordID: "one", Prompts: []DesktopTextSegment{{Text: "first"}}}, {RecordID: "two", Response: &DesktopTextSegment{Text: "second"}}}
	inputs := reportSourceInputs(sources, "gpt-4o")
	require.Len(t, inputs, 1)
	var decoded []DesktopReportSource
	require.NoError(t, json.Unmarshal([]byte(inputs[0]), &decoded))
	require.Equal(t, sources, decoded)
}

func TestDesktopReportUsageCaptureReceivesBilledValues(t *testing.T) {
	var captured *UsageLog
	ctx := WithDesktopReportUsageCapture(context.Background(), func(log *UsageLog) { captured = log })
	usage := &UsageLog{InputTokens: 10, OutputTokens: 20, ActualCost: 0.15}
	writeUsageLogBestEffort(ctx, nil, usage, "test")
	require.Same(t, usage, captured)
}

func TestDesktopAnalysisNeverCapturesUsagePrompt(t *testing.T) {
	settings := &contentModerationRuntimeSettingRepo{values: map[string]string{SettingKeyRiskControlEnabled: "true"}}
	s := &ContentModerationService{settingRepo: settings}
	body := []byte(`{"messages":[{"role":"user","content":"PRIVATE-SOURCE"}]}`)
	require.NotNil(t, s.CaptureUsageRequestBody(context.Background(), ContentModerationProtocolOpenAIChat, body, "application/json"))
	require.Nil(t, s.CaptureUsageRequestBody(WithDesktopReportRequest(context.Background()), ContentModerationProtocolOpenAIChat, body, "application/json"))
}

type desktopAnalysisBudgetTestRepo struct {
	*reportTestRepo
	used float64
}

func (r *desktopAnalysisBudgetTestRepo) AnalysisUsage(context.Context, int64) (float64, error) {
	return r.used, nil
}

func TestDesktopAnalysisBudgetBlocksBeforeSourceReadAndRetry(t *testing.T) {
	s, r, a, o := reportFixture()
	s.repo = &desktopAnalysisBudgetTestRepo{reportTestRepo: r, used: 1}
	o.ManagementPolicy.AnalysisLimit1d = 1
	task := &DesktopReportTask{OrganizationID: o.ID, Date: "2026-10-09"}
	_, err := s.process(context.Background(), task)
	require.Equal(t, "analysis_budget_exhausted", reportUnavailableReason(err))
	require.Empty(t, a.calls)
	require.Empty(t, task.Payload.Snapshots)
	require.Zero(t, r.saved)
}

func TestDesktopMemberUsageCutoffRemainsFixedAcrossPages(t *testing.T) {
	repo := &desktopRepositoryStub{members: []DesktopMember{{ID: 1}}}
	usage := &desktopUsageRepositoryStub{}
	now := time.Date(2026, 10, 12, 14, 0, 0, 0, time.UTC)
	s := &DesktopService{repo: repo, usage: usage, now: func() time.Time { return now }}
	cutoff := now.Add(-time.Minute)
	members, _, err := s.ListMembersWithUsage(context.Background(), "org", pagination.PaginationParams{Page: 2, PageSize: 20}, DesktopMemberListFilters{}, cutoff)
	require.NoError(t, err)
	require.True(t, cutoff.Equal(*members[0].UsageAsOf))
	require.True(t, cutoff.Equal(usage.memberCalls[0].endTime))
	_, _, err = s.ListMembersWithUsage(context.Background(), "org", pagination.PaginationParams{Page: 1, PageSize: 20}, DesktopMemberListFilters{}, now.Add(time.Second))
	require.ErrorIs(t, err, ErrDesktopValidation)
}
