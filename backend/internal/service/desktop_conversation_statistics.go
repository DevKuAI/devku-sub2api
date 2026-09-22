package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

type DesktopConversationCounts struct {
	RecordCount int64 `json:"record_count"`
	PromptCount int64 `json:"prompt_count"`
}

type DesktopConversationStatistics struct {
	Timezone string                    `json:"timezone"`
	AsOf     time.Time                 `json:"as_of"`
	Today    DesktopConversationCounts `json:"today"`
	Week     DesktopConversationCounts `json:"week"`
	Month    DesktopConversationCounts `json:"month"`
	Total    DesktopConversationCounts `json:"total"`
}

type DesktopConversationPeriods struct {
	Today time.Time
	Week  time.Time
	Month time.Time
	AsOf  time.Time
}

func (s *DesktopService) ConversationStatistics(ctx context.Context, organizationID string, managerID int64, filters DesktopConversationFilters) (*DesktopConversationStatistics, error) {
	if err := filters.Validate(); err != nil {
		return nil, err
	}
	organization, err := s.conversationOrganization(ctx, organizationID, managerID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	periods := DesktopConversationPeriods{
		Today: timezone.StartOfDay(now), Week: timezone.StartOfWeek(now),
		Month: timezone.StartOfMonth(now), AsOf: now.UTC(),
	}
	result, err := s.conversations.Statistics(ctx, organization.ID, filters, periods)
	if err != nil {
		return nil, err
	}
	result.Timezone = timezone.Location().String()
	result.AsOf = periods.AsOf
	return result, nil
}
