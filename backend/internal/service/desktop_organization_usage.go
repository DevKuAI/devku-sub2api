package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

type DesktopOrganizationUsagePeriod struct {
	TotalTokens int64   `json:"total_tokens"`
	ActualCost  float64 `json:"actual_cost"`
}

type DesktopUsageWindows struct {
	Today, Week, Month, AsOf time.Time
	Selected                 DesktopAnalyticsRange
}

type DesktopUsageDay struct {
	Date string `json:"date"`
	DesktopOrganizationUsagePeriod
}

type DesktopUsageBreakdown struct {
	InputTokens         int64 `json:"input_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	CacheCreationTokens int64 `json:"cache_creation_tokens"`
	CacheReadTokens     int64 `json:"cache_read_tokens"`
}

type DesktopUsageModel struct {
	Model     string `json:"model"`
	Requests  int64  `json:"requests"`
	CostRank  int64  `json:"cost_rank"`
	TokenRank int64  `json:"token_rank"`
	DesktopOrganizationUsagePeriod
}

type DesktopUsageMember struct {
	MemberID  string `json:"member_id"`
	Name      string `json:"name"`
	Deleted   bool   `json:"deleted"`
	Requests  int64  `json:"requests"`
	CostRank  int64  `json:"cost_rank"`
	TokenRank int64  `json:"token_rank"`
	DesktopOrganizationUsagePeriod
}

type DesktopUsageMemberModel struct {
	MemberID    string `json:"member_id"`
	Name        string `json:"name"`
	Deleted     bool   `json:"deleted"`
	Model       string `json:"model"`
	Requests    int64  `json:"requests"`
	TotalTokens int64  `json:"total_tokens"`
	DesktopUsageBreakdown
}

type DesktopOrganizationUsageStatistics struct {
	Timezone        string                         `json:"timezone"`
	AsOf            time.Time                      `json:"as_of"`
	Today           DesktopOrganizationUsagePeriod `json:"today"`
	Week            DesktopOrganizationUsagePeriod `json:"week"`
	Month           DesktopOrganizationUsagePeriod `json:"month"`
	Total           DesktopOrganizationUsagePeriod `json:"total"`
	Last30Days      DesktopOrganizationUsagePeriod `json:"last_30_days"`
	Selected        DesktopOrganizationUsagePeriod `json:"selected"`
	Previous        DesktopOrganizationUsagePeriod `json:"previous"`
	RangeStart      string                         `json:"range_start"`
	RangeEnd        string                         `json:"range_end"`
	PreviousStart   string                         `json:"previous_start"`
	PreviousEnd     string                         `json:"previous_end"`
	Daily           []DesktopUsageDay              `json:"daily"`
	Breakdown       DesktopUsageBreakdown          `json:"breakdown"`
	Models          []DesktopUsageModel            `json:"models"`
	Members         []DesktopUsageMember           `json:"members"`
	MemberModels    []DesktopUsageMemberModel      `json:"member_models"`
	ObservedMembers int64                          `json:"observed_members"`
}

func (s *DesktopService) OrganizationUsageStatistics(ctx context.Context, organizationID string, managerID int64, input *DesktopAnalyticsRangeInput) (*DesktopOrganizationUsageStatistics, error) {
	selection := DesktopAnalyticsRangeInput{}
	if input != nil {
		selection = *input
	}
	now := s.now()
	selected, err := selection.Resolve(now)
	if err != nil {
		return nil, err
	}
	var organization *DesktopOrganization
	if managerID > 0 {
		organization, err = s.GetManagedOrganization(ctx, managerID)
	} else {
		organization, err = s.GetOrganization(ctx, organizationID)
	}
	if err != nil {
		return nil, err
	}
	result, err := s.usage.GetDesktopOrganizationUsage(ctx, organization.ID, DesktopUsageWindows{
		Today: timezone.StartOfDay(now).UTC(), Week: timezone.StartOfWeek(now).UTC(), Month: timezone.StartOfMonth(now).UTC(), AsOf: now.UTC(), Selected: selected,
	})
	if err != nil {
		return nil, ErrDesktopUsageUnavailable.WithCause(err)
	}
	result.Timezone = timezone.Location().String()
	result.AsOf = now.UTC()
	result.RangeStart, result.RangeEnd = selected.StartDate, selected.EndDate
	result.PreviousStart = selected.PreviousStart.In(timezone.Location()).Format("2006-01-02")
	previousLast := selected.Start.AddDate(0, 0, -1)
	result.PreviousEnd = previousLast.Format("2006-01-02")
	return result, nil
}
