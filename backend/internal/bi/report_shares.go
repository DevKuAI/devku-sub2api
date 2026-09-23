package bi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

func shareDTO(id, reportID string, expires time.Time) Share {
	return Share{ID: id, ReportID: reportID, ExpiresAt: expires.UTC().Format(time.RFC3339Nano), Path: "/pages/report/index?share_id=" + id, Title: "得酷 · 管理简报"}
}

func (s *Service) CreateShare(ctx context.Context, p Principal, org, reportID, key string, seconds int64) (Share, error) {
	if seconds == 0 {
		seconds = 604800
	}
	if seconds < 60 || seconds > 604800 {
		return Share{}, invalid("expires_in_seconds", "Share lifetime must be 60–604800 seconds")
	}
	if _, err := s.AuthorizeOrganization(ctx, p, org, "reports:share"); err != nil {
		return Share{}, err
	}
	report, err := s.ReadReport(ctx, p, org, reportID, "reports:share")
	if err != nil {
		return Share{}, err
	}
	if report.Report.Status != "ready" {
		return Share{}, apiError(409, "CONFLICT", "Report is not ready")
	}
	input := struct {
		ReportID string
		Seconds  int64
	}{reportID, seconds}
	raw, err := command(ctx, s, p, org, "createShare", key, input, func(tx *sql.Tx) (any, error) {
		now := s.now().UTC().Truncate(time.Microsecond)
		expires := now.Add(time.Duration(seconds) * time.Second)
		reportExpiry, err := time.Parse(time.RFC3339Nano, report.Report.ExpiresAt)
		if err != nil {
			return nil, err
		}
		if expires.After(reportExpiry) {
			expires = reportExpiry
		}
		share := shareDTO(randomToken("share_"), reportID, expires)
		_, err = tx.ExecContext(ctx, `INSERT INTO bi_report_shares(id,report_id,organization_id,manager_id,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6)`, share.ID, reportID, org, p.ManagerID, now, expires)
		if err != nil {
			return nil, err
		}
		if err := recordSecurityEvent(ctx, tx, p.ManagerID, "report.share", share.ID, randomToken("share_request_")); err != nil {
			return nil, err
		}
		return share, nil
	})
	if err != nil {
		return Share{}, err
	}
	var result Share
	err = json.Unmarshal(raw, &result)
	return result, err
}

func (s *Service) RevokeShare(ctx context.Context, p Principal, org, id string) error {
	scope, err := s.AuthorizeOrganization(ctx, p, org, "reports:share")
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var creator string
	err = tx.QueryRowContext(ctx, `SELECT manager_id FROM bi_report_shares WHERE id=$1 AND organization_id=$2 FOR UPDATE`, id, org).Scan(&creator)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if creator != p.ManagerID && scope.Role != "org_admin" {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `UPDATE bi_report_shares SET revoked_at=COALESCE(revoked_at,$2) WHERE id=$1`, id, s.now().UTC()); err != nil {
		return err
	}
	if err := recordSecurityEvent(ctx, tx, p.ManagerID, "report.share_revoke", id, randomToken("share_request_")); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) ResolveShare(ctx context.Context, p Principal, id string) (SharedReport, error) {
	var org, reportID, managerID string
	err := s.db.QueryRowContext(ctx, `SELECT organization_id,report_id,manager_id FROM bi_report_shares WHERE id=$1 AND revoked_at IS NULL AND expires_at>$2`, id, s.now().UTC()).Scan(&org, &reportID, &managerID)
	if errors.Is(err, sql.ErrNoRows) {
		return SharedReport{}, ErrNotFound
	}
	if err != nil {
		return SharedReport{}, err
	}
	creator, err := s.reportPrincipal(ctx, managerID)
	if errors.Is(err, ErrUnauthenticated) || errors.Is(err, sql.ErrNoRows) {
		return SharedReport{}, ErrNotFound
	}
	if err != nil {
		return SharedReport{}, err
	}
	if _, err := s.ReadReport(ctx, creator, org, reportID, "reports:share"); err != nil {
		return SharedReport{}, err
	}
	report, err := s.ReadReport(ctx, p, org, reportID, "reports:read")
	if err != nil {
		return SharedReport{}, err
	}
	return SharedReport{ShareID: id, Report: report}, nil
}

func (s *Service) reportSummaries(ctx context.Context, p Principal, org string) ([]ReportSummary, error) {
	if _, err := s.AuthorizeOrganization(ctx, p, org, "reports:read"); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM bi_reports WHERE organization_id=$1 AND expires_at>$2 ORDER BY created_at DESC,id`, org, s.now().UTC())
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	result := []ReportSummary{}
	for _, id := range ids {
		report, err := s.ReadReport(ctx, p, org, id, "reports:read")
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		result = append(result, report.Report)
	}
	return result, nil
}

func (s *Service) reportShares(ctx context.Context, p Principal, org, reportID string) ([]Share, error) {
	if _, err := s.AuthorizeOrganization(ctx, p, org, "reports:share"); err != nil {
		return nil, err
	}
	if _, err := s.ReadReport(ctx, p, org, reportID, "reports:share"); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,expires_at FROM bi_report_shares WHERE manager_id=$1 AND organization_id=$2 AND report_id=$3 AND revoked_at IS NULL AND expires_at>$4 ORDER BY created_at DESC,id`, p.ManagerID, org, reportID, s.now().UTC())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []Share{}
	for rows.Next() {
		var id string
		var expires time.Time
		if err := rows.Scan(&id, &expires); err != nil {
			return nil, err
		}
		result = append(result, shareDTO(id, reportID, expires))
	}
	return result, rows.Err()
}
