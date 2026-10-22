package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

func DesktopUsageTokenStatus(measured, unmeasured int64) string {
	if unmeasured == 0 {
		return "ready"
	}
	if measured == 0 {
		return "unavailable"
	}
	return "partial"
}

type DesktopUsageMeasurement struct {
	TokenStatus        string `json:"token_status,omitempty"`
	MeasuredRequests   int64  `json:"measured_requests,omitempty"`
	UnmeasuredRequests int64  `json:"unmeasured_requests,omitempty"`
}
type DesktopOrganizationUsagePeriod struct {
	TotalTokens int64   `json:"total_tokens"`
	ActualCost  float64 `json:"actual_cost"`
	DesktopUsageMeasurement
}

type DesktopUsageWindows struct {
	SummaryOnly, MemberModelsOnly bool
	Query                         DesktopUsageQuery
	Today, Week, Month, AsOf      time.Time
	Selected                      DesktopAnalyticsRange
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
	DesktopUsageMeasurement
	MemberID    string `json:"member_id"`
	Name        string `json:"name"`
	Deleted     bool   `json:"deleted"`
	Model       string `json:"model"`
	Requests    int64  `json:"requests"`
	TotalTokens int64  `json:"total_tokens"`
	DesktopUsageBreakdown
}

type DesktopMemberModelPage struct {
	Page         int      `json:"page"`
	PageSize     int      `json:"page_size"`
	TotalMembers int64    `json:"total_members"`
	TotalRows    int64    `json:"total_rows"`
	ModelCount   int64    `json:"model_count"`
	TotalTokens  int64    `json:"total_tokens"`
	Models       []string `json:"models"`
	DesktopUsageMeasurement
}
type DesktopUsageQuery struct {
	Section             string
	Page, PageSize      int
	Search, Model, Sort string
}
type DesktopOrganizationUsageStatistics struct {
	MemberModelsPage *DesktopMemberModelPage        `json:"member_models_page,omitempty"`
	Analysis         DesktopOrganizationUsagePeriod `json:"analysis"`
	Timezone         string                         `json:"timezone"`
	AsOf             time.Time                      `json:"as_of"`
	Today            DesktopOrganizationUsagePeriod `json:"today"`
	Week             DesktopOrganizationUsagePeriod `json:"week"`
	Month            DesktopOrganizationUsagePeriod `json:"month"`
	Total            DesktopOrganizationUsagePeriod `json:"total"`
	Last30Days       DesktopOrganizationUsagePeriod `json:"last_30_days"`
	Selected         DesktopOrganizationUsagePeriod `json:"selected"`
	Previous         DesktopOrganizationUsagePeriod `json:"previous"`
	RangeStart       string                         `json:"range_start"`
	RangeEnd         string                         `json:"range_end"`
	PreviousStart    string                         `json:"previous_start"`
	PreviousEnd      string                         `json:"previous_end"`
	Daily            []DesktopUsageDay              `json:"daily"`
	Breakdown        DesktopUsageBreakdown          `json:"breakdown"`
	Models           []DesktopUsageModel            `json:"models"`
	Members          []DesktopUsageMember           `json:"members"`
	MemberModels     []DesktopUsageMemberModel      `json:"member_models"`
	ObservedMembers  int64                          `json:"observed_members"`
}

func (s *DesktopService) OrganizationUsageStatistics(ctx context.Context, organizationID string, managerID int64, input *DesktopAnalyticsRangeInput, queries ...DesktopUsageQuery) (*DesktopOrganizationUsageStatistics, error) {
	query := DesktopUsageQuery{Section: "insights", Page: 1, PageSize: 20, Sort: "tokens"}
	if len(queries) > 0 {
		query = queries[0]
	}
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		return nil, ErrDesktopValidation
	}
	if query.Section != "insights" && query.Section != "summary" && query.Section != "member_models" {
		return nil, ErrDesktopValidation
	}
	if query.Sort != "" && query.Sort != "tokens" && query.Sort != "name" {
		return nil, ErrDesktopValidation
	}
	selection := DesktopAnalyticsRangeInput{}
	if input != nil {
		selection = *input
	}
	now := s.now()
	selected, err := selection.Resolve(now)
	if err != nil {
		return nil, err
	}
	if !selection.AsOf.IsZero() {
		now = selection.AsOf
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
		SummaryOnly: query.Section == "summary", MemberModelsOnly: query.Section == "member_models", Query: query,
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
