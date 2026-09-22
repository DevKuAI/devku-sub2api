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

type DesktopOrganizationUsageStatistics struct {
	Timezone string                         `json:"timezone"`
	AsOf     time.Time                      `json:"as_of"`
	Today    DesktopOrganizationUsagePeriod `json:"today"`
	Week     DesktopOrganizationUsagePeriod `json:"week"`
	Month    DesktopOrganizationUsagePeriod `json:"month"`
	Total    DesktopOrganizationUsagePeriod `json:"total"`
}

func (s *DesktopService) OrganizationUsageStatistics(ctx context.Context, organizationID string, managerID int64) (*DesktopOrganizationUsageStatistics, error) {
	var organization *DesktopOrganization
	var err error
	if managerID > 0 {
		organization, err = s.GetManagedOrganization(ctx, managerID)
	} else {
		organization, err = s.GetOrganization(ctx, organizationID)
	}
	if err != nil {
		return nil, err
	}
	now := s.now()
	result, err := s.usage.GetDesktopOrganizationUsage(ctx, organization.ID,
		timezone.StartOfDay(now).UTC(), timezone.StartOfWeek(now).UTC(), timezone.StartOfMonth(now).UTC(), now.UTC())
	if err != nil {
		return nil, ErrDesktopUsageUnavailable.WithCause(err)
	}
	result.Timezone = timezone.Location().String()
	result.AsOf = now.UTC()
	return result, nil
}
