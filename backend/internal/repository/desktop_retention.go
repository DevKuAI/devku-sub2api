package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Bounded batches protect active work and keep retention opt-in per organization.
func (r *desktopReportRepository) Cleanup(ctx context.Context, now time.Time) error {
	rows, err := r.db.QueryContext(ctx, `SELECT id,management_policy FROM desktop_organizations WHERE management_policy<>'{}'::jsonb`)
	if err != nil {
		return err
	}
	type policy struct {
		id    int64
		value service.DesktopManagementPolicy
	}
	policies := []policy{}
	for rows.Next() {
		var p policy
		var raw []byte
		if err = rows.Scan(&p.id, &raw); err != nil {
			break
		}
		if err = json.Unmarshal(raw, &p.value); err != nil {
			break
		}
		policies = append(policies, p)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return err
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	today := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 0, 0, 0, 0, loc)
	for _, p := range policies {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		var locked bool
		err = tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended('desktop-retention-' || $1::text,0))`, p.id).Scan(&locked)
		if err != nil || !locked {
			_ = tx.Rollback()
			if err != nil {
				return err
			}
			continue
		}
		var raw []byte
		if err = tx.QueryRowContext(ctx, `SELECT management_policy FROM desktop_organizations WHERE id=$1 FOR UPDATE`, p.id).Scan(&raw); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err = json.Unmarshal(raw, &p.value); err != nil {
			_ = tx.Rollback()
			return err
		}
		if p.value.ConversationRetentionDays > 0 {
			cutoff := today.AddDate(0, 0, -p.value.ConversationRetentionDays)
			_, err = tx.ExecContext(ctx, `DELETE FROM desktop_conversation_records WHERE id IN(SELECT c.id FROM desktop_conversation_records c WHERE c.organization_id=$1 AND c.received_at<$2 AND NOT EXISTS(SELECT 1 FROM desktop_daily_report_tasks t WHERE t.organization_id=$1 AND t.report_date=(c.stopped_at AT TIME ZONE 'Asia/Shanghai')::date AND (t.status IN ('pending','waiting','running') OR t.lease_until>NOW())) ORDER BY c.received_at,c.id LIMIT 1000)`, p.id, cutoff)
			if err == nil {
				_, err = tx.ExecContext(ctx, `UPDATE usage_logs SET request_body=NULL WHERE id IN(SELECT ul.id FROM usage_logs ul JOIN api_keys k ON k.id=ul.api_key_id WHERE k.desktop_analysis_organization_id=$1 AND ul.created_at<$2 AND ul.request_body IS NOT NULL ORDER BY ul.id LIMIT 1000)`, p.id, cutoff)
			}
			if err == nil {
				_, err = tx.ExecContext(ctx, `UPDATE desktop_daily_report_members SET sources='[]',work='{}' WHERE (task_id,member_id) IN(SELECT m.task_id,m.member_id FROM desktop_daily_report_members m JOIN desktop_daily_report_tasks t ON m.task_id=t.id WHERE t.organization_id=$1 AND t.report_date<$2::date AND t.status NOT IN ('pending','waiting','running') AND (t.lease_until IS NULL OR t.lease_until<=NOW()) AND (m.sources NOT IN ('[]'::jsonb,'null'::jsonb) OR m.work<>'{}'::jsonb) ORDER BY t.id,m.member_id LIMIT 1000)`, p.id, cutoff.Format("2006-01-02"))
			}
			if err == nil {
				_, err = tx.ExecContext(ctx, `UPDATE desktop_daily_report_tasks SET payload=payload-'summary_work' WHERE id IN(SELECT id FROM desktop_daily_report_tasks WHERE organization_id=$1 AND report_date<$2::date AND status NOT IN ('pending','waiting','running') AND (lease_until IS NULL OR lease_until<=NOW()) AND payload ? 'summary_work' ORDER BY id LIMIT 1000)`, p.id, cutoff.Format("2006-01-02"))
			}
		}
		if err == nil && p.value.ReportRetentionDays > 0 {
			_, err = tx.ExecContext(ctx, `DELETE FROM desktop_daily_report_tasks WHERE id IN(SELECT id FROM desktop_daily_report_tasks WHERE organization_id=$1 AND report_date<$2::date AND status NOT IN ('pending','waiting','running') AND (lease_until IS NULL OR lease_until<=NOW()) AND NOT EXISTS(SELECT 1 FROM desktop_period_report_tasks p WHERE p.organization_id=$1 AND desktop_daily_report_tasks.report_date>=p.period_start AND desktop_daily_report_tasks.report_date<p.period_end AND (p.status IN ('pending','waiting','running') OR p.lease_until>NOW()) AND COALESCE(p.configuration->>'captured','false')<>'true') ORDER BY report_date,id LIMIT 100)`, p.id, today.AddDate(0, 0, -p.value.ReportRetentionDays).Format("2006-01-02"))
		}
		if err == nil && p.value.ExecutionRetentionDays > 0 {
			_, err = tx.ExecContext(ctx, `DELETE FROM desktop_daily_report_executions WHERE id IN(SELECT e.id FROM desktop_daily_report_executions e LEFT JOIN desktop_daily_report_tasks t ON t.id=e.task_id WHERE e.organization_id=$1 AND e.report_date<$2::date AND (t.id IS NULL OR (t.status NOT IN ('pending','waiting','running') AND (t.lease_until IS NULL OR t.lease_until<=NOW()))) ORDER BY e.id LIMIT 1000)`, p.id, today.AddDate(0, 0, -p.value.ExecutionRetentionDays).Format("2006-01-02"))
		}
		if err == nil && p.value.ReportRetentionDays > 0 {
			_, err = tx.ExecContext(ctx, `DELETE FROM desktop_period_report_tasks WHERE id IN(SELECT id FROM desktop_period_report_tasks WHERE organization_id=$1 AND period_end<=$2::date AND status NOT IN ('pending','waiting','running') AND (lease_until IS NULL OR lease_until<=NOW()) ORDER BY period_end,id LIMIT 100)`, p.id, today.AddDate(0, 0, -p.value.ReportRetentionDays).Format("2006-01-02"))
		}
		if err == nil && p.value.ExecutionRetentionDays > 0 {
			_, err = tx.ExecContext(ctx, `DELETE FROM desktop_period_report_executions WHERE id IN(SELECT e.id FROM desktop_period_report_executions e LEFT JOIN desktop_period_report_tasks p ON p.id=e.task_id WHERE e.organization_id=$1 AND e.started_at<$2 AND (p.id IS NULL OR (p.status NOT IN ('pending','waiting','running') AND (p.lease_until IS NULL OR p.lease_until<=NOW()))) ORDER BY e.id LIMIT 1000)`, p.id, today.AddDate(0, 0, -p.value.ExecutionRetentionDays))
		}
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
