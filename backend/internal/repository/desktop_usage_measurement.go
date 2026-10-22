package repository

import (
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const nativeTotal = `COALESCE(ul.input_tokens,0)::numeric+COALESCE(ul.output_tokens,0)::numeric+COALESCE(ul.cache_read_tokens,0)::numeric+COALESCE(ul.cache_creation_tokens,0)::numeric`
const nativeMeasured = `(COALESCE(NULLIF(ul.billing_mode,''),'token')='token' AND COALESCE(ul.image_count,0)=0 AND COALESCE(ul.video_count,0)=0 AND (` + nativeTotal + `)>0)`
const nativeWhere = ` WHERE ul.api_key_id IN (` + nativeKeys + `) AND ul.created_at >= $2 AND ul.created_at < $3`

const desktopMeasuredTotalSQL = `CASE WHEN ` + nativeMeasured + ` THEN (` + nativeTotal + `) ELSE 0::numeric END`

func desktopMeasuredComponentSQL(field string) string {
	return `CASE WHEN ` + nativeMeasured + ` THEN COALESCE(ul.` + field + `,0)::numeric ELSE 0::numeric END`
}
func desktopUsageStatus(measured, unmeasured int64) string {
	return service.DesktopUsageTokenStatus(measured, unmeasured)
}

func desktopUsageMeasurements(w service.DesktopUsageWindows, out *service.DesktopOrganizationUsageStatistics) (string, []any, []any, func()) {
	ranges := []struct {
		start, end time.Time
		target     *service.DesktopOrganizationUsagePeriod
	}{
		{w.Today, w.AsOf, &out.Today}, {w.Week, w.AsOf, &out.Week}, {w.Month, w.AsOf, &out.Month},
		{time.Time{}, w.AsOf, &out.Total}, {w.Selected.Start, w.Selected.End, &out.Selected},
		{w.Selected.PreviousStart, w.Selected.PreviousEnd, &out.Previous},
		{timezone.StartOfDay(w.AsOf).AddDate(0, 0, -29), w.AsOf, &out.Last30Days},
	}
	args := []any{}
	measurements := make([]service.DesktopUsageMeasurement, len(ranges))
	columns := []string{}
	dest := []any{}
	for i, window := range ranges {
		args = append(args, window.start, window.end)
		predicate := fmt.Sprintf("ul.created_at >= $%d AND ul.created_at < $%d", len(args)+4, len(args)+5)
		columns = append(columns, "COUNT(*) FILTER(WHERE "+predicate+" AND "+nativeMeasured+")", "COUNT(*) FILTER(WHERE "+predicate+" AND NOT "+nativeMeasured+")")
		dest = append(dest, &measurements[i].MeasuredRequests, &measurements[i].UnmeasuredRequests)
	}
	return strings.Join(columns, ","), args, dest, func() {
		for i, window := range ranges {
			measurement := measurements[i]
			measurement.TokenStatus = desktopUsageStatus(measurement.MeasuredRequests, measurement.UnmeasuredRequests)
			window.target.DesktopUsageMeasurement = measurement
		}
	}
}
