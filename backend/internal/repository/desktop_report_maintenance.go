package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *desktopReportRepository) ReportScanDate(ctx context.Context, kind string, first time.Time) (time.Time, error) {
	var date string
	err := r.db.QueryRowContext(ctx, `INSERT INTO desktop_maintenance_progress(name,next_date) VALUES($1,$2::date)
 ON CONFLICT(name) DO UPDATE SET next_date=GREATEST(desktop_maintenance_progress.next_date,EXCLUDED.next_date),updated_at=NOW()
 RETURNING next_date::text`, "backfill:"+kind, first.Format("2006-01-02")).Scan(&date)
	if err != nil {
		return time.Time{}, err
	}
	return time.ParseInLocation("2006-01-02", date, first.Location())
}
func (r *desktopReportRepository) AdvanceReportScan(ctx context.Context, kind string, next time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE desktop_maintenance_progress SET next_date=GREATEST(next_date,$2::date),updated_at=NOW() WHERE name=$1`, "backfill:"+kind, next.Format("2006-01-02"))
	return err
}

var _ service.DesktopReportMaintenanceRepository = (*desktopReportRepository)(nil)
