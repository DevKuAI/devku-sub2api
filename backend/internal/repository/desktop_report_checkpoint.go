package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/lib/pq"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type desktopReportQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func desktopReportTaskPayload(task *service.DesktopReportTask) ([]byte, error) {
	payload := task.Payload
	payload.Snapshots = nil
	return json.Marshal(payload)
}

func loadDesktopReportMembers(ctx context.Context, db desktopReportQueryer, task *service.DesktopReportTask) error {
	rows, err := db.QueryContext(ctx, `SELECT member,sources,work FROM desktop_daily_report_members WHERE task_id=$1 ORDER BY member_id`, task.ID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	task.Payload.Snapshots = []service.DesktopReportSnapshot{}
	for rows.Next() {
		var snapshot service.DesktopReportSnapshot
		var member, sources, work []byte
		if err = rows.Scan(&member, &sources, &work); err != nil {
			return err
		}
		for _, value := range []struct {
			data   []byte
			target any
		}{{member, &snapshot.Member}, {sources, &snapshot.Sources}, {work, &snapshot.Work}} {
			if err = json.Unmarshal(value.data, value.target); err != nil {
				return err
			}
		}
		task.Payload.Snapshots = append(task.Payload.Snapshots, snapshot)
	}
	return rows.Err()
}

func saveDesktopReportMembers(ctx context.Context, tx *sql.Tx, task *service.DesktopReportTask) error {
	if task.Checkpoint == "summary" {
		return nil
	}
	if task.Checkpoint == "" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM desktop_daily_report_members WHERE task_id=$1`, task.ID); err != nil {
			return err
		}
	}
	for _, snapshot := range task.Payload.Snapshots {
		if task.Checkpoint != "" && strings.TrimPrefix(task.Checkpoint, "member:") != snapshot.Member.MemberID {
			continue
		}
		member, err := json.Marshal(snapshot.Member)
		if err != nil {
			return err
		}
		metadata := snapshot.Member
		metadata.Content = ""
		metadata.SourceIDs = nil
		metadata.Executions = nil
		meta, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		sources, err := json.Marshal(snapshot.Sources)
		if err != nil {
			return err
		}
		work, err := json.Marshal(snapshot.Work)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO desktop_daily_report_members(task_id,member_id,member,metadata,sources,work) VALUES($1,$2,$3::jsonb,$4::jsonb,$5::jsonb,$6::jsonb) ON CONFLICT(task_id,member_id) DO UPDATE SET member=EXCLUDED.member,metadata=EXCLUDED.metadata,sources=EXCLUDED.sources,work=EXCLUDED.work`, task.ID, snapshot.Member.MemberID, string(member), string(meta), string(sources), string(work)); err != nil {
			return err
		}
	}
	return nil
}

func (r *desktopReportRepository) SnapshotMetadata(ctx context.Context, org int64, date string) ([]service.DesktopReportSnapshot, error) {
	members, err := r.Day(ctx, org, date)
	if err != nil {
		return nil, err
	}
	snapshots := []service.DesktopReportSnapshot{}
	for _, m := range members {
		if m.RecordCount > 0 {
			snapshots = append(snapshots, service.DesktopReportSnapshot{Member: m})
		}
	}
	return snapshots, nil
}
func (r *desktopReportRepository) SnapshotSources(ctx context.Context, org int64, ids []string) ([]service.DesktopReportSource, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT record_id::text,client,source_session_id,stopped_at,prompts,response,capture_status FROM desktop_conversation_records WHERE organization_id=$1 AND record_id=ANY($2::uuid[]) ORDER BY stopped_at,id`, org, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []service.DesktopReportSource{}
	for rows.Next() {
		var source service.DesktopReportSource
		var prompts, response []byte
		if err = rows.Scan(&source.RecordID, &source.Client, &source.SessionID, &source.StoppedAt, &prompts, &response, &source.CaptureStatus); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(prompts, &source.Prompts); err != nil {
			return nil, err
		}
		if len(response) > 0 {
			if err = json.Unmarshal(response, &source.Response); err != nil {
				return nil, err
			}
		}
		result = append(result, source)
	}
	return result, rows.Err()
}
