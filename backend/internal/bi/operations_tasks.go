package bi

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// These projections intentionally never select report text, snapshots or checkpoint inputs.
const operationsTaskUnion = `
 SELECT d.id AS task_id,o.public_id AS organization_id,o.name AS organization_name,d.report_date::text AS report_id,'day'::text AS kind,d.report_date AS period_start,d.report_date AS period_end,d.revision,d.status,d.reason,COALESCE(d.metadata->>'model','') AS model,(d.summary->>'generated_at')::timestamptz AS generated_at,
 (SELECT COUNT(*) FROM desktop_daily_report_members m WHERE m.task_id=d.id AND m.metadata->>'status'='completed') AS available_sources,
 (SELECT COUNT(*) FROM desktop_daily_report_members m WHERE m.task_id=d.id AND m.metadata->>'status'<>'completed' AND COALESCE((m.metadata->>'record_count')::int,0)>0) AS missing_sources,
 (SELECT SUM(e.total_tokens)::text FROM desktop_daily_report_executions e WHERE e.task_id=d.id AND e.revision=d.revision) AS total_tokens,
 (SELECT SUM(e.actual_cost)::float8 FROM desktop_daily_report_executions e WHERE e.task_id=d.id AND e.revision=d.revision) AS actual_cost
 FROM desktop_daily_report_tasks d JOIN desktop_organizations o ON o.id=d.organization_id WHERE o.deleted_at IS NULL
 UNION ALL
 SELECT p.id,o.public_id,o.name,p.public_id,p.kind,p.period_start,p.period_end-1,p.revision,p.status,p.reason,COALESCE(p.configuration->>'model',''),p.generated_at,
 jsonb_array_length(p.source_versions),COALESCE(jsonb_array_length(p.coverage->'missing_dates'),0),
 (SELECT SUM(e.total_tokens)::text FROM desktop_period_report_executions e WHERE e.task_id=p.id AND e.revision=p.revision),
 (SELECT SUM(e.actual_cost)::float8 FROM desktop_period_report_executions e WHERE e.task_id=p.id AND e.revision=p.revision)
 FROM desktop_period_report_tasks p JOIN desktop_organizations o ON o.id=p.organization_id WHERE o.deleted_at IS NULL`
const executionUnion = `SELECT e.id,e.task_id,o.public_id AS organization_id,'day'::text AS kind,e.report_date AS period_start,e.report_date AS period_end,e.revision,e.model,e.round,e.chunk,e.attempt,e.request_id,e.status,e.error,e.started_at,e.finished_at,e.total_tokens::text,e.actual_cost::float8 FROM desktop_daily_report_executions e JOIN desktop_organizations o ON o.id=e.organization_id WHERE o.deleted_at IS NULL
 UNION ALL SELECT e.id,e.task_id,o.public_id,e.kind,e.period_start,e.period_end-1,e.revision,e.model,e.round,e.chunk,e.attempt,e.request_id,e.status,e.error,e.started_at,e.finished_at,e.total_tokens::text,e.actual_cost::float8 FROM desktop_period_report_executions e JOIN desktop_organizations o ON o.id=e.organization_id WHERE o.deleted_at IS NULL`

func (h *Handler) taskRows(c *gin.Context, single bool) {
	page, size, ok := queryPage(c)
	if !ok {
		return
	}
	kind := c.Query("kind")
	if single {
		kind = c.Param("kind")
	}
	if kind != "" && kind != "day" && kind != "week" && kind != "month" {
		WriteError(c, invalid("kind", "Invalid task kind"))
		return
	}
	id := int64(0)
	if single {
		var e error
		id, e = strconv.ParseInt(c.Param("task_id"), 10, 64)
		if e != nil || id < 1 {
			WriteError(c, ErrNotFound)
			return
		}
	}
	filter := ` WHERE ($1='' OR organization_id=$1) AND ($2='' OR kind=$2) AND ($3='' OR status=$3) AND ($4=0 OR task_id=$4)`
	args := []any{c.Query("organization_id"), kind, c.Query("status"), id}
	var total int64
	if e := h.service.db.QueryRowContext(c.Request.Context(), `WITH tasks AS (`+operationsTaskUnion+`) SELECT COUNT(*) FROM tasks`+filter, args...).Scan(&total); e != nil {
		WriteError(c, e)
		return
	}
	args = append(args, size, (page-1)*size)
	rows, e := h.service.db.QueryContext(c.Request.Context(), `WITH tasks AS (`+operationsTaskUnion+`) SELECT task_id,organization_id,organization_name,report_id,kind,period_start::text,period_end::text,revision,status,reason,model,generated_at,available_sources,missing_sources,total_tokens,actual_cost FROM tasks`+filter+` ORDER BY period_start DESC,task_id DESC LIMIT $5 OFFSET $6`, args...)
	if e != nil {
		WriteError(c, e)
		return
	}
	defer func() { _ = rows.Close() }()
	items := []gin.H{}
	for rows.Next() {
		var id, available, missing int64
		var org, name, report, kind, start, end, status, reason, model string
		var revision int
		var generated *time.Time
		var tokens *string
		var cost *float64
		if e = rows.Scan(&id, &org, &name, &report, &kind, &start, &end, &revision, &status, &reason, &model, &generated, &available, &missing, &tokens, &cost); e != nil {
			WriteError(c, e)
			return
		}
		items = append(items, gin.H{"task_id": id, "organization_id": org, "organization_name": name, "report_id": report, "kind": kind, "period_start": start, "period_end": end, "revision": revision, "status": status, "reason": reason, "model": model, "generated_at": generated, "available_sources": available, "missing_sources": missing, "total_tokens": tokens, "actual_cost": cost})
	}
	if e = rows.Err(); e != nil {
		WriteError(c, e)
		return
	}
	if single {
		if len(items) == 0 {
			WriteError(c, ErrNotFound)
			return
		}
		adminRespond(c, items[0], nil)
	} else {
		adminRespond(c, pageResult(items, page, size, total), nil)
	}
}
func (h *Handler) OperationsTasks(c *gin.Context) { h.taskRows(c, false) }
func (h *Handler) OperationsTask(c *gin.Context)  { h.taskRows(c, true) }
func (h *Handler) executionRows(c *gin.Context, task bool) {
	page, size, ok := queryPage(c)
	if !ok {
		return
	}
	kind := c.Query("kind")
	id := int64(0)
	if task {
		kind = c.Param("kind")
		var e error
		id, e = strconv.ParseInt(c.Param("task_id"), 10, 64)
		if e != nil || id < 1 {
			WriteError(c, ErrNotFound)
			return
		}
	}
	if kind != "" && kind != "day" && kind != "week" && kind != "month" {
		WriteError(c, invalid("kind", "Invalid execution kind"))
		return
	}
	filter := ` WHERE ($1='' OR organization_id=$1) AND ($2='' OR kind=$2) AND ($3=0 OR task_id=$3) AND ($4='' OR request_id=$4)`
	args := []any{c.Query("organization_id"), kind, id, c.Query("request_id")}
	var total int64
	if e := h.service.db.QueryRowContext(c.Request.Context(), `WITH executions AS (`+executionUnion+`) SELECT COUNT(*) FROM executions`+filter, args...).Scan(&total); e != nil {
		WriteError(c, e)
		return
	}
	args = append(args, size, (page-1)*size)
	rows, e := h.service.db.QueryContext(c.Request.Context(), `WITH executions AS (`+executionUnion+`) SELECT id,task_id,organization_id,kind,period_start::text,period_end::text,revision,model,round,chunk,attempt,request_id,status,error,started_at,finished_at,total_tokens,actual_cost FROM executions`+filter+` ORDER BY started_at DESC,id DESC LIMIT $5 OFFSET $6`, args...)
	if e != nil {
		WriteError(c, e)
		return
	}
	defer func() { _ = rows.Close() }()
	items := []gin.H{}
	for rows.Next() {
		var id int64
		var taskID *int64
		var org, kind, start, end, model, request, status, reason string
		var revision, round, chunk, attempt int
		var began time.Time
		var finished *time.Time
		var tokens *string
		var cost *float64
		if e = rows.Scan(&id, &taskID, &org, &kind, &start, &end, &revision, &model, &round, &chunk, &attempt, &request, &status, &reason, &began, &finished, &tokens, &cost); e != nil {
			WriteError(c, e)
			return
		}
		items = append(items, gin.H{"id": id, "task_id": taskID, "organization_id": org, "kind": kind, "period_start": start, "period_end": end, "revision": revision, "model": model, "round": round, "chunk": chunk, "attempt": attempt, "request_id": request, "status": status, "error": reason, "started_at": began, "finished_at": finished, "total_tokens": tokens, "actual_cost": cost})
	}
	adminRespond(c, pageResult(items, page, size, total), rows.Err())
}
func (h *Handler) OperationsTaskExecutions(c *gin.Context) { h.executionRows(c, true) }
func (h *Handler) OperationsExecutions(c *gin.Context)     { h.executionRows(c, false) }
func (h *Handler) OperationsSessions(c *gin.Context) {
	page, size, ok := queryPage(c)
	if !ok {
		return
	}
	rows, e := h.service.db.QueryContext(c.Request.Context(), `SELECT s.id,u.id,u.display_name,u.status,s.created_at,s.expires_at,s.revoked_at,s.last_seen_at,s.client_version,(s.auth_version=u.auth_version) FROM bi_sessions s JOIN bi_users u ON u.id=s.bi_user_id ORDER BY s.last_seen_at DESC,s.id LIMIT $1 OFFSET $2`, size, (page-1)*size)
	if e != nil {
		WriteError(c, e)
		return
	}
	defer func() { _ = rows.Close() }()
	items := []gin.H{}
	for rows.Next() {
		var id, user, name, status, version string
		var created, expires, last time.Time
		var revoked *time.Time
		var current bool
		if e = rows.Scan(&id, &user, &name, &status, &created, &expires, &revoked, &last, &version, &current); e != nil {
			WriteError(c, e)
			return
		}
		if revoked != nil || !expires.After(h.service.now()) || !current {
			status = "revoked"
		}
		items = append(items, gin.H{"id": id, "bi_user_id": user, "display_name": name, "status": status, "created_at": created, "expires_at": expires, "last_seen_at": last, "client_version": version})
	}
	if e = rows.Err(); e != nil {
		WriteError(c, e)
		return
	}
	var total int64
	e = h.service.db.QueryRowContext(c.Request.Context(), `SELECT COUNT(*) FROM bi_sessions`).Scan(&total)
	adminRespond(c, pageResult(items, page, size, total), e)
}
