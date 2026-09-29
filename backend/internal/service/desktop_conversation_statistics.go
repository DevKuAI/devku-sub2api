package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

type DesktopConversationCounts struct {
	RecordCount         int64    `json:"record_count"`
	PromptCount         int64    `json:"prompt_count"`
	DurationRecordCount int64    `json:"duration_record_count"`
	TotalDurationMS     int64    `json:"total_duration_ms"`
	AverageDurationMS   *float64 `json:"average_duration_ms"`
}

type DesktopConversationDay struct {
	Date string `json:"date"`
	DesktopConversationCounts
}

type DesktopConversationStatistics struct {
	Timezone                  string                    `json:"timezone"`
	AsOf                      time.Time                 `json:"as_of"`
	Today                     DesktopConversationCounts `json:"today"`
	Week                      DesktopConversationCounts `json:"week"`
	Month                     DesktopConversationCounts `json:"month"`
	Total                     DesktopConversationCounts `json:"total"`
	Last30Days                DesktopConversationCounts `json:"last_30_days"`
	CapturedLast30Days        int64                     `json:"captured_last_30_days"`
	ResponseMissingLast30Days int64                     `json:"response_missing_last_30_days"`
	WorkbuddyLast30Days       int64                     `json:"workbuddy_last_30_days"`
	ChatGPTCodexLast30Days    int64                     `json:"chatgpt_codex_last_30_days"`
	DistinctMembers           int64                     `json:"distinct_members"`
	DistinctSessions          int64                     `json:"distinct_sessions"`
	Captured                  int64                     `json:"captured"`
	ResponseMissing           int64                     `json:"response_missing"`
	Workbuddy                 int64                     `json:"workbuddy"`
	ChatGPTCodex              int64                     `json:"chatgpt_codex"`
	Daily                     []DesktopConversationDay  `json:"daily"`
	RangeStart                string                    `json:"range_start,omitempty"`
	RangeEnd                  string                    `json:"range_end,omitempty"`
}

type DesktopConversationPeriods struct {
	Today      time.Time
	Week       time.Time
	Month      time.Time
	Last30Days time.Time
	TrendStart time.Time
	TrendEnd   time.Time
	TrendDays  int
	AsOf       time.Time
}

func (s *DesktopService) ConversationStatistics(ctx context.Context, organizationID string, managerID int64, filters DesktopConversationFilters) (*DesktopConversationStatistics, error) {
	now := s.now()
	var selected *DesktopAnalyticsRange
	if filters.AnalyticsRange != nil {
		if filters.ReceivedFrom != nil || filters.ReceivedTo != nil {
			return nil, ErrDesktopValidation
		}
		resolved, err := filters.AnalyticsRange.Resolve(now)
		if err != nil {
			return nil, err
		}
		selected = &resolved
		filters.ReceivedFrom, filters.ReceivedTo = &resolved.Start, &resolved.End
	}
	if err := filters.Validate(); err != nil {
		return nil, err
	}
	organization, err := s.conversationOrganization(ctx, organizationID, managerID)
	if err != nil {
		return nil, err
	}
	periods := DesktopConversationPeriods{
		Today: timezone.StartOfDay(now), Week: timezone.StartOfWeek(now),
		Month: timezone.StartOfMonth(now), Last30Days: timezone.StartOfDay(now).AddDate(0, 0, -29), AsOf: now.UTC(),
	}
	periods.TrendStart, periods.TrendEnd, periods.TrendDays = periods.Last30Days, now, 30
	if selected != nil {
		periods.TrendStart, periods.TrendEnd, periods.TrendDays = selected.Start, selected.End, selected.Days
	}
	result, err := s.conversations.Statistics(ctx, organization.ID, filters, periods)
	if err != nil {
		return nil, err
	}
	result.Timezone = timezone.Location().String()
	result.AsOf = periods.AsOf
	if selected != nil {
		result.RangeStart, result.RangeEnd = selected.StartDate, selected.EndDate
	}
	return result, nil
}
