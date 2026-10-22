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

func nativeSum(expression string) string {
	return `CASE WHEN COUNT(*)>0 AND COUNT(*) FILTER(WHERE ` + nativeMeasured + `)=0 THEN NULL ELSE COALESCE(SUM(` + expression + `) FILTER(WHERE ` + nativeMeasured + `),0)::text END`
}

type nativePage[T any] struct {
	items []T
	total int64
}

// All components of a response read the same committed database snapshot.
func nativeSnapshot[T any](ctx context.Context, r *usageLogRepository, read func(*usageLogRepository) (T, error)) (T, error) {
	if r.db == nil {
		return read(r)
	}
	var empty T
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return empty, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := read(&usageLogRepository{sql: tx})
	if err != nil {
		return empty, err
	}
	if err = tx.Commit(); err != nil {
		return empty, err
	}
	return result, nil
}
func (r *usageLogRepository) NativeUsage(ctx context.Context, org int64, selected service.DesktopAnalyticsRange) (*service.DesktopNativeUsage, error) {
	return nativeSnapshot(ctx, r, func(reader *usageLogRepository) (*service.DesktopNativeUsage, error) {
		return reader.nativeUsage(ctx, org, selected)
	})
}

func (r *usageLogRepository) nativeUsage(ctx context.Context, org int64, selected service.DesktopAnalyticsRange) (*service.DesktopNativeUsage, error) {
	out := &service.DesktopNativeUsage{Range: service.DesktopNativeRange{Start: selected.StartDate, End: selected.EndDate, Timezone: selected.Start.Location().String(), IncludesToday: true}, AsOf: selected.End.UTC(), Daily: []service.DesktopNativeUsageDay{}}
	out.ComparisonRange = service.DesktopNativeRange{Start: selected.PreviousStart.Format("2006-01-02"), End: selected.PreviousEnd.Add(-time.Nanosecond).Format("2006-01-02"), Timezone: out.Range.Timezone}
	columns := []string{nativeSum(`COALESCE(ul.input_tokens,0)::numeric+COALESCE(ul.cache_read_tokens,0)::numeric+COALESCE(ul.cache_creation_tokens,0)::numeric`), nativeSum(`COALESCE(ul.output_tokens,0)::numeric`), nativeSum(`COALESCE(ul.cache_read_tokens,0)::numeric`), nativeSum(`COALESCE(ul.cache_creation_tokens,0)::numeric`), nativeSum(nativeTotal), `COUNT(*) FILTER(WHERE ` + nativeMeasured + `)`, `COUNT(*) FILTER(WHERE NOT ` + nativeMeasured + `)`, `COUNT(*)`}
	err := scanSingleRow(ctx, r.sql, `SELECT `+strings.Join(columns, ",")+` FROM usage_logs ul`+nativeWhere, []any{org, selected.Start.UTC(), selected.End.UTC()}, &out.Totals.Input, &out.Totals.Output, &out.Totals.CacheRead, &out.Totals.CacheCreation, &out.Totals.Total, &out.Totals.Measured, &out.Totals.Unmeasured, &out.Requests)
	if err != nil {
		return nil, err
	}
	out.Totals.Status = desktopUsageStatus(out.Totals.Measured, out.Totals.Unmeasured)

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
	readiness, err := r.nativeReadiness(ctx, org, selected)
	if err != nil {
		return nil, err
	}
	out.Readiness = readiness
	return out, nil
}
func (r *usageLogRepository) NativeModels(ctx context.Context, org int64, selected service.DesktopAnalyticsRange, member string, page, size int) ([]service.DesktopNativeModel, int64, error) {
	out, err := nativeSnapshot(ctx, r, func(reader *usageLogRepository) (nativePage[service.DesktopNativeModel], error) {
		items, total, err := reader.nativeModels(ctx, org, selected, member, page, size)
		return nativePage[service.DesktopNativeModel]{items, total}, err
	})
	return out.items, out.total, err
}
func (r *usageLogRepository) nativeModels(ctx context.Context, org int64, selected service.DesktopAnalyticsRange, member string, page, size int) ([]service.DesktopNativeModel, int64, error) {
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
func (r *usageLogRepository) NativeMembers(ctx context.Context, org int64, selected service.DesktopAnalyticsRange, search, status string, page, size int, withUsage bool, missingReporting ...bool) ([]service.DesktopNativeMember, int64, error) {
	out, err := nativeSnapshot(ctx, r, func(reader *usageLogRepository) (nativePage[service.DesktopNativeMember], error) {
		items, total, err := reader.nativeMembers(ctx, org, selected, search, status, page, size, withUsage, missingReporting...)
		return nativePage[service.DesktopNativeMember]{items, total}, err
	})
	return out.items, out.total, err
}
func (r *usageLogRepository) nativeMembers(ctx context.Context, org int64, selected service.DesktopAnalyticsRange, search, status string, page, size int, withUsage bool, missingReporting ...bool) ([]service.DesktopNativeMember, int64, error) {
	where := ` WHERE m.organization_id=$1 AND (m.deleted_at IS NULL OR EXISTS(SELECT 1 FROM desktop_member_api_keys a JOIN usage_logs ul ON ul.api_key_id=a.api_key_id WHERE a.member_id=m.id AND ul.created_at >= $2 AND ul.created_at < $3)) AND ($4='' OR strpos(lower(m.name),lower($4))>0) AND ($5='' OR m.status=$5)`
	if len(missingReporting) > 0 && missingReporting[0] {
		where += ` AND EXISTS(SELECT 1 FROM desktop_member_api_keys a JOIN usage_logs ul ON ul.api_key_id=a.api_key_id WHERE a.member_id=m.id AND ul.created_at >= $2 AND ul.created_at < $3) AND NOT EXISTS(SELECT 1 FROM desktop_conversation_records c WHERE c.organization_id=$1 AND c.member_id=m.id AND c.received_at >= $2 AND c.received_at < $3)`
	}
	args := []any{org, selected.Start.UTC(), selected.End.UTC(), search, status}
	var total int64
	if err := scanSingleRow(ctx, r.sql, `SELECT COUNT(*) FROM desktop_members m`+where, args, &total); err != nil {
		return nil, 0, err
	}
	args = append(args, size, (page-1)*size)
	query := `SELECT m.public_id,m.name,m.status,m.deleted_at IS NOT NULL,NULL::text,NULL::bigint FROM desktop_members m` + where + ` ORDER BY m.name,m.public_id LIMIT $6 OFFSET $7`
	if withUsage {
		query = `WITH page_members AS MATERIALIZED(SELECT m.id,m.public_id,m.name,m.status,m.deleted_at FROM desktop_members m` + where + ` ORDER BY m.name,m.public_id LIMIT $6 OFFSET $7),
  page_usage AS(SELECT a.member_id,` + nativeSum(nativeTotal) + ` AS tokens,COUNT(*) AS requests FROM page_members p JOIN desktop_member_api_keys a ON a.member_id=p.id JOIN usage_logs ul ON ul.api_key_id=a.api_key_id WHERE ul.created_at >= $2 AND ul.created_at < $3 GROUP BY a.member_id)
  SELECT m.public_id,m.name,m.status,m.deleted_at IS NOT NULL,CASE WHEN u.requests IS NULL THEN '0' ELSE u.tokens END,COALESCE(u.requests,0) FROM page_members m LEFT JOIN page_usage u ON u.member_id=m.id ORDER BY m.name,m.public_id`
	}
	rows, err := r.sql.QueryContext(ctx, query, args...)
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
	return nativeSnapshot(ctx, r, func(reader *usageLogRepository) (*service.DesktopNativeMember, error) {
		return reader.nativeMember(ctx, org, selected, id, withUsage)
	})
}
func (r *usageLogRepository) nativeMember(ctx context.Context, org int64, selected service.DesktopAnalyticsRange, id string, withUsage bool) (*service.DesktopNativeMember, error) {
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
