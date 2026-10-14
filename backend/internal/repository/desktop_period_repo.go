package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

const periodChanged = `COALESCE((p.configuration->>'captured')::boolean,FALSE) AND (EXISTS(SELECT 1 FROM jsonb_array_elements(p.source_versions) v LEFT JOIN desktop_daily_report_tasks d ON d.organization_id=p.organization_id AND d.report_date=(v->>'date')::date WHERE d.id IS NULL OR d.revision<>(v->>'revision')::int OR (d.summary->>'generated_at')::timestamptz IS DISTINCT FROM (v->>'generated_at')::timestamptz OR encode(sha256(convert_to(COALESCE(d.summary->>'content',''),'UTF8')),'hex') IS DISTINCT FROM v->>'content_hash') OR EXISTS(SELECT 1 FROM desktop_daily_report_tasks d WHERE d.organization_id=p.organization_id AND d.report_date>=p.period_start AND d.report_date<p.period_end AND d.summary->>'status' IN ('completed','partial') AND COALESCE(d.summary->>'content','')<>'' AND NOT EXISTS(SELECT 1 FROM jsonb_array_elements(p.source_versions) v WHERE (v->>'date')::date=d.report_date)))`

const periodMetadataColumns = `p.public_id,p.kind,p.period_start::text,p.period_end::text,p.revision,p.status,p.reason,p.generated_at,p.coverage,COALESCE(p.configuration->>'model',''),` + periodChanged

func scanPeriodMetadata(row interface{ Scan(...any) error }) (service.DesktopPeriodMetadata, error) {
	var m service.DesktopPeriodMetadata
	var start, end string
	var coverage []byte
	e := row.Scan(&m.ID, &m.Kind, &start, &end, &m.Revision, &m.Status, &m.Reason, &m.GeneratedAt, &coverage, &m.Model, &m.SourceChanged)
	if e != nil {
		return m, e
	}
	m.Range = service.DesktopNativeRange{Start: start, Timezone: "Asia/Shanghai"}
	date, e := time.Parse("2006-01-02", end)
	if e != nil {
		return m, e
	}
	m.Range.End = date.AddDate(0, 0, -1).Format("2006-01-02")
	e = json.Unmarshal(coverage, &m.Coverage)
	return m, e
}
func periodTaskColumns(content bool) string {
	private := `NULL::text,'[]'::jsonb,'{}'::jsonb,'{}'::jsonb`
	if content {
		private = `p.content,p.sources,p.work,p.configuration`
	}
	return `p.id,p.organization_id,` + periodMetadataColumns + `,COALESCE(p.lease_token,''),` + private
}
func scanPeriodTask(row interface{ Scan(...any) error }) (*service.DesktopPeriodTask, error) {
	t := &service.DesktopPeriodTask{}
	var start, end string
	var coverage, sources, work, configuration []byte
	e := row.Scan(&t.InternalID, &t.OrganizationID, &t.ID, &t.Kind, &start, &end, &t.Revision, &t.Status, &t.Reason, &t.GeneratedAt, &coverage, &t.Model, &t.SourceChanged, &t.LeaseToken, &t.Content, &sources, &work, &configuration)
	if e != nil {
		return nil, e
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	t.Start, e = time.ParseInLocation("2006-01-02", start, loc)
	if e != nil {
		return nil, e
	}
	t.End, e = time.ParseInLocation("2006-01-02", end, loc)
	if e != nil {
		return nil, e
	}
	t.Range = service.DesktopNativeRange{Start: start, End: t.End.AddDate(0, 0, -1).Format("2006-01-02"), Timezone: "Asia/Shanghai"}
	for _, field := range []struct {
		raw    []byte
		target any
	}{{coverage, &t.Coverage}, {sources, &t.Sources}, {work, &t.Work}, {configuration, &t.Configuration}} {
		if e = json.Unmarshal(field.raw, field.target); e != nil {
			return nil, e
		}
	}
	return t, nil
}
func (r *desktopReportRepository) ReadPeriod(ctx context.Context, org int64, id string, content bool) (*service.DesktopPeriodTask, error) {
	t, e := scanPeriodTask(r.db.QueryRowContext(ctx, `SELECT `+periodTaskColumns(content)+` FROM desktop_period_report_tasks p WHERE p.organization_id=$1 AND p.public_id=$2`, org, id))
	if errors.Is(e, sql.ErrNoRows) {
		return nil, service.ErrDesktopMemberNotFound
	}
	return t, e
}
func (r *desktopReportRepository) ListPeriods(ctx context.Context, org int64, kind, start string, page, size int, readableOnly ...bool) ([]service.DesktopPeriodMetadata, int64, error) {
	where := ` WHERE p.organization_id=$1 AND ($2='' OR p.kind=$2) AND ($3='' OR p.period_start::text=$3)`
	if len(readableOnly) > 0 && readableOnly[0] {
		where += ` AND p.status IN ('completed','partial') AND p.content IS NOT NULL AND NOT EXISTS(SELECT 1 FROM desktop_period_report_tasks newer WHERE newer.organization_id=p.organization_id AND newer.kind=p.kind AND newer.period_start=p.period_start AND newer.revision>p.revision AND newer.status IN ('completed','partial') AND newer.content IS NOT NULL)`
	}
	var total int64
	e := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM desktop_period_report_tasks p`+where, org, kind, start).Scan(&total)
	if e != nil {
		return nil, 0, e
	}
	rows, e := r.db.QueryContext(ctx, `SELECT `+periodMetadataColumns+` FROM desktop_period_report_tasks p`+where+` ORDER BY p.period_start DESC,p.revision DESC,p.id DESC LIMIT $4 OFFSET $5`, org, kind, start, size, (page-1)*size)
	if e != nil {
		return nil, 0, e
	}
	defer func() { _ = rows.Close() }()
	items := []service.DesktopPeriodMetadata{}
	for rows.Next() {
		m, e := scanPeriodMetadata(rows)
		if e != nil {
			return nil, 0, e
		}
		items = append(items, m)
	}
	return items, total, rows.Err()
}
func (r *desktopReportRepository) EnqueuePeriod(ctx context.Context, org int64, kind string, start, end time.Time, mode string) (*service.DesktopPeriodTask, error) {
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("desktop-period:%d:%s:%s", org, kind, start.Format("2006-01-02"))); e != nil {
		return nil, e
	}
	previous, e := scanPeriodTask(tx.QueryRowContext(ctx, `SELECT `+periodTaskColumns(true)+` FROM desktop_period_report_tasks p WHERE p.organization_id=$1 AND p.kind=$2 AND p.period_start=$3::date ORDER BY p.revision DESC LIMIT 1 FOR UPDATE OF p`, org, kind, start.Format("2006-01-02")))
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return nil, e
	}
	revision := 1
	if previous != nil {
		var leased bool
		if e = tx.QueryRowContext(ctx, `SELECT COALESCE(lease_until>NOW(),FALSE) FROM desktop_period_report_tasks WHERE id=$1`, previous.InternalID).Scan(&leased); e != nil {
			return nil, e
		}
		if mode == "generate" {
			if e = tx.Commit(); e != nil {
				return nil, e
			}
			return previous, nil
		}
		if leased || (mode == "regenerate" && (previous.Status == "pending" || previous.Status == "waiting" || previous.Status == "running")) {
			return nil, service.ErrDesktopRotationConflict
		}
		if mode == "retry" {
			if previous.Content != nil {
				if e = tx.Commit(); e != nil {
					return nil, e
				}
				return previous, nil
			}
			previous.Work.Attempts = 0
			previous.Work.RetryAt = time.Time{}
			raw, _ := json.Marshal(previous.Work)
			if _, e = tx.ExecContext(ctx, `UPDATE desktop_period_report_tasks SET status='pending',reason='',work=$2,next_run_at=NOW(),lease_token=NULL,lease_until=NULL,updated_at=NOW() WHERE id=$1`, previous.InternalID, string(raw)); e != nil {
				return nil, e
			}
			if e = tx.Commit(); e != nil {
				return nil, e
			}
			return r.ReadPeriod(ctx, org, previous.ID, true)
		}
		revision = previous.Revision + 1
	}
	id := "dpr_" + uuid.NewString()
	emptyCoverage, _ := json.Marshal(service.DesktopPeriodCoverage{Available: []string{}, Partial: []string{}, Missing: []string{}, NoRecords: []string{}, Reasons: map[string]string{}})
	_, e = tx.ExecContext(ctx, `INSERT INTO desktop_period_report_tasks(public_id,organization_id,kind,period_start,period_end,revision,coverage) VALUES($1,$2,$3,$4::date,$5::date,$6,$7)`, id, org, kind, start.Format("2006-01-02"), end.Format("2006-01-02"), revision, string(emptyCoverage))
	if e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return r.ReadPeriod(ctx, org, id, true)
}
func (r *desktopReportRepository) PeriodSources(ctx context.Context, org int64, start, end time.Time) ([]service.DesktopPeriodSource, service.DesktopPeriodCoverage, error) {
	coverage := service.DesktopPeriodCoverage{Available: []string{}, Partial: []string{}, Missing: []string{}, NoRecords: []string{}, Reasons: map[string]string{}}
	sources := []service.DesktopPeriodSource{}
	// Only enterprise summary JSON is selected. Employee text and conversation records are not inputs.
	rows, e := r.db.QueryContext(ctx, `SELECT report_date::text,revision,status,reason,summary FROM desktop_daily_report_tasks WHERE organization_id=$1 AND report_date >= $2::date AND report_date < $3::date ORDER BY report_date`, org, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if e != nil {
		return nil, coverage, e
	}
	defer func() { _ = rows.Close() }()
	byDate := map[string]service.DesktopPeriodSource{}
	reasons := map[string]string{}
	for rows.Next() {
		var date, status, reason string
		var revision int
		var raw []byte
		if e = rows.Scan(&date, &revision, &status, &reason, &raw); e != nil {
			return nil, coverage, e
		}
		var summary service.DesktopGeneratedReport
		if e = json.Unmarshal(raw, &summary); e != nil {
			return nil, coverage, e
		}
		source := service.DesktopPeriodSource{Date: date, Revision: revision, Status: status, GeneratedAt: summary.GeneratedAt}
		if summary.Content != "" && (summary.Status == "completed" || summary.Status == "partial") {
			source.Content = summary.Content
			source.Hash = fmt.Sprintf("%x", sha256.Sum256([]byte(summary.Content)))
			source.Status = "completed"
			if status != "completed" || summary.Status == "partial" {
				source.Status = "partial"
			}
		}
		byDate[date] = source
		reasons[date] = reason
	}
	if e = rows.Err(); e != nil {
		return nil, coverage, e
	}
	for day := start; day.Before(end); day = day.AddDate(0, 0, 1) {
		date := day.Format("2006-01-02")
		source, exists := byDate[date]
		if exists && source.Content != "" {
			sources = append(sources, source)
			coverage.Available = append(coverage.Available, date)
			if source.Status == "partial" {
				coverage.Partial = append(coverage.Partial, date)
			}
		} else if exists && source.Status == "skipped" && reasons[date] == "no_records" {
			coverage.NoRecords = append(coverage.NoRecords, date)
			coverage.Reasons[date] = "no_records"
		} else {
			coverage.Missing = append(coverage.Missing, date)
			reason := "not_generated"
			if exists {
				reason = source.Status
				if reasons[date] != "" {
					reason = reasons[date]
				}
			}
			coverage.Reasons[date] = reason
		}
	}
	return sources, coverage, nil
}
func (r *desktopReportRepository) ClaimPeriod(ctx context.Context) (*service.DesktopPeriodTask, error) {
	token := uuid.NewString()
	t, e := scanPeriodTask(r.db.QueryRowContext(ctx, `UPDATE desktop_period_report_tasks p SET lease_token=$1,lease_until=NOW()+INTERVAL '3 minutes',status='running',updated_at=NOW() WHERE p.id=(SELECT id FROM desktop_period_report_tasks WHERE status IN ('pending','waiting','running') AND next_run_at<=NOW() AND (lease_until IS NULL OR lease_until<=NOW()) ORDER BY next_run_at,id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING `+periodTaskColumns(true), token))
	if errors.Is(e, sql.ErrNoRows) {
		return nil, nil
	}
	return t, e
}
func (r *desktopReportRepository) SavePeriod(ctx context.Context, t *service.DesktopPeriodTask, next time.Time, release bool) error {
	if t.Sources == nil {
		t.Sources = []service.DesktopPeriodSource{}
	}
	sources, _ := json.Marshal(t.Sources)
	coverage, _ := json.Marshal(t.Coverage)
	work, _ := json.Marshal(t.Work)
	config, _ := json.Marshal(t.Configuration)
	versions := make([]service.DesktopPeriodSource, len(t.Sources))
	copy(versions, t.Sources)
	for i := range versions {
		versions[i].Content = ""
	}
	meta, _ := json.Marshal(versions)
	result, e := r.db.ExecContext(ctx, `UPDATE desktop_period_report_tasks SET status=$3,reason=$4,sources=$5,source_versions=$6,coverage=$7,work=$8,configuration=$9,content=$10,generated_at=$11,next_run_at=$12,lease_token=CASE WHEN $13 THEN NULL ELSE lease_token END,lease_until=CASE WHEN $13 THEN NULL ELSE lease_until END,updated_at=NOW() WHERE id=$1 AND lease_token=$2 AND lease_until>NOW()`, t.InternalID, t.LeaseToken, t.Status, t.Reason, string(sources), string(meta), string(coverage), string(work), string(config), t.Content, t.GeneratedAt, next, release)
	return checkReportLease(result, e)
}
func (r *desktopReportRepository) RenewPeriod(ctx context.Context, t *service.DesktopPeriodTask) error {
	result, e := r.db.ExecContext(ctx, `UPDATE desktop_period_report_tasks SET lease_until=NOW()+INTERVAL '3 minutes' WHERE id=$1 AND lease_token=$2 AND lease_until>NOW()`, t.InternalID, t.LeaseToken)
	return checkReportLease(result, e)
}
func (r *desktopReportRepository) DuePeriodOrganizations(ctx context.Context, kind string, start time.Time) ([]string, error) {
	rows, e := r.db.QueryContext(ctx, `SELECT o.public_id FROM desktop_organizations o WHERE o.deleted_at IS NULL AND o.status='active' AND o.conversation_summary_enabled AND (($1='week' AND o.weekly_summary_enabled) OR ($1='month' AND o.monthly_summary_enabled)) AND NOT EXISTS(SELECT 1 FROM desktop_period_report_tasks p WHERE p.organization_id=o.id AND p.kind=$1 AND p.period_start=$2::date)`, kind, start.Format("2006-01-02"))
	if e != nil {
		return nil, e
	}
	defer func() { _ = rows.Close() }()
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return nil, e
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func (r *desktopReportRepository) BeginPeriodExecution(ctx context.Context, t *service.DesktopPeriodTask, e *service.DesktopReportExecution) error {
	err := r.db.QueryRowContext(ctx, `INSERT INTO desktop_period_report_executions(task_id,organization_id,kind,period_start,period_end,revision,model,round,chunk,attempt,request_id) SELECT id,organization_id,kind,period_start,period_end,revision,$3,$4,$5,$6,$7 FROM desktop_period_report_tasks WHERE id=$1 AND lease_token=$2 AND lease_until>NOW() RETURNING id,started_at`, t.InternalID, t.LeaseToken, e.Model, e.Round, e.Chunk, e.Attempt, e.RequestID).Scan(&e.ID, &e.StartedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrDesktopReportLeaseLost
	}
	return err
}
func (r *desktopReportRepository) FinishPeriodExecution(ctx context.Context, e *service.DesktopReportExecution) error {
	result, err := r.db.ExecContext(ctx, `UPDATE desktop_period_report_executions SET status=$2,error=$3,finished_at=$4 WHERE id=$1 AND request_id=$5 AND status='running'`, e.ID, e.Status, e.Error, e.FinishedAt, e.RequestID)
	return checkReportLease(result, err)
}
func (r *desktopReportRepository) RecordPeriodExecutionUsage(ctx context.Context, id int64, usage *service.UsageLog) error {
	_, e := r.db.ExecContext(ctx, `UPDATE desktop_period_report_executions SET total_tokens=$2,actual_cost=$3 WHERE id=$1`, id, usage.InputTokens+usage.OutputTokens+usage.CacheCreationTokens+usage.CacheReadTokens, usage.ActualCost)
	return e
}

func (r *desktopReportRepository) ListPeriodSources(ctx context.Context, org int64, id string) ([]service.DesktopPeriodSource, error) {
	rows, e := r.db.QueryContext(ctx, `SELECT v,CASE WHEN d.id IS NULL THEN 'cleared' WHEN d.revision<>(v->>'revision')::int OR (d.summary->>'generated_at')::timestamptz IS DISTINCT FROM (v->>'generated_at')::timestamptz OR encode(sha256(convert_to(COALESCE(d.summary->>'content',''),'UTF8')),'hex') IS DISTINCT FROM v->>'content_hash' THEN 'changed' ELSE 'available' END FROM desktop_period_report_tasks p CROSS JOIN LATERAL jsonb_array_elements(p.source_versions) v LEFT JOIN desktop_daily_report_tasks d ON d.organization_id=p.organization_id AND d.report_date=(v->>'date')::date WHERE p.organization_id=$1 AND p.public_id=$2 ORDER BY v->>'date'`, org, id)
	if e != nil {
		return nil, e
	}
	defer func() { _ = rows.Close() }()
	out := []service.DesktopPeriodSource{}
	for rows.Next() {
		var raw []byte
		var state string
		if e = rows.Scan(&raw, &state); e != nil {
			return nil, e
		}
		var source service.DesktopPeriodSource
		if e = json.Unmarshal(raw, &source); e != nil {
			return nil, e
		}
		source.Content = ""
		source.CurrentStatus = state
		out = append(out, source)
	}
	return out, rows.Err()
}
func (r *desktopReportRepository) ReadPeriodSource(ctx context.Context, org int64, id, date string) (service.DesktopPeriodSource, error) {
	var raw []byte
	e := r.db.QueryRowContext(ctx, `SELECT v FROM desktop_period_report_tasks p CROSS JOIN LATERAL jsonb_array_elements(p.sources) v WHERE p.organization_id=$1 AND p.public_id=$2 AND v->>'date'=$3`, org, id, date).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return service.DesktopPeriodSource{}, service.ErrDesktopMemberNotFound
	}
	var source service.DesktopPeriodSource
	if e != nil {
		return source, e
	}
	e = json.Unmarshal(raw, &source)
	return source, e
}
