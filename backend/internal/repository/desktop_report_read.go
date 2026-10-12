package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Only metadata is selected from the checkpoint table in list and summary reads.
const desktopReportReadMembers = `WITH records AS (
 SELECT member_id,COUNT(*) AS count FROM desktop_conversation_records
 WHERE organization_id=$1 AND stopped_at >= $3 AND stopped_at < $4 GROUP BY member_id
), members AS (
 SELECT m.public_id,m.name,m.deleted_at IS NOT NULL AS deleted,
 COALESCE((s.metadata->>'record_count')::int,c.count,0) AS record_count,
 CASE WHEN s.metadata IS NOT NULL THEN s.metadata->>'status'
 WHEN c.count>0 THEN CASE WHEN t.metadata->>'model'<>'' THEN 'not_in_snapshot' ELSE 'pending' END
 ELSE 'no_records' END AS status,
 COALESCE(s.metadata->>'model','') AS model,s.metadata->'generated_at' AS generated_at,
 COALESCE(s.metadata->>'error','') AS error,s.member
 FROM desktop_members m LEFT JOIN records c ON c.member_id=m.id
 LEFT JOIN desktop_daily_report_tasks t ON t.organization_id=$1 AND t.report_date=$2::date
 LEFT JOIN desktop_daily_report_members s ON s.task_id=t.id AND s.member_id=m.public_id
 WHERE m.organization_id=$1 AND (m.deleted_at IS NULL OR c.count>0 OR s.metadata IS NOT NULL)
) `

func (r *desktopReportRepository) ReadDay(ctx context.Context, org int64, date string, q service.DesktopReportQuery) (*service.DesktopReportDay, error) {
	start, end, err := reportBounds(date)
	if err != nil {
		return nil, err
	}
	day := &service.DesktopReportDay{Summary: service.DesktopGeneratedReport{Status: "pending"}, Date: date, Timezone: "Asia/Shanghai", Status: "pending", Members: []service.DesktopReportMember{}, MissingMembers: []string{}, Page: q.Page, PageSize: q.PageSize}
	var payload []byte
	var task service.DesktopReportTask
	err = r.db.QueryRowContext(ctx, `SELECT id,status,reason,revision,metadata || jsonb_build_object('summary',CASE WHEN $3 THEN summary ELSE '{}'::jsonb END) FROM desktop_daily_report_tasks WHERE organization_id=$1 AND report_date=$2::date`, org, date, q.Section == "summary").Scan(&task.ID, &task.Status, &task.Reason, &task.Revision, &payload)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		if err = json.Unmarshal(payload, &task.Payload); err != nil {
			return nil, err
		}
		task.OrganizationID, task.Date = org, date
		day.Task, day.Status, day.Reason, day.Summary = &task, task.Status, task.Reason, task.Payload.Summary
	}
	var missing []byte
	err = r.db.QueryRowContext(ctx, desktopReportReadMembers+`SELECT COUNT(*),COUNT(*) FILTER(WHERE record_count=0),COUNT(*) FILTER(WHERE record_count>0 AND status<>'not_in_snapshot'),COUNT(*) FILTER(WHERE record_count>0 AND status='completed'),COALESCE(jsonb_agg(public_id ORDER BY public_id) FILTER(WHERE record_count>0 AND status NOT IN ('completed','not_in_snapshot')),'[]') FROM members`, org, date, start, end).Scan(&day.TotalMembers, &day.NoRecordMembers, &day.ExpectedMembers, &day.CompletedMembers, &missing)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(missing, &day.MissingMembers); err != nil {
		return nil, err
	}
	if q.Section == "summary" {
		if day.Summary.Status == "completed" && day.CompletedMembers < day.ExpectedMembers {
			day.Summary.Status = "partial"
		}
		day.Summary.Executions, err = r.reportExecutions(ctx, org, date, "", q.ExecutionPage)
		if err != nil {
			return nil, err
		}
		day.Queue, err = r.reportQueue(ctx, org)
		if err != nil {
			return nil, err
		}
		day.MissingDates, err = r.reportMissingDates(ctx, org, date)
		return day, err
	}
	rows, err := r.db.QueryContext(ctx, desktopReportReadMembers+`SELECT jsonb_build_object('member_id',public_id,'name',name,'deleted',deleted,'record_count',record_count,'status',status,'model',model,'generated_at',generated_at,'error',error,'content',CASE WHEN $6<>'' THEN member->>'content' ELSE '' END,'source_ids',CASE WHEN $6<>'' THEN COALESCE(member->'source_ids','[]') ELSE '[]'::jsonb END,'executions','[]'::jsonb) FROM members WHERE ($5='' OR strpos(lower(name),lower($5))>0 OR strpos(public_id,$5)>0) AND ($6='' OR public_id=$6) ORDER BY name,public_id LIMIT $7 OFFSET $8`, org, date, start, end, q.Search, q.MemberID, q.PageSize, (q.Page-1)*q.PageSize)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var raw []byte
		var member service.DesktopReportMember
		if err = rows.Scan(&raw); err != nil {
			break
		}
		if err = json.Unmarshal(raw, &member); err != nil {
			break
		}
		day.Members = append(day.Members, member)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	if q.MemberID != "" {
		if len(day.Members) == 0 {
			return nil, service.ErrDesktopMemberNotFound
		}
		day.Members[0].Executions, err = r.reportExecutions(ctx, org, date, q.MemberID, q.ExecutionPage)
	} else if q.Search != "" {
		err = r.db.QueryRowContext(ctx, desktopReportReadMembers+`SELECT COUNT(*) FROM members WHERE strpos(lower(name),lower($5))>0 OR strpos(public_id,$5)>0`, org, date, start, end, q.Search).Scan(&day.TotalMembers)
	}
	return day, err
}

func (r *desktopReportRepository) reportExecutions(ctx context.Context, org int64, date, member string, page int) ([]service.DesktopReportExecution, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,revision,kind,member_id,model,round,chunk,attempt,request_id,status,error,started_at,finished_at,total_tokens,actual_cost FROM desktop_daily_report_executions WHERE organization_id=$1 AND report_date=$2::date AND member_id=$3 ORDER BY id DESC LIMIT 50 OFFSET $4`, org, date, member, (page-1)*50)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []service.DesktopReportExecution{}
	for rows.Next() {
		var e service.DesktopReportExecution
		if err = rows.Scan(&e.ID, &e.Revision, &e.Kind, &e.MemberID, &e.Model, &e.Round, &e.Chunk, &e.Attempt, &e.RequestID, &e.Status, &e.Error, &e.StartedAt, &e.FinishedAt, &e.TotalTokens, &e.ActualCost); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

func (r *desktopReportRepository) reportQueue(ctx context.Context, org int64) (*service.DesktopReportQueue, error) {
	q := &service.DesktopReportQueue{}
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FILTER(WHERE status IN ('pending','waiting')),COUNT(*) FILTER(WHERE status='running'),MIN(created_at) FILTER(WHERE status IN ('pending','waiting')) FROM desktop_daily_report_tasks WHERE organization_id=$1 AND status IN ('pending','waiting','running')`, org).Scan(&q.Pending, &q.Running, &q.OldestPendingAt)
	return q, err
}

func (r *desktopReportRepository) reportMissingDates(ctx context.Context, org int64, date string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT (c.stopped_at AT TIME ZONE 'Asia/Shanghai')::date::text FROM desktop_conversation_records c JOIN desktop_organizations o ON o.id=c.organization_id WHERE c.organization_id=$1 AND c.stopped_at >= GREATEST(($2::date-INTERVAL '30 days') AT TIME ZONE 'Asia/Shanghai',date_trunc('day',o.summary_enabled_at AT TIME ZONE 'Asia/Shanghai') AT TIME ZONE 'Asia/Shanghai') AND (COALESCE((o.management_policy->>'report_retention_days')::int,0)=0 OR (c.stopped_at AT TIME ZONE 'Asia/Shanghai')::date >= (NOW() AT TIME ZONE 'Asia/Shanghai')::date - (o.management_policy->>'report_retention_days')::int) AND c.stopped_at < ($2::date+INTERVAL '1 day') AT TIME ZONE 'Asia/Shanghai' AND NOT EXISTS(SELECT 1 FROM desktop_daily_report_tasks t WHERE t.organization_id=$1 AND t.report_date=(c.stopped_at AT TIME ZONE 'Asia/Shanghai')::date) ORDER BY 1 DESC LIMIT 30`, org, date)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []string{}
	for rows.Next() {
		var d string
		if err = rows.Scan(&d); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}
