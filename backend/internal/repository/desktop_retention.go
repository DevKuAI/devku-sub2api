package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

// Cleanup rotates a durable enterprise cursor and bounds each batch and lock wait.
func (r *desktopReportRepository) Cleanup(parent context.Context, now time.Time) error {
	ctx, cancel := context.WithTimeout(parent, 45*time.Second)
	defer cancel()
	token := uuid.NewString()
	var cursor int64
	err := r.db.QueryRowContext(ctx, `INSERT INTO desktop_maintenance_progress(name,lease_token,lease_until) VALUES('retention',$1,NOW()+INTERVAL '45 seconds')
 ON CONFLICT(name) DO UPDATE SET lease_token=EXCLUDED.lease_token,lease_until=EXCLUDED.lease_until,updated_at=NOW()
 WHERE desktop_maintenance_progress.lease_until IS NULL OR desktop_maintenance_progress.lease_until<=NOW() RETURNING organization_id`, token).Scan(&cursor)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() {
		releaseCtx, releaseCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer releaseCancel()
		_, _ = r.db.ExecContext(releaseCtx, `UPDATE desktop_maintenance_progress SET lease_token=NULL,lease_until=NULL WHERE name='retention' AND lease_token=$1`, token)
	}()

	ids, err := r.retentionOrganizations(ctx, cursor)
	if err != nil {
		return err
	}
	if len(ids) == 0 && cursor > 0 {
		if err = r.advanceRetentionCursor(ctx, token, 0, true); err != nil {
			return err
		}
		ids, err = r.retentionOrganizations(ctx, 0)
		if err != nil {
			return err
		}
	}
	if len(ids) == 0 {
		return nil
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	local := now.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	failures := []error{}
	for _, id := range ids {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		started := time.Now()
		var removed int64
		batches := 0
		more := false
		for batches < 3 {
			batchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			n, e := r.cleanupOrganizationBatch(batchCtx, id, today)
			cancel()
			batches++
			removed += n
			more = n > 0
			if e != nil {
				failures = append(failures, e)
				slog.Error("desktop retention enterprise failed", "organization_id", id, "error", e)
				break
			}
			if !more {
				break
			}
		}
		slog.Info("desktop retention batch", "organization_id", id, "affected_rows", removed, "batches", batches, "may_have_backlog", more, "duration_ms", time.Since(started).Milliseconds())
		if err = r.advanceRetentionCursor(ctx, token, id, false); err != nil {
			return err
		}
	}
	return errors.Join(failures...)
}

func (r *desktopReportRepository) advanceRetentionCursor(ctx context.Context, token string, id int64, reset bool) error {
	result, err := r.db.ExecContext(ctx, `UPDATE desktop_maintenance_progress SET organization_id=CASE WHEN $3 THEN 0 ELSE GREATEST(organization_id,$1) END,updated_at=NOW() WHERE name='retention' AND lease_token=$2 AND lease_until>NOW()`, id, token, reset)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("desktop retention scan lease lost")
	}
	return nil
}

func (r *desktopReportRepository) retentionOrganizations(ctx context.Context, cursor int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM desktop_organizations WHERE id>$1 AND management_policy<>'{}'::jsonb ORDER BY id LIMIT 16`, cursor)
	if err != nil {
		return nil, err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *desktopReportRepository) cleanupOrganizationBatch(ctx context.Context, id int64, today time.Time) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SET LOCAL lock_timeout='500ms'`); err != nil {
		return 0, err
	}
	var affected int64
	exec := func(ctx context.Context, query string, args ...any) error {
		result, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		affected += n
		return err
	}
	var policy service.DesktopManagementPolicy
	var locked bool
	err = tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended('desktop-retention-' || $1::text,0))`, id).Scan(&locked)
	if err != nil || !locked {
		_ = tx.Rollback()
		if err != nil {
			return 0, err
		}
		return 0, nil
	}
	var raw []byte
	if err = tx.QueryRowContext(ctx, `SELECT management_policy FROM desktop_organizations WHERE id=$1 FOR UPDATE`, id).Scan(&raw); err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if err = json.Unmarshal(raw, &policy); err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if policy.ConversationRetentionDays > 0 {
		cutoff := today.AddDate(0, 0, -policy.ConversationRetentionDays)
		err = exec(ctx, `DELETE FROM desktop_conversation_records WHERE id IN(SELECT c.id FROM desktop_conversation_records c WHERE c.organization_id=$1 AND c.received_at<$2 AND NOT EXISTS(SELECT 1 FROM desktop_daily_report_tasks t WHERE t.organization_id=$1 AND t.report_date=(c.stopped_at AT TIME ZONE 'Asia/Shanghai')::date AND (t.status IN ('pending','waiting','running') OR t.lease_until>NOW())) ORDER BY c.received_at,c.id LIMIT 1000)`, id, cutoff)
		if err == nil {
			err = exec(ctx, `UPDATE usage_logs SET request_body=NULL WHERE id IN(SELECT ul.id FROM usage_logs ul JOIN api_keys k ON k.id=ul.api_key_id WHERE k.desktop_analysis_organization_id=$1 AND ul.created_at<$2 AND ul.request_body IS NOT NULL ORDER BY ul.id LIMIT 1000)`, id, cutoff)
		}
		if err == nil {
			err = exec(ctx, `UPDATE desktop_daily_report_members SET sources='[]',work='{}' WHERE (task_id,member_id) IN(SELECT m.task_id,m.member_id FROM desktop_daily_report_members m JOIN desktop_daily_report_tasks t ON m.task_id=t.id WHERE t.organization_id=$1 AND t.report_date<$2::date AND t.status NOT IN ('pending','waiting','running') AND (t.lease_until IS NULL OR t.lease_until<=NOW()) AND (m.sources NOT IN ('[]'::jsonb,'null'::jsonb) OR m.work<>'{}'::jsonb) ORDER BY t.id,m.member_id LIMIT 1000)`, id, cutoff.Format("2006-01-02"))
		}
		if err == nil {
			err = exec(ctx, `UPDATE desktop_daily_report_tasks SET payload=payload-'summary_work' WHERE id IN(SELECT id FROM desktop_daily_report_tasks WHERE organization_id=$1 AND report_date<$2::date AND status NOT IN ('pending','waiting','running') AND (lease_until IS NULL OR lease_until<=NOW()) AND payload ? 'summary_work' ORDER BY id LIMIT 1000)`, id, cutoff.Format("2006-01-02"))
		}
	}
	if err == nil && policy.ReportRetentionDays > 0 {
		err = exec(ctx, `DELETE FROM desktop_daily_report_tasks WHERE id IN(SELECT id FROM desktop_daily_report_tasks WHERE organization_id=$1 AND report_date<$2::date AND status NOT IN ('pending','waiting','running') AND (lease_until IS NULL OR lease_until<=NOW()) AND NOT EXISTS(SELECT 1 FROM desktop_period_report_tasks p WHERE p.organization_id=$1 AND desktop_daily_report_tasks.report_date>=p.period_start AND desktop_daily_report_tasks.report_date<p.period_end AND (p.status IN ('pending','waiting','running') OR p.lease_until>NOW()) AND COALESCE(p.configuration->>'captured','false')<>'true') ORDER BY report_date,id LIMIT 100)`, id, today.AddDate(0, 0, -policy.ReportRetentionDays).Format("2006-01-02"))
	}
	if err == nil && policy.ExecutionRetentionDays > 0 {
		err = exec(ctx, `DELETE FROM desktop_daily_report_executions WHERE id IN(SELECT e.id FROM desktop_daily_report_executions e LEFT JOIN desktop_daily_report_tasks t ON t.id=e.task_id WHERE e.organization_id=$1 AND e.report_date<$2::date AND (t.id IS NULL OR (t.status NOT IN ('pending','waiting','running') AND (t.lease_until IS NULL OR t.lease_until<=NOW()))) ORDER BY e.id LIMIT 1000)`, id, today.AddDate(0, 0, -policy.ExecutionRetentionDays).Format("2006-01-02"))
	}
	if err == nil && policy.ReportRetentionDays > 0 {
		err = exec(ctx, `DELETE FROM desktop_period_report_tasks WHERE id IN(SELECT id FROM desktop_period_report_tasks WHERE organization_id=$1 AND period_end<=$2::date AND status NOT IN ('pending','waiting','running') AND (lease_until IS NULL OR lease_until<=NOW()) ORDER BY period_end,id LIMIT 100)`, id, today.AddDate(0, 0, -policy.ReportRetentionDays).Format("2006-01-02"))
	}
	if err == nil && policy.ExecutionRetentionDays > 0 {
		err = exec(ctx, `DELETE FROM desktop_period_report_executions WHERE id IN(SELECT e.id FROM desktop_period_report_executions e LEFT JOIN desktop_period_report_tasks p ON p.id=e.task_id WHERE e.organization_id=$1 AND e.started_at<$2 AND (p.id IS NULL OR (p.status NOT IN ('pending','waiting','running') AND (p.lease_until IS NULL OR p.lease_until<=NOW()))) ORDER BY e.id LIMIT 1000)`, id, today.AddDate(0, 0, -policy.ExecutionRetentionDays))
	}
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return affected, nil
}
