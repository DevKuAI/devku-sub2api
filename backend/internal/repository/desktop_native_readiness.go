package repository

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Correlate members by identity; receipt dates and summary dates stay distinct.
const nativeReadinessQuery = `WITH used_members AS (
 SELECT DISTINCT a.member_id FROM desktop_member_api_keys a
 JOIN desktop_members m ON m.id=a.member_id JOIN usage_logs ul ON ul.api_key_id=a.api_key_id
 WHERE m.organization_id=$1 AND ul.created_at >= $2 AND ul.created_at < $3
), reported_members AS (
 SELECT DISTINCT member_id FROM desktop_conversation_records WHERE organization_id=$1 AND received_at >= $2 AND received_at < $3
), source_dates AS (
 SELECT DISTINCT (c.stopped_at AT TIME ZONE 'Asia/Shanghai')::date AS date
 FROM desktop_conversation_records c JOIN desktop_organizations o ON o.id=c.organization_id
 WHERE c.organization_id=$1 AND c.stopped_at >= $2 AND c.stopped_at < $3
 AND c.stopped_at < date_trunc('day',$3::timestamptz AT TIME ZONE 'Asia/Shanghai') AT TIME ZONE 'Asia/Shanghai'
 AND (o.summary_enabled_at IS NULL OR c.stopped_at >= date_trunc('day',o.summary_enabled_at AT TIME ZONE 'Asia/Shanghai') AT TIME ZONE 'Asia/Shanghai')
 AND (COALESCE((o.management_policy->>'report_retention_days')::int,0)=0 OR (c.stopped_at AT TIME ZONE 'Asia/Shanghai')::date >= ($3::timestamptz AT TIME ZONE 'Asia/Shanghai')::date-(o.management_policy->>'report_retention_days')::int)
), pending_dates AS (
 SELECT s.date FROM source_dates s LEFT JOIN desktop_daily_report_tasks d ON d.organization_id=$1 AND d.report_date=s.date
 WHERE d.id IS NULL OR COALESCE(d.summary->>'status','') NOT IN ('completed','partial') OR COALESCE(d.summary->>'content','')=''
)
SELECT (SELECT COUNT(*) FROM used_members u WHERE NOT EXISTS(SELECT 1 FROM reported_members r WHERE r.member_id=u.member_id)),
 (SELECT COUNT(*) FROM desktop_conversation_records WHERE organization_id=$1 AND received_at >= $2 AND received_at < $3 AND capture_status='response_missing'),
 COALESCE((SELECT jsonb_agg(to_char(date,'YYYY-MM-DD') ORDER BY date DESC) FROM pending_dates),'[]'::jsonb)`

func (r *usageLogRepository) nativeReadiness(ctx context.Context, org int64, selected service.DesktopAnalyticsRange) (service.DesktopNativeReadiness, error) {
	var result service.DesktopNativeReadiness
	var raw []byte
	err := scanSingleRow(ctx, r.sql, nativeReadinessQuery, []any{org, selected.Start.UTC(), selected.End.UTC()}, &result.UsageWithoutReporting, &result.ResponseMissing, &raw)
	if err == nil {
		err = json.Unmarshal(raw, &result.PendingSummaryDates)
	}
	return result, err
}
