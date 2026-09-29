package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

type desktopConversationStatisticsStub struct {
	DesktopConversationRepository
	organizationID int64
	filters        DesktopConversationFilters
	periods        DesktopConversationPeriods
	err            error
}

func (r *desktopConversationStatisticsStub) Statistics(_ context.Context, organizationID int64, filters DesktopConversationFilters, periods DesktopConversationPeriods) (*DesktopConversationStatistics, error) {
	r.organizationID, r.filters, r.periods = organizationID, filters, periods
	return &DesktopConversationStatistics{}, r.err
}

func TestDesktopConversationStatisticsCalendarBoundaries(t *testing.T) {
	original := timezone.Location().String()
	t.Cleanup(func() { require.NoError(t, timezone.Init(original)) })
	for _, tc := range []struct{ name, zone, now, today, week, month string }{
		{"Shanghai midnight", "Asia/Shanghai", "2026-09-21T16:00:00Z", "2026-09-21T16:00:00Z", "2026-09-20T16:00:00Z", "2026-08-31T16:00:00Z"},
		{"Sunday across year", "Asia/Shanghai", "2027-01-03T15:59:59Z", "2027-01-02T16:00:00Z", "2026-12-27T16:00:00Z", "2026-12-31T16:00:00Z"},
		{"Monday new month", "UTC", "2027-02-01T00:00:00Z", "2027-02-01T00:00:00Z", "2027-02-01T00:00:00Z", "2027-02-01T00:00:00Z"},
		{"DST change", "America/New_York", "2026-03-09T04:30:00Z", "2026-03-09T04:00:00Z", "2026-03-09T04:00:00Z", "2026-03-01T05:00:00Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, timezone.Init(tc.zone))
			now, err := time.Parse(time.RFC3339, tc.now)
			require.NoError(t, err)
			records := &desktopConversationStatisticsStub{}
			svc := &DesktopService{repo: &desktopRepositoryStub{organization: &DesktopOrganization{ID: 7}}, conversations: records, now: func() time.Time { return now }}
			stats, err := svc.ConversationStatistics(context.Background(), "org_one", 0, DesktopConversationFilters{})
			require.NoError(t, err)
			require.Equal(t, tc.today, records.periods.Today.UTC().Format(time.RFC3339))
			require.Equal(t, tc.week, records.periods.Week.UTC().Format(time.RFC3339))
			require.Equal(t, tc.month, records.periods.Month.UTC().Format(time.RFC3339))
			require.Equal(t, tc.zone, stats.Timezone)
			require.Equal(t, now, stats.AsOf)
		})
	}
}

func TestDesktopConversationStatisticsAuthorizationAndFilters(t *testing.T) {
	for _, managerID := range []int64{0, 42} {
		for _, enabled := range []bool{false, true} {
			records := &desktopConversationStatisticsStub{}
			identity := &desktopRepositoryStub{organization: &DesktopOrganization{ID: 7, PublicID: "org_owned", ConversationReportingEnabled: enabled}}
			svc := &DesktopService{repo: identity, conversations: records, now: time.Now}
			filters := DesktopConversationFilters{MemberSearch: "member", Client: "workbuddy"}
			_, err := svc.ConversationStatistics(context.Background(), "org_other", managerID, filters)
			if managerID > 0 && !enabled {
				require.ErrorIs(t, err, ErrDesktopConversationReportingDisabled)
				require.Zero(t, records.organizationID)
			} else {
				require.NoError(t, err)
				require.EqualValues(t, 7, records.organizationID)
				require.Equal(t, filters, records.filters)
			}
			if managerID > 0 {
				require.Equal(t, []int64{42}, identity.scopedUserIDs)
			}
		}
	}
	records := &desktopConversationStatisticsStub{err: ErrDesktopConversationStorage}
	svc := &DesktopService{repo: &desktopRepositoryStub{organization: &DesktopOrganization{ID: 7}}, conversations: records, now: time.Now}
	_, err := svc.ConversationStatistics(context.Background(), "org_one", 0, DesktopConversationFilters{RecordID: "invalid"})
	require.ErrorIs(t, err, ErrDesktopValidation)
	require.Zero(t, records.organizationID)
	_, err = svc.ConversationStatistics(context.Background(), "org_one", 0, DesktopConversationFilters{})
	require.ErrorIs(t, err, ErrDesktopConversationStorage)
}

func TestDesktopConversationStatisticsSelectedRange(t *testing.T) {
	original := timezone.Location().String()
	t.Cleanup(func() { require.NoError(t, timezone.Init(original)) })
	require.NoError(t, timezone.Init("Asia/Shanghai"))
	now := time.Date(2026, 9, 22, 4, 0, 0, 0, time.UTC)
	records := &desktopConversationStatisticsStub{}
	svc := &DesktopService{repo: &desktopRepositoryStub{organization: &DesktopOrganization{ID: 7}}, conversations: records, now: func() time.Time { return now }}
	stats, err := svc.ConversationStatistics(context.Background(), "org_one", 0, DesktopConversationFilters{
		AnalyticsRange: &DesktopAnalyticsRangeInput{FromDate: "2026-09-19", ToDate: "2026-09-21"},
	})
	require.NoError(t, err)
	require.Equal(t, "2026-09-19", stats.RangeStart)
	require.Equal(t, "2026-09-21", stats.RangeEnd)
	require.Equal(t, 3, records.periods.TrendDays)
	require.Equal(t, records.periods.TrendStart, *records.filters.ReceivedFrom)
	require.Equal(t, records.periods.TrendEnd, *records.filters.ReceivedTo)
	_, err = svc.ConversationStatistics(context.Background(), "org_one", 0, DesktopConversationFilters{
		AnalyticsRange: &DesktopAnalyticsRangeInput{Days: 7}, ReceivedFrom: &now,
	})
	require.ErrorIs(t, err, ErrDesktopValidation)
}
