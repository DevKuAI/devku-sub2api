package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const nativeKeys = `SELECT a.api_key_id FROM desktop_member_api_keys a JOIN desktop_members m ON m.id=a.member_id WHERE m.organization_id=$1
 UNION SELECT id FROM api_keys WHERE desktop_analysis_organization_id=$1`
const nativeTotal = `COALESCE(ul.input_tokens,0)::numeric+COALESCE(ul.output_tokens,0)::numeric+COALESCE(ul.cache_read_tokens,0)::numeric+COALESCE(ul.cache_creation_tokens,0)::numeric`
const nativeMeasured = `(COALESCE(NULLIF(ul.billing_mode,''),'token')='token' AND COALESCE(ul.image_count,0)=0 AND COALESCE(ul.video_count,0)=0 AND (` + nativeTotal + `)>0)`
const nativeWhere = ` WHERE ul.api_key_id IN (` + nativeKeys + `) AND ul.created_at >= $2 AND ul.created_at < $3`

func nativeSum(expression string) string {
	return `CASE WHEN COUNT(*)>0 AND COUNT(*) FILTER(WHERE ` + nativeMeasured + `)=0 THEN NULL ELSE COALESCE(SUM(` + expression + `) FILTER(WHERE ` + nativeMeasured + `),0)::text END`
}
func (r *usageLogRepository) NativeUsage(ctx context.Context, org int64, selected service.DesktopAnalyticsRange) (*service.DesktopNativeUsage, error) {
	out := &service.DesktopNativeUsage{Range: service.DesktopNativeRange{Start: selected.StartDate, End: selected.EndDate, Timezone: selected.Start.Location().String(), IncludesToday: true}, AsOf: selected.End.UTC(), Daily: []service.DesktopNativeUsageDay{}}
	out.ComparisonRange = service.DesktopNativeRange{Start: selected.PreviousStart.Format("2006-01-02"), End: selected.PreviousEnd.Add(-time.Nanosecond).Format("2006-01-02"), Timezone: out.Range.Timezone}
	columns := []string{nativeSum(`COALESCE(ul.input_tokens,0)::numeric+COALESCE(ul.cache_read_tokens,0)::numeric+COALESCE(ul.cache_creation_tokens,0)::numeric`), nativeSum(`COALESCE(ul.output_tokens,0)::numeric`), nativeSum(`COALESCE(ul.cache_read_tokens,0)::numeric`), nativeSum(`COALESCE(ul.cache_creation_tokens,0)::numeric`), nativeSum(nativeTotal), `COUNT(*) FILTER(WHERE ` + nativeMeasured + `)`, `COUNT(*) FILTER(WHERE NOT ` + nativeMeasured + `)`, `COUNT(*)`}
	err := scanSingleRow(ctx, r.sql, `SELECT `+strings.Join(columns, ",")+` FROM usage_logs ul`+nativeWhere, []any{org, selected.Start.UTC(), selected.End.UTC()}, &out.Totals.Input, &out.Totals.Output, &out.Totals.CacheRead, &out.Totals.CacheCreation, &out.Totals.Total, &out.Totals.Measured, &out.Totals.Unmeasured, &out.Requests)
	if err != nil {
		return nil, err
	}
	out.Totals.Status = "ready"
	if out.Totals.Unmeasured > 0 {
		out.Totals.Status = "partial"
	}
	if out.Totals.Total == nil {
		out.Totals.Status = "unavailable"
	}
	err = scanSingleRow(ctx, r.sql, `SELECT `+nativeSum(nativeTotal)+` FROM usage_logs ul`+nativeWhere, []any{org, selected.PreviousStart.UTC(), selected.PreviousEnd.UTC()}, &out.PreviousTokens)
	if err != nil {
		return nil, err
	}
	err = scanSingleRow(ctx, r.sql, `SELECT `+nativeSum(nativeTotal)+` FROM usage_logs ul JOIN api_keys k ON k.id=ul.api_key_id WHERE k.desktop_analysis_organization_id=$1 AND ul.created_at >= $2 AND ul.created_at < $3`, []any{org, selected.Start.UTC(), selected.End.UTC()}, &out.AnalysisTokens)
	if err != nil {
		return nil, err
	}
	err = scanSingleRow(ctx, r.sql, `SELECT COUNT(DISTINCT m.id) FROM desktop_members m JOIN desktop_member_api_keys a ON a.member_id=m.id JOIN usage_logs ul ON ul.api_key_id=a.api_key_id WHERE m.organization_id=$1 AND ul.created_at >= $2 AND ul.created_at < $3`, []any{org, selected.Start.UTC(), selected.End.UTC()}, &out.ObservedMembers)
	if err != nil {
		return nil, err
	}
	rows, err := r.sql.QueryContext(ctx, `SELECT to_char(ul.created_at AT TIME ZONE $4,'YYYY-MM-DD'),`+nativeSum(nativeTotal)+` FROM usage_logs ul`+nativeWhere+` GROUP BY 1 ORDER BY 1`, org, selected.Start.UTC(), selected.End.UTC(), out.Range.Timezone)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	byDay := map[string]*string{}
	for rows.Next() {
		var day string
		var token *string
		if err = rows.Scan(&day, &token); err != nil {
			return nil, err
		}
		byDay[day] = token
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for i := 0; i < selected.Days; i++ {
		day := selected.Start.AddDate(0, 0, i).Format("2006-01-02")
		value, exists := byDay[day]
		if !exists {
			zero := "0"
			value = &zero
		}
		out.Daily = append(out.Daily, service.DesktopNativeUsageDay{Date: day, Tokens: value})
	}
	return out, nil
}
func (r *usageLogRepository) NativeModels(ctx context.Context, org int64, selected service.DesktopAnalyticsRange, member string, page, size int) ([]service.DesktopNativeModel, int64, error) {
	where := nativeWhere
	args := []any{org, selected.Start.UTC(), selected.End.UTC()}
	if member != "" {
		where = ` WHERE ul.api_key_id IN(SELECT a.api_key_id FROM desktop_member_api_keys a JOIN desktop_members m ON m.id=a.member_id WHERE m.organization_id=$1 AND m.public_id=$4) AND ul.created_at >= $2 AND ul.created_at < $3`
		args = append(args, member)
	}
	model := `COALESCE(NULLIF(ul.requested_model,''),NULLIF(ul.model,''),'unknown')`
	var total int64
	if err := scanSingleRow(ctx, r.sql, `SELECT COUNT(*) FROM(SELECT `+model+` FROM usage_logs ul`+where+` GROUP BY 1) models`, args, &total); err != nil {
		return nil, 0, err
	}
	count := len(args)
	args = append(args, size, (page-1)*size)
	query := `SELECT ` + model + `,` + nativeSum(nativeTotal) + `,COUNT(*) FROM usage_logs ul` + where + fmt.Sprintf(` GROUP BY 1 ORDER BY SUM((%s)::numeric) DESC NULLS LAST,1 LIMIT $%d OFFSET $%d`, nativeTotal, count+1, count+2)
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.DesktopNativeModel{}
	for rows.Next() {
		var row service.DesktopNativeModel
		if err = rows.Scan(&row.Model, &row.Tokens, &row.Requests); err != nil {
			return nil, 0, err
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}
func (r *usageLogRepository) NativeMembers(ctx context.Context, org int64, selected service.DesktopAnalyticsRange, search, status string, page, size int, withUsage bool) ([]service.DesktopNativeMember, int64, error) {
	where := ` WHERE m.organization_id=$1 AND (m.deleted_at IS NULL OR EXISTS(SELECT 1 FROM desktop_member_api_keys a JOIN usage_logs ul ON ul.api_key_id=a.api_key_id WHERE a.member_id=m.id AND ul.created_at >= $2 AND ul.created_at < $3)) AND ($4='' OR strpos(lower(m.name),lower($4))>0) AND ($5='' OR m.status=$5)`
	args := []any{org, selected.Start.UTC(), selected.End.UTC(), search, status}
	var total int64
	if err := scanSingleRow(ctx, r.sql, `SELECT COUNT(*) FROM desktop_members m`+where, args, &total); err != nil {
		return nil, 0, err
	}
	usage := `NULL::text,NULL::bigint`
	if withUsage {
		usage = `(SELECT ` + nativeSum(nativeTotal) + ` FROM usage_logs ul WHERE ul.api_key_id IN(SELECT api_key_id FROM desktop_member_api_keys WHERE member_id=m.id) AND ul.created_at >= $2 AND ul.created_at < $3),(SELECT COUNT(*) FROM usage_logs ul WHERE ul.api_key_id IN(SELECT api_key_id FROM desktop_member_api_keys WHERE member_id=m.id) AND ul.created_at >= $2 AND ul.created_at < $3)`
	}
	args = append(args, size, (page-1)*size)
	rows, err := r.sql.QueryContext(ctx, `SELECT m.public_id,m.name,m.status,m.deleted_at IS NOT NULL,`+usage+` FROM desktop_members m`+where+` ORDER BY m.name,m.public_id LIMIT $6 OFFSET $7`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.DesktopNativeMember{}
	for rows.Next() {
		var m service.DesktopNativeMember
		if err = rows.Scan(&m.ID, &m.Name, &m.Status, &m.Deleted, &m.Tokens, &m.Requests); err != nil {
			return nil, 0, err
		}
		out = append(out, m)
	}
	return out, total, rows.Err()
}
func (r *usageLogRepository) NativeMember(ctx context.Context, org int64, selected service.DesktopAnalyticsRange, id string, withUsage bool) (*service.DesktopNativeMember, error) {
	var m service.DesktopNativeMember
	err := scanSingleRow(ctx, r.sql, `SELECT public_id,name,status,deleted_at IS NOT NULL FROM desktop_members WHERE organization_id=$1 AND public_id=$2`, []any{org, id}, &m.ID, &m.Name, &m.Status, &m.Deleted)
	if err == sql.ErrNoRows {
		return nil, service.ErrDesktopMemberNotFound
	}
	if err != nil {
		return nil, err
	}
	if !withUsage {
		return &m, nil
	}
	var requests int64
	err = scanSingleRow(ctx, r.sql, `SELECT `+nativeSum(nativeTotal)+`,COUNT(*) FROM usage_logs ul WHERE ul.api_key_id IN(SELECT a.api_key_id FROM desktop_member_api_keys a JOIN desktop_members m ON m.id=a.member_id WHERE m.organization_id=$1 AND m.public_id=$4) AND ul.created_at >= $2 AND ul.created_at < $3`, []any{org, selected.Start.UTC(), selected.End.UTC(), id}, &m.Tokens, &requests)
	m.Requests = &requests
	return &m, err
}
