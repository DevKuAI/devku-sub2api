package repository

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// A page contains complete model groups for a bounded set of members.
func (r *usageLogRepository) desktopMemberModelPage(ctx context.Context, org int64, selection service.DesktopAnalyticsRange, q service.DesktopUsageQuery) ([]service.DesktopUsageMemberModel, *service.DesktopMemberModelPage, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = 20
	}
	page := &service.DesktopMemberModelPage{Page: q.Page, PageSize: q.PageSize, Models: []string{}}
	rowsOut := []service.DesktopUsageMemberModel{}
	model := `COALESCE(NULLIF(ul.requested_model,''),NULLIF(ul.model,''),'unknown')`
	from := ` FROM desktop_members m JOIN desktop_member_api_keys a ON a.member_id=m.id JOIN usage_logs ul ON ul.api_key_id=a.api_key_id WHERE m.organization_id=$1 AND ul.created_at >= $2 AND ul.created_at < $3`
	totals := `WITH totals AS(SELECT m.public_id AS member_id,m.name,m.deleted_at IS NOT NULL AS deleted,` + model + ` AS model,COUNT(*) AS requests,
 SUM(` + desktopMeasuredComponentSQL("input_tokens") + `) AS input_tokens,SUM(` + desktopMeasuredComponentSQL("output_tokens") + `) AS output_tokens,
 SUM(` + desktopMeasuredComponentSQL("cache_creation_tokens") + `) AS cache_creation_tokens,SUM(` + desktopMeasuredComponentSQL("cache_read_tokens") + `) AS cache_read_tokens,
 SUM(` + desktopMeasuredTotalSQL + `) AS total_tokens,COUNT(*) FILTER(WHERE ` + nativeMeasured + `) AS measured,COUNT(*) FILTER(WHERE NOT ` + nativeMeasured + `) AS unmeasured` + from + `
 AND ($4='' OR strpos(lower(m.name||' '||m.public_id||' '||` + model + `),lower($4))>0) AND ($5='' OR ` + model + `=$5)
 GROUP BY m.id,m.public_id,m.name,m.deleted_at,4)`
	args := []any{org, selection.Start.UTC(), selection.End.UTC(), q.Search, q.Model}
	err := scanSingleRow(ctx, r.sql, totals+` SELECT COUNT(DISTINCT member_id),COUNT(*),COUNT(DISTINCT model),COALESCE(SUM(total_tokens),0),COALESCE(SUM(measured),0),COALESCE(SUM(unmeasured),0) FROM totals`, args, &page.TotalMembers, &page.TotalRows, &page.ModelCount, &page.TotalTokens, &page.MeasuredRequests, &page.UnmeasuredRequests)
	if err != nil {
		return nil, nil, err
	}
	page.TokenStatus = desktopUsageStatus(page.MeasuredRequests, page.UnmeasuredRequests)
	order := `tokens DESC,name,member_id`
	outputOrder := `s.tokens DESC,s.name,s.member_id,t.total_tokens DESC,t.model`
	if q.Sort == "name" {
		order = `name,member_id`
		outputOrder = `s.name,s.member_id,t.total_tokens DESC,t.model`
	}
	args = append(args, q.PageSize, (q.Page-1)*q.PageSize)
	rows, err := r.sql.QueryContext(ctx, totals+`,selected_members AS(SELECT member_id,MIN(name) AS name,SUM(total_tokens) AS tokens FROM totals GROUP BY member_id ORDER BY `+order+` LIMIT $6 OFFSET $7)
 SELECT t.member_id,t.name,t.deleted,t.model,t.requests,t.input_tokens,t.output_tokens,t.cache_creation_tokens,t.cache_read_tokens,t.total_tokens,t.measured,t.unmeasured FROM totals t JOIN selected_members s USING(member_id) ORDER BY `+outputOrder, args...)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var row service.DesktopUsageMemberModel
		if err = rows.Scan(&row.MemberID, &row.Name, &row.Deleted, &row.Model, &row.Requests, &row.InputTokens, &row.OutputTokens, &row.CacheCreationTokens, &row.CacheReadTokens, &row.TotalTokens, &row.MeasuredRequests, &row.UnmeasuredRequests); err != nil {
			break
		}
		row.TokenStatus = desktopUsageStatus(row.MeasuredRequests, row.UnmeasuredRequests)
		rowsOut = append(rowsOut, row)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return nil, nil, err
	}
	rows, err = r.sql.QueryContext(ctx, `SELECT DISTINCT `+model+from+` ORDER BY 1 LIMIT 500`, args[:3]...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, nil, err
		}
		page.Models = append(page.Models, value)
	}
	return rowsOut, page, rows.Err()
}
