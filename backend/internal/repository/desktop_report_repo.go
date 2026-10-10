package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/apikey"
	"github.com/Wei-Shaw/sub2api/ent/desktoporganization"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

type desktopReportRepository struct {
	db     *sql.DB
	client *dbent.Client
}

func NewDesktopReportRepository(db *sql.DB, client *dbent.Client) service.DesktopReportRepository {
	return &desktopReportRepository{db: db, client: client}
}

func reportBounds(date string) (time.Time, time.Time, error) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	start, err := time.ParseInLocation("2006-01-02", date, loc)
	return start, start.AddDate(0, 0, 1), err
}

func (r *desktopReportRepository) Day(ctx context.Context, org int64, date string) ([]service.DesktopReportMember, error) {
	start, end, err := reportBounds(date)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT m.public_id,m.name,m.deleted_at IS NOT NULL,
 COUNT(c.id), COALESCE(jsonb_agg(c.record_id::text ORDER BY c.stopped_at,c.id) FILTER(WHERE c.id IS NOT NULL),'[]'::jsonb)
 FROM desktop_members m LEFT JOIN desktop_conversation_records c ON c.member_id=m.id AND c.organization_id=$1 AND c.stopped_at >= $2 AND c.stopped_at < $3
 WHERE m.organization_id=$1 AND (m.deleted_at IS NULL OR c.id IS NOT NULL)
 GROUP BY m.id ORDER BY m.name,m.id`, org, start, end)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	members := make([]service.DesktopReportMember, 0)
	for rows.Next() {
		var m service.DesktopReportMember
		var ids []byte
		if err = rows.Scan(&m.MemberID, &m.Name, &m.Deleted, &m.RecordCount, &ids); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(ids, &m.SourceIDs); err != nil {
			return nil, err
		}
		m.Status = "no_records"
		if m.RecordCount > 0 {
			m.Status = "pending"
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *desktopReportRepository) Snapshot(ctx context.Context, org int64, date string) ([]service.DesktopReportSnapshot, error) {
	start, end, err := reportBounds(date)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT m.public_id,m.name,m.deleted_at IS NOT NULL,c.record_id::text,c.client,c.source_session_id,c.stopped_at,c.prompts,c.response,c.capture_status
 FROM desktop_conversation_records c JOIN desktop_members m ON m.id=c.member_id
 WHERE c.organization_id=$1 AND c.stopped_at >= $2 AND c.stopped_at < $3 ORDER BY m.id,c.stopped_at,c.id`, org, start, end)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]service.DesktopReportSnapshot, 0)
	index := map[string]int{}
	for rows.Next() {
		var id, name string
		var deleted bool
		var source service.DesktopReportSource
		var prompts, response []byte
		if err = rows.Scan(&id, &name, &deleted, &source.RecordID, &source.Client, &source.SessionID, &source.StoppedAt, &prompts, &response, &source.CaptureStatus); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(prompts, &source.Prompts); err != nil {
			return nil, err
		}
		if len(response) > 0 && string(response) != "null" {
			if err = json.Unmarshal(response, &source.Response); err != nil {
				return nil, err
			}
		}
		i, ok := index[id]
		if !ok {
			i = len(result)
			index[id] = i
			result = append(result, service.DesktopReportSnapshot{Member: service.DesktopReportMember{MemberID: id, Name: name, Deleted: deleted, Status: "pending", SourceIDs: []string{}}, Sources: []service.DesktopReportSource{}})
		}
		result[i].Sources = append(result[i].Sources, source)
		result[i].Member.RecordCount++
		result[i].Member.SourceIDs = append(result[i].Member.SourceIDs, source.RecordID)
	}
	return result, rows.Err()
}

type reportScanner interface{ Scan(...any) error }

func scanReportTask(row reportScanner) (*service.DesktopReportTask, error) {
	var t service.DesktopReportTask
	var payload []byte
	err := row.Scan(&t.ID, &t.OrganizationID, &t.Date, &t.Status, &t.Reason, &t.Revision, &t.LeaseToken, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(payload, &t.Payload); err != nil {
		return nil, err
	}
	return &t, nil
}

const reportTaskColumns = `id,organization_id,report_date::text,status,reason,revision,COALESCE(lease_token,''),payload`

func (r *desktopReportRepository) Task(ctx context.Context, org int64, date string) (*service.DesktopReportTask, error) {
	return scanReportTask(r.db.QueryRowContext(ctx, `SELECT `+reportTaskColumns+` FROM desktop_daily_report_tasks WHERE organization_id=$1 AND report_date=$2::date`, org, date))
}

func (r *desktopReportRepository) Enqueue(ctx context.Context, org int64, date, mode string) (*service.DesktopReportTask, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `INSERT INTO desktop_daily_report_tasks(organization_id,report_date) VALUES($1,$2::date) ON CONFLICT DO NOTHING`, org, date)
	if err != nil {
		return nil, err
	}
	var leased bool
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(lease_until > NOW(),FALSE) FROM desktop_daily_report_tasks WHERE organization_id=$1 AND report_date=$2::date FOR UPDATE`, org, date).Scan(&leased); err != nil {
		return nil, err
	}
	if leased {
		return nil, service.ErrDesktopRotationConflict
	}
	t, err := scanReportTask(tx.QueryRowContext(ctx, `SELECT `+reportTaskColumns+` FROM desktop_daily_report_tasks WHERE organization_id=$1 AND report_date=$2::date`, org, date))
	if err != nil {
		return nil, err
	}
	if mode == "automatic" && t.Status != "pending" {
		return t, tx.Commit()
	}
	if mode == "generate" && (t.Status == "completed" || t.Status == "partial") {
		return t, tx.Commit()
	}
	if mode == "regenerate" {
		for i := range t.Payload.Snapshots {
			t.Payload.Snapshots[i].Sources = nil
			t.Payload.Snapshots[i].Work = service.DesktopReportWork{}
			t.Payload.Snapshots[i].Member.Status = "pending"
			t.Payload.Snapshots[i].Member.Error = ""
		}
		t.Payload.Model = ""
		t.Payload.SummaryWork = service.DesktopReportWork{}
		t.Payload.SummaryFingerprint = ""
		t.Payload.Summary.Status = "pending"
		t.Revision++
	}
	if mode == "retry" {
		for i := range t.Payload.Snapshots {
			if t.Payload.Snapshots[i].Member.Status != "completed" {
				t.Payload.Snapshots[i].Work.Attempts = 0
				t.Payload.Snapshots[i].Work.RetryAt = time.Time{}
				t.Payload.Snapshots[i].Member.Status = "pending"
				t.Payload.Snapshots[i].Member.Error = ""
			}
		}
		t.Payload.SummaryWork.Attempts = 0
		t.Payload.SummaryWork.RetryAt = time.Time{}
		if t.Payload.Summary.Status == "failed" {
			t.Payload.Summary.Status = "pending"
		}
	}
	data, err := json.Marshal(t.Payload)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE desktop_daily_report_tasks SET status='pending',reason='',payload=$3::jsonb,revision=$4,next_run_at=NOW(),lease_token=NULL,lease_until=NULL,updated_at=NOW() WHERE organization_id=$1 AND report_date=$2::date`, org, date, string(data), t.Revision)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.Task(ctx, org, date)
}
func (r *desktopReportRepository) DueOrganizations(ctx context.Context, date string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT public_id FROM desktop_organizations WHERE deleted_at IS NULL AND status='active' AND conversation_summary_enabled AND summary_enabled_at IS NOT NULL AND (summary_enabled_at AT TIME ZONE 'Asia/Shanghai')::date <= $1::date
 AND NOT EXISTS(SELECT 1 FROM desktop_daily_report_tasks t WHERE t.organization_id=desktop_organizations.id AND t.report_date=$1::date)`, date)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func (r *desktopReportRepository) Claim(ctx context.Context) (*service.DesktopReportTask, error) {
	token := uuid.NewString()
	return scanReportTask(r.db.QueryRowContext(ctx, `UPDATE desktop_daily_report_tasks SET lease_token=$1,lease_until=NOW()+INTERVAL '3 minutes',status='running',updated_at=NOW()
 WHERE id=(SELECT id FROM desktop_daily_report_tasks WHERE status IN ('pending','waiting','running') AND next_run_at<=NOW() AND (lease_until IS NULL OR lease_until<=NOW()) ORDER BY next_run_at,id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING `+reportTaskColumns, token))
}
func (r *desktopReportRepository) Save(ctx context.Context, t *service.DesktopReportTask, next time.Time, release bool) error {
	data, err := json.Marshal(t.Payload)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `UPDATE desktop_daily_report_tasks SET status=$3,reason=$4,payload=$5::jsonb,next_run_at=$6,updated_at=NOW(),lease_token=CASE WHEN $7 THEN NULL ELSE lease_token END,lease_until=CASE WHEN $7 THEN NULL ELSE lease_until END WHERE id=$1 AND lease_token=$2 AND lease_until>NOW()`, t.ID, t.LeaseToken, t.Status, t.Reason, string(data), next, release)
	return checkReportLease(result, err)
}
func (r *desktopReportRepository) Renew(ctx context.Context, t *service.DesktopReportTask) error {
	result, err := r.db.ExecContext(ctx, `UPDATE desktop_daily_report_tasks SET lease_until=NOW()+INTERVAL '3 minutes' WHERE id=$1 AND lease_token=$2 AND lease_until>NOW()`, t.ID, t.LeaseToken)
	return checkReportLease(result, err)
}
func checkReportLease(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return service.ErrDesktopReportLeaseLost
	}
	return nil
}
func (r *desktopReportRepository) OrganizationPublicID(ctx context.Context, id int64) (string, error) {
	var value string
	err := r.db.QueryRowContext(ctx, `SELECT public_id FROM desktop_organizations WHERE id=$1 AND deleted_at IS NULL`, id).Scan(&value)
	return value, err
}

func (r *desktopReportRepository) AnalysisKey(ctx context.Context, o *service.DesktopOrganization) (*service.APIKey, error) {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	current, err := tx.DesktopOrganization.Query().Where(desktoporganization.IDEQ(o.ID)).ForUpdate().Only(ctx)
	if err != nil {
		return nil, err
	}
	if !current.ConversationSummaryEnabled || current.GatewayUserID != o.GatewayUserID || current.GroupID != o.GroupID {
		return nil, service.ErrDesktopRotationConflict
	}
	key, err := tx.APIKey.Query().Where(apikey.DesktopAnalysisOrganizationIDEQ(o.ID), apikey.DeletedAtIsNil()).Only(ctx)
	if dbent.IsNotFound(err) {
		secret, genErr := service.GenerateDesktopRefreshToken()
		if genErr != nil {
			return nil, genErr
		}
		key, err = tx.APIKey.Create().SetUserID(o.GatewayUserID).SetGroupID(o.GroupID).SetKey("sk-da-" + secret).SetName(fmt.Sprintf("desktop-analysis-%d", o.ID)).SetDesktopAnalysisOrganizationID(o.ID).Save(ctx)
	} else if err == nil {
		key, err = tx.APIKey.UpdateOne(key).SetUserID(o.GatewayUserID).SetGroupID(o.GroupID).SetStatus(service.StatusActive).Save(ctx)
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return apiKeyEntityToService(key), nil
}

func (r *desktopReportRepository) BeginExecution(ctx context.Context, task *service.DesktopReportTask, execution *service.DesktopReportExecution) error {
	err := r.db.QueryRowContext(ctx, `INSERT INTO desktop_daily_report_executions
 (task_id,organization_id,report_date,revision,kind,member_id,model,round,chunk,attempt,request_id)
 SELECT id,organization_id,report_date,revision,$3,$4,$5,$6,$7,$8,$9
 FROM desktop_daily_report_tasks WHERE id=$1 AND lease_token=$2 AND lease_until>NOW()
 RETURNING id,started_at`, task.ID, task.LeaseToken, execution.Kind, execution.MemberID, execution.Model, execution.Round, execution.Chunk, execution.Attempt, execution.RequestID).Scan(&execution.ID, &execution.StartedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrDesktopReportLeaseLost
	}
	return err
}
func (r *desktopReportRepository) FinishExecution(ctx context.Context, execution *service.DesktopReportExecution) error {
	result, err := r.db.ExecContext(ctx, `UPDATE desktop_daily_report_executions SET status=$2,error=$3,finished_at=$4 WHERE id=$1 AND request_id=$5 AND status='running'`, execution.ID, execution.Status, execution.Error, execution.FinishedAt, execution.RequestID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return service.ErrDesktopReportLeaseLost
	}
	return nil
}
func (r *desktopReportRepository) Executions(ctx context.Context, org int64, date string) ([]service.DesktopReportExecution, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,revision,kind,member_id,model,round,chunk,attempt,request_id,status,error,started_at,finished_at
 FROM desktop_daily_report_executions WHERE organization_id=$1 AND report_date=$2::date ORDER BY id DESC`, org, date)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	executions := []service.DesktopReportExecution{}
	for rows.Next() {
		var execution service.DesktopReportExecution
		if err = rows.Scan(&execution.ID, &execution.Revision, &execution.Kind, &execution.MemberID, &execution.Model, &execution.Round, &execution.Chunk, &execution.Attempt, &execution.RequestID, &execution.Status, &execution.Error, &execution.StartedAt, &execution.FinishedAt); err != nil {
			return nil, err
		}
		executions = append(executions, execution)
	}
	return executions, rows.Err()
}
