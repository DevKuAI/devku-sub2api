package service

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

func TestDesktopAnalyticsRangeResolve(t *testing.T) {
	original := timezone.Location().String()
	t.Cleanup(func() { require.NoError(t, timezone.Init(original)) })
	require.NoError(t, timezone.Init("Asia/Shanghai"))
	now := time.Date(2026, 9, 22, 4, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		input                                  DesktopAnalyticsRangeInput
		start, end, previousStart, previousEnd string
		days                                   int
	}{
		{DesktopAnalyticsRangeInput{}, "2026-08-24", "2026-09-22", "2026-07-25", "2026-08-23", 30},
		{DesktopAnalyticsRangeInput{Days: 7}, "2026-09-16", "2026-09-22", "2026-09-09", "2026-09-15", 7},
		{DesktopAnalyticsRangeInput{FromDate: "2026-09-19", ToDate: "2026-09-21"}, "2026-09-19", "2026-09-21", "2026-09-16", "2026-09-18", 3},
	} {
		selection, err := tc.input.Resolve(now)
		require.NoError(t, err)
		require.Equal(t, tc.start, selection.StartDate)
		require.Equal(t, tc.end, selection.EndDate)
		require.Equal(t, tc.days, selection.Days)
		require.Equal(t, tc.previousStart, selection.PreviousStart.In(timezone.Location()).Format("2006-01-02"))
		require.Equal(t, tc.previousEnd, selection.PreviousEnd.In(timezone.Location()).Add(-time.Nanosecond).Format("2006-01-02"))
	}
	for _, input := range []DesktopAnalyticsRangeInput{
		{Days: 8}, {Days: 7, FromDate: "2026-09-01", ToDate: "2026-09-07"},
		{FromDate: "2026-09-01"}, {FromDate: "2026-09-23", ToDate: "2026-09-23"},
		{FromDate: "2026-09-22", ToDate: "2026-09-21"}, {FromDate: "2026-06-01", ToDate: "2026-09-22"},
		{FromDate: "2026-02-30", ToDate: "2026-03-01"},
	} {
		_, err := input.Resolve(now)
		require.ErrorIs(t, err, ErrDesktopValidation)
	}
}

func TestDesktopAnalyticsRangeAcrossDST(t *testing.T) {
	original := timezone.Location().String()
	t.Cleanup(func() { require.NoError(t, timezone.Init(original)) })
	require.NoError(t, timezone.Init("America/New_York"))
	now := time.Date(2026, 3, 10, 16, 0, 0, 0, time.UTC)
	selection, err := (DesktopAnalyticsRangeInput{Days: 7}).Resolve(now)
	require.NoError(t, err)
	require.Equal(t, "2026-03-04", selection.StartDate)
	require.Equal(t, "2026-03-10", selection.EndDate)
	require.Equal(t, 7, selection.Days)
	require.Equal(t, 6*24*time.Hour+11*time.Hour, selection.End.Sub(selection.Start))
}
