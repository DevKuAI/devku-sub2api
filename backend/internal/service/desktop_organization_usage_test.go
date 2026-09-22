package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

type desktopOrganizationUsageStub struct {
	DesktopUsageRepository
	organizationID int64
	boundaries     []time.Time
	err            error
}

func (r *desktopOrganizationUsageStub) GetDesktopOrganizationUsage(_ context.Context, organizationID int64, today, week, month, end time.Time) (*DesktopOrganizationUsageStatistics, error) {
	r.organizationID = organizationID
	r.boundaries = []time.Time{today, week, month, end}
	return &DesktopOrganizationUsageStatistics{Total: DesktopOrganizationUsagePeriod{TotalTokens: 1234, ActualCost: 1.25}}, r.err
}

func TestDesktopOrganizationUsageStatisticsCalendarAndScope(t *testing.T) {
	original := timezone.Location().String()
	t.Cleanup(func() { require.NoError(t, timezone.Init(original)) })
	require.NoError(t, timezone.Init("Asia/Shanghai"))
	now := time.Date(2026, 9, 22, 4, 0, 0, 0, time.UTC)
	for _, managerID := range []int64{0, 42} {
		usage := &desktopOrganizationUsageStub{}
		identity := &desktopRepositoryStub{organization: &DesktopOrganization{ID: 7, PublicID: "org_owned", ConversationReportingEnabled: false}}
		svc := &DesktopService{repo: identity, usage: usage, now: func() time.Time { return now }}
		result, err := svc.OrganizationUsageStatistics(context.Background(), "org_untrusted", managerID)
		require.NoError(t, err)
		require.EqualValues(t, 7, usage.organizationID)
		require.Equal(t, []time.Time{time.Date(2026, 9, 21, 16, 0, 0, 0, time.UTC), time.Date(2026, 9, 20, 16, 0, 0, 0, time.UTC), time.Date(2026, 8, 31, 16, 0, 0, 0, time.UTC), now}, usage.boundaries)
		require.Equal(t, "Asia/Shanghai", result.Timezone)
		require.Equal(t, now, result.AsOf)
		require.EqualValues(t, 1234, result.Total.TotalTokens)
		require.InDelta(t, 1.25, result.Total.ActualCost, 0.000001)
		if managerID > 0 {
			require.Equal(t, []int64{42}, identity.scopedUserIDs)
		}
	}
}

func TestDesktopOrganizationUsageStatisticsMissingScopeAndStorageFailure(t *testing.T) {
	usage := &desktopOrganizationUsageStub{}
	svc := &DesktopService{repo: &desktopRepositoryStub{}, usage: usage, now: time.Now}
	_, err := svc.OrganizationUsageStatistics(context.Background(), "org_other", 42)
	require.ErrorIs(t, err, ErrDesktopOrganizationNotFound)
	require.Zero(t, usage.organizationID)
	svc.repo = &desktopRepositoryStub{organization: &DesktopOrganization{ID: 7}}
	usage.err = errors.New("database offline")
	_, err = svc.OrganizationUsageStatistics(context.Background(), "org_one", 0)
	require.ErrorIs(t, err, ErrDesktopUsageUnavailable)
}
