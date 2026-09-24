package bi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
)

const (
	defaultReportRetentionMonths  = 24
	defaultFactRetentionMonths    = 24
	defaultAuditRetentionDays     = 180
	defaultEphemeralRetentionDays = 7
)

type AdminRetentionPolicy struct {
	ReportMonths  int        `json:"report_months"`
	FactMonths    int        `json:"fact_months"`
	AuditDays     int        `json:"audit_days"`
	EphemeralDays int        `json:"ephemeral_days"`
	UpdatedBy     *int64     `json:"updated_by,omitempty"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
}

type AdminOverview struct {
	Config    AdminBIConfigStatus  `json:"config"`
	Counts    map[string]int64     `json:"counts"`
	Retention AdminRetentionPolicy `json:"retention"`
}

type AdminBIConfigStatus struct {
	Enabled               bool   `json:"enabled"`
	AppID                 string `json:"appid"`
	MinClientVersion      string `json:"min_client_version"`
	AppSecretConfigured   bool   `json:"app_secret_configured"`
	JWTSecretConfigured   bool   `json:"jwt_secret_configured"`
	IdentityConfigured    bool   `json:"identity_secret_configured"`
	ReportRetentionMonths int    `json:"report_retention_months"`
}

type AdminBinding struct {
	ID           string     `json:"id"`
	UserID       int64      `json:"user_id"`
	DisplayName  string     `json:"display_name"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	RevokedAt    *time.Time `json:"revoked_at"`
	SessionCount int64      `json:"session_count"`
}

type AdminChallenge struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	BindingID *string   `json:"binding_id"`
}

type AdminSession struct {
	ID            string     `json:"id"`
	BindingID     string     `json:"binding_id"`
	UserID        int64      `json:"user_id"`
	Status        string     `json:"status"`
	DeviceID      *string    `json:"device_id"`
	Platform      *string    `json:"platform"`
	ClientVersion *string    `json:"client_version"`
	CreatedAt     time.Time  `json:"created_at"`
	LastSeenAt    *time.Time `json:"last_seen_at"`
	ExpiresAt     time.Time  `json:"expires_at"`
	RevokedAt     *time.Time `json:"revoked_at"`
}

type AdminSource struct {
	OrganizationID   string     `json:"organization_id"`
	SourceID         string     `json:"source_id"`
	Namespace        string     `json:"namespace"`
	AllowedKinds     []string   `json:"allowed_kinds"`
	Checkpoint       *string    `json:"checkpoint"`
	LastAppliedAt    *time.Time `json:"last_applied_at"`
	CompleteThrough  *time.Time `json:"complete_through"`
	HistoryStartDate *string    `json:"history_start_date"`
	BackfillComplete bool       `json:"initial_backfill_complete"`
	CredentialCount  int64      `json:"credential_count"`
}

type AdminCredential struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	SourceID       string     `json:"source_id"`
	TokenPrefix    string     `json:"token_prefix"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
	RevokedAt      *time.Time `json:"revoked_at"`
}

type AdminImport struct {
	ID             string        `json:"id"`
	OrganizationID string        `json:"organization_id"`
	SourceID       string        `json:"source_id"`
	Status         string        `json:"status"`
	ReceivedAt     time.Time     `json:"received_at"`
	AppliedAt      *time.Time    `json:"applied_at"`
	Checkpoint     *string       `json:"checkpoint"`
	DataRevision   *string       `json:"data_revision"`
	Errors         []ImportError `json:"errors"`
	LeaseUntil     *time.Time    `json:"lease_until"`
	AttemptCount   int           `json:"attempt_count"`
	RetryOf        *string       `json:"retry_of"`
}

type AdminQuality struct {
	OrganizationID    string     `json:"organization_id"`
	SourceID          string     `json:"source_id"`
	Status            string     `json:"status"`
	CompleteThrough   *time.Time `json:"complete_through"`
	HistoryStartDate  *string    `json:"history_start_date"`
	CoverageStart     *string    `json:"coverage_start"`
	CoverageEnd       *string    `json:"coverage_end"`
	LatencySeconds    *float64   `json:"latency_seconds"`
	MissingDimensions []string   `json:"missing_dimensions"`
	Reason            *string    `json:"reason"`
}

type AdminReport struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	ManagerID      string     `json:"manager_id"`
	Title          string     `json:"title"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
	FailureCode    *string    `json:"failure_code"`
	GeneratedAt    *time.Time `json:"generated_at"`
	LeaseUntil     *time.Time `json:"lease_until"`
	AttemptCount   int        `json:"attempt_count"`
	ArchivedAt     *time.Time `json:"archived_at"`
	RetryOf        *string    `json:"retry_of"`
}

type AdminCleanup struct {
	LastRunAt   *time.Time           `json:"last_run_at"`
	LastDeleted int64                `json:"last_deleted"`
	Retention   AdminRetentionPolicy `json:"retention"`
}

type AdminAuditEvent struct {
	ID             int64          `json:"id"`
	ActorUserID    *int64         `json:"actor_user_id"`
	OrganizationID *string        `json:"organization_id"`
	Action         string         `json:"action"`
	TargetID       string         `json:"target_id"`
	RequestID      string         `json:"request_id"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
}

type adminExec interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (s *Service) retentionPolicy(ctx context.Context) (AdminRetentionPolicy, error) {
	policy := AdminRetentionPolicy{ReportMonths: defaultReportRetentionMonths, FactMonths: defaultFactRetentionMonths, AuditDays: defaultAuditRetentionDays, EphemeralDays: defaultEphemeralRetentionDays}
	if s.db == nil {
		return policy, nil
	}
	var updatedBy sql.NullInt64
	var updatedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT report_months,fact_months,audit_days,ephemeral_days,updated_by,updated_at FROM bi_retention_policies WHERE id=TRUE`).
		Scan(&policy.ReportMonths, &policy.FactMonths, &policy.AuditDays, &policy.EphemeralDays, &updatedBy, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return policy, nil
	}
	if err != nil {
		return policy, err
	}
	if updatedBy.Valid {
		policy.UpdatedBy = &updatedBy.Int64
	}
	if updatedAt.Valid {
		policy.UpdatedAt = &updatedAt.Time
	}
	return policy, nil
}

func (s *Service) saveRetentionPolicy(ctx context.Context, policy AdminRetentionPolicy, actorID int64, requestID string) error {
	if policy.ReportMonths < 1 || policy.ReportMonths > 120 || policy.FactMonths < 1 || policy.FactMonths > 120 || policy.AuditDays < 1 || policy.AuditDays > 3650 || policy.EphemeralDays < 1 || policy.EphemeralDays > 365 {
		return invalid("retention", "Retention values are outside the supported range")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `INSERT INTO bi_retention_policies(id,report_months,fact_months,audit_days,ephemeral_days,updated_by,updated_at)
		VALUES(TRUE,$1,$2,$3,$4,$5,NOW()) ON CONFLICT(id) DO UPDATE SET report_months=EXCLUDED.report_months,fact_months=EXCLUDED.fact_months,audit_days=EXCLUDED.audit_days,ephemeral_days=EXCLUDED.ephemeral_days,updated_by=EXCLUDED.updated_by,updated_at=NOW()`, policy.ReportMonths, policy.FactMonths, policy.AuditDays, policy.EphemeralDays, actorID)
	if err != nil {
		return err
	}
	if err := insertAdminSecurityEvent(ctx, tx, actorID, "retention.update", "bi_retention_policy", "", requestID, nil); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) adminConfigStatus(ctx context.Context) (AdminBIConfigStatus, error) {
	retention, err := s.retentionPolicy(ctx)
	if err != nil {
		return AdminBIConfigStatus{}, err
	}
	return AdminBIConfigStatus{Enabled: s.config.Enabled, AppID: s.config.AppID, MinClientVersion: s.config.MinClientVersion,
		AppSecretConfigured: strings.TrimSpace(s.config.AppSecret) != "", JWTSecretConfigured: strings.TrimSpace(s.config.JWTSecret) != "", IdentityConfigured: strings.TrimSpace(s.config.IdentitySecret) != "", ReportRetentionMonths: retention.ReportMonths}, nil
}

func (s *Service) adminOverview(ctx context.Context) (AdminOverview, error) {
	configStatus, err := s.adminConfigStatus(ctx)
	if err != nil {
		return AdminOverview{}, err
	}
	retention, err := s.retentionPolicy(ctx)
	if err != nil {
		return AdminOverview{}, err
	}
	counts := map[string]int64{}
	if s.db == nil {
		return AdminOverview{Config: configStatus, Counts: counts, Retention: retention}, nil
	}
	queries := map[string]string{
		"bindings":           `SELECT COUNT(*) FROM bi_wechat_bindings`,
		"pending_challenges": `SELECT COUNT(*) FROM bi_binding_challenges WHERE status IN ('pending','approved') AND expires_at>NOW()`,
		"active_sessions":    `SELECT COUNT(*) FROM bi_sessions WHERE revoked_at IS NULL AND expires_at>NOW()`,
		"sources":            `SELECT COUNT(*) FROM bi_connector_sources`,
		"imports_processing": `SELECT COUNT(*) FROM bi_import_batches WHERE status IN ('queued','validating')`,
		"reports_processing": `SELECT COUNT(*) FROM bi_reports WHERE status IN ('queued','running')`,
	}
	for key, query := range queries {
		var count int64
		if err := s.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
			return AdminOverview{}, err
		}
		counts[key] = count
	}
	return AdminOverview{Config: configStatus, Counts: counts, Retention: retention}, nil
}

func adminPagination(ctx *gin.Context) (int, int) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func adminRequestID(ctx *gin.Context) string {
	if value, ok := ctx.Request.Context().Value(ctxkey.RequestID).(string); ok && value != "" {
		return value
	}
	return "admin-" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

func importStatus(status string, leaseUntil *time.Time, now time.Time, attempts int) string {
	if (status == "validating" || status == "queued") && leaseUntil != nil && leaseUntil.Before(now) && attempts > 0 {
		return "failed"
	}
	if status == "validating" {
		return "processing"
	}
	return status
}

func reportStatus(status string, leaseUntil *time.Time, now time.Time) string {
	if status == "running" && leaseUntil != nil && leaseUntil.Before(now) {
		return "lease_timeout"
	}
	return status
}

func (h *Handler) AdminOverview(c *gin.Context) {
	result, err := h.service.adminOverview(c.Request.Context())
	adminRespond(c, result, err)
}

func (h *Handler) AdminGetRetention(c *gin.Context) {
	result, err := h.service.retentionPolicy(c.Request.Context())
	adminRespond(c, result, err)
}

func (h *Handler) AdminUpdateRetention(c *gin.Context, actorID int64) {
	var input AdminRetentionPolicy
	if !decodeRequest(c, &input) {
		return
	}
	err := h.service.saveRetentionPolicy(c.Request.Context(), input, actorID, adminRequestID(c))
	if err == nil {
		input, err = h.service.retentionPolicy(c.Request.Context())
	}
	adminRespond(c, input, err)
}

func (s *Service) listAdminBindings(ctx context.Context, page, pageSize int, status string) ([]AdminBinding, int64, error) {
	if s.db == nil {
		return []AdminBinding{}, 0, nil
	}
	where := ""
	args := []any{}
	if status != "" {
		where = " WHERE CASE WHEN b.revoked_at IS NOT NULL THEN 'revoked' ELSE 'active' END=$1"
		args = append(args, status)
	}
	var total int64
	countSQL := `SELECT COUNT(*) FROM bi_wechat_bindings b` + where
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, `SELECT b.id,m.user_id,COALESCE(u.username,''),CASE WHEN b.revoked_at IS NOT NULL THEN 'revoked' ELSE 'active' END,b.created_at,b.last_login_at,b.revoked_at,
		(SELECT COUNT(*) FROM bi_sessions s WHERE s.binding_id=b.id)`+where+` ORDER BY b.created_at DESC,b.id LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := []AdminBinding{}
	for rows.Next() {
		var item AdminBinding
		if err := rows.Scan(&item.ID, &item.UserID, &item.DisplayName, &item.Status, &item.CreatedAt, &item.LastLoginAt, &item.RevokedAt, &item.SessionCount); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (h *Handler) AdminListBindings(c *gin.Context) {
	page, size := adminPagination(c)
	items, total, err := h.service.listAdminBindings(c.Request.Context(), page, size, c.Query("status"))
	adminRespond(c, gin.H{"items": items, "total": total, "page": page, "page_size": size}, err)
}

func (s *Service) listAdminChallenges(ctx context.Context, page, pageSize int, status string) ([]AdminChallenge, int64, error) {
	if s.db == nil {
		return []AdminChallenge{}, 0, nil
	}
	where := ""
	args := []any{}
	if status != "" {
		where = " WHERE c.status=$1"
		args = append(args, status)
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM bi_binding_challenges c"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, "SELECT c.ticket_hash,c.status,c.created_at,c.expires_at,c.binding_id FROM bi_binding_challenges c"+where+" ORDER BY c.created_at DESC LIMIT $"+strconv.Itoa(len(args)-1)+" OFFSET $"+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := []AdminChallenge{}
	for rows.Next() {
		var item AdminChallenge
		var binding sql.NullString
		if err := rows.Scan(&item.ID, &item.Status, &item.CreatedAt, &item.ExpiresAt, &binding); err != nil {
			return nil, 0, err
		}
		if binding.Valid {
			item.BindingID = &binding.String
		}
		if item.ExpiresAt.Before(s.now()) && item.Status != "consumed" && item.Status != "revoked" {
			item.Status = "expired"
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (h *Handler) AdminListChallenges(c *gin.Context) {
	page, size := adminPagination(c)
	items, total, err := h.service.listAdminChallenges(c.Request.Context(), page, size, c.Query("status"))
	adminRespond(c, gin.H{"items": items, "total": total, "page": page, "page_size": size}, err)
}

func (s *Service) listAdminSessions(ctx context.Context, page, pageSize int, status string) ([]AdminSession, int64, error) {
	if s.db == nil {
		return []AdminSession{}, 0, nil
	}
	where := ""
	args := []any{}
	if status != "" {
		where = " WHERE CASE WHEN s.revoked_at IS NOT NULL THEN 'revoked' WHEN s.expires_at<=NOW() THEN 'expired' ELSE 'active' END=$1"
		args = append(args, status)
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM bi_sessions s"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, "SELECT s.id,s.binding_id,m.user_id,CASE WHEN s.revoked_at IS NOT NULL THEN 'revoked' WHEN s.expires_at<=NOW() THEN 'expired' ELSE 'active' END,s.device_id_hash,s.device_platform,s.client_version,s.created_at,s.last_seen_at,s.expires_at,s.revoked_at FROM bi_sessions s JOIN bi_wechat_bindings b ON b.id=s.binding_id JOIN bi_managers m ON m.id=b.manager_id"+where+" ORDER BY s.created_at DESC LIMIT $"+strconv.Itoa(len(args)-1)+" OFFSET $"+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := []AdminSession{}
	for rows.Next() {
		var item AdminSession
		var hash sql.NullString
		if err := rows.Scan(&item.ID, &item.BindingID, &item.UserID, &item.Status, &hash, &item.Platform, &item.ClientVersion, &item.CreatedAt, &item.LastSeenAt, &item.ExpiresAt, &item.RevokedAt); err != nil {
			return nil, 0, err
		}
		if hash.Valid {
			item.DeviceID = ptr("sha256:" + hash.String[:12])
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (h *Handler) AdminListSessions(c *gin.Context) {
	page, size := adminPagination(c)
	items, total, err := h.service.listAdminSessions(c.Request.Context(), page, size, c.Query("status"))
	adminRespond(c, gin.H{"items": items, "total": total, "page": page, "page_size": size}, err)
}

func (s *Service) forceRevokeBinding(ctx context.Context, bindingID string, actorID int64, requestID string) error {
	var openHash, managerID string
	if err := s.db.QueryRowContext(ctx, `SELECT openid_hash,manager_id FROM bi_wechat_bindings WHERE id=$1 AND appid=$2`, bindingID, s.config.AppID).Scan(&openHash, &managerID); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	return s.identityTx(ctx, openHash, func(tx *sql.Tx) error {
		now := s.now().UTC()
		if _, err := tx.ExecContext(ctx, `UPDATE bi_wechat_bindings SET revoked_at=COALESCE(revoked_at,$2) WHERE id=$1`, bindingID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE bi_sessions SET revoked_at=COALESCE(revoked_at,$2) WHERE binding_id=$1`, bindingID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE bi_binding_challenges SET status='revoked' WHERE appid=$1 AND openid_hash=$2 AND status IN ('pending','approved')`, s.config.AppID, openHash); err != nil {
			return err
		}
		return insertAdminSecurityEvent(ctx, tx, actorID, "binding.admin_revoke", bindingID, "", requestID, map[string]any{"manager_id": managerID})
	})
}

func (h *Handler) AdminRevokeBinding(c *gin.Context, actorID int64) {
	err := h.service.forceRevokeBinding(c.Request.Context(), c.Param("binding_id"), actorID, adminRequestID(c))
	adminRespond(c, gin.H{"revoked": err == nil}, err)
}

func (s *Service) listAdminSources(ctx context.Context, page, size int, org, source, namespace string) ([]AdminSource, int64, error) {
	if s.db == nil {
		return []AdminSource{}, 0, nil
	}
	where := " WHERE 1=1"
	args := []any{}
	for _, item := range []struct{ value, key string }{{org, "s.organization_id"}, {source, "s.source_id"}, {namespace, "s.namespace"}} {
		if item.value != "" {
			args = append(args, item.value)
			where += " AND " + item.key + "=$" + strconv.Itoa(len(args))
		}
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM bi_connector_sources s"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, size, (page-1)*size)
	rows, err := s.db.QueryContext(ctx, "SELECT s.organization_id,s.source_id,s.namespace,s.allowed_kinds,s.checkpoint,s.last_applied_at,s.complete_through,to_char(s.history_start_date,'YYYY-MM-DD'),s.initial_backfill_complete,(SELECT COUNT(*) FROM bi_connector_credentials c WHERE c.organization_id=s.organization_id AND c.source_id=s.source_id AND c.revoked_at IS NULL AND c.expires_at>NOW()) FROM bi_connector_sources s"+where+" ORDER BY s.organization_id,s.source_id LIMIT $"+strconv.Itoa(len(args)-1)+" OFFSET $"+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := []AdminSource{}
	for rows.Next() {
		var item AdminSource
		var kinds []byte
		var history sql.NullString
		if err := rows.Scan(&item.OrganizationID, &item.SourceID, &item.Namespace, &kinds, &item.Checkpoint, &item.LastAppliedAt, &item.CompleteThrough, &history, &item.BackfillComplete, &item.CredentialCount); err != nil {
			return nil, 0, err
		}
		if err := json.Unmarshal(kinds, &item.AllowedKinds); err != nil {
			return nil, 0, err
		}
		if history.Valid {
			item.HistoryStartDate = &history.String
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (h *Handler) AdminListSources(c *gin.Context) {
	page, size := adminPagination(c)
	items, total, err := h.service.listAdminSources(c.Request.Context(), page, size, c.Query("organization_id"), c.Query("source_id"), c.Query("namespace"))
	adminRespond(c, gin.H{"items": items, "total": total, "page": page, "page_size": size}, err)
}

func (s *Service) listAdminCredentials(ctx context.Context, sourceID, organizationID string) ([]AdminCredential, error) {
	if s.db == nil {
		return []AdminCredential{}, nil
	}
	args := []any{}
	where := ""
	if sourceID != "" {
		args = append(args, sourceID)
		where = " WHERE source_id=$1"
	}
	if organizationID != "" {
		args = append(args, organizationID)
		if where == "" {
			where = " WHERE organization_id=$1"
		} else {
			where += " AND organization_id=$" + strconv.Itoa(len(args))
		}
	}
	rows, err := s.db.QueryContext(ctx, "SELECT id,organization_id,source_id,LEFT(token_hash,12),CASE WHEN revoked_at IS NOT NULL THEN 'revoked' WHEN expires_at<=NOW() THEN 'expired' ELSE 'active' END,created_at,expires_at,revoked_at FROM bi_connector_credentials"+where+" ORDER BY created_at DESC", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []AdminCredential{}
	for rows.Next() {
		var item AdminCredential
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.SourceID, &item.TokenPrefix, &item.Status, &item.CreatedAt, &item.ExpiresAt, &item.RevokedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (h *Handler) AdminListCredentials(c *gin.Context) {
	items, err := h.service.listAdminCredentials(c.Request.Context(), c.Param("source_id"), c.Query("organization_id"))
	adminRespond(c, gin.H{"items": items}, err)
}

type AdminSourceInput struct {
	OrganizationID string    `json:"organization_id"`
	SourceID       string    `json:"source_id"`
	Namespace      string    `json:"namespace"`
	AllowedKinds   []string  `json:"allowed_kinds"`
	ExpiresAt      time.Time `json:"expires_at"`
}

func (h *Handler) AdminIssueCredential(c *gin.Context, actorID int64) {
	var input AdminSourceInput
	if !decodeRequest(c, &input) {
		return
	}
	if input.SourceID == "" {
		input.SourceID = c.Param("source_id")
	}
	id, token, err := RegisterConnector(c.Request.Context(), h.service.db, ConnectorRegistration{OrganizationID: input.OrganizationID, SourceID: input.SourceID, Namespace: input.Namespace, AllowedKinds: input.AllowedKinds, ExpiresAt: input.ExpiresAt})
	if err == nil {
		err = h.service.insertSecurityEvent(c.Request.Context(), actorID, "connector.issue", id, input.OrganizationID, adminRequestID(c), nil)
	}
	adminRespond(c, gin.H{"credential_id": id, "token": token}, err)
}

func (h *Handler) AdminRevokeCredential(c *gin.Context, actorID int64) {
	err := RevokeConnector(c.Request.Context(), h.service.db, c.Param("credential_id"))
	if err == nil {
		err = h.service.insertSecurityEvent(c.Request.Context(), actorID, "connector.revoke", c.Param("credential_id"), "", adminRequestID(c), nil)
	}
	adminRespond(c, gin.H{"revoked": err == nil}, err)
}

func (h *Handler) AdminRotateCredential(c *gin.Context, actorID int64) {
	var input struct {
		ExpiresAt time.Time `json:"expires_at"`
	}
	if !decodeRequest(c, &input) {
		return
	}
	var registration ConnectorRegistration
	var rawKinds []byte
	err := h.service.db.QueryRowContext(c.Request.Context(), `SELECT c.organization_id,c.source_id,s.namespace,s.allowed_kinds FROM bi_connector_credentials c JOIN bi_connector_sources s ON s.organization_id=c.organization_id AND s.source_id=c.source_id WHERE c.id=$1`, c.Param("credential_id")).Scan(&registration.OrganizationID, &registration.SourceID, &registration.Namespace, &rawKinds)
	if err != nil {
		adminRespond(c, nil, ErrNotFound)
		return
	}
	if err = json.Unmarshal(rawKinds, &registration.AllowedKinds); err != nil {
		adminRespond(c, nil, err)
		return
	}
	id, token, err := RegisterConnector(c.Request.Context(), h.service.db, registrationWithExpiry(registration, input.ExpiresAt))
	if err == nil {
		err = h.service.insertSecurityEvent(c.Request.Context(), actorID, "connector.rotate", id, registration.OrganizationID, adminRequestID(c), map[string]any{"replaced": c.Param("credential_id")})
	}
	if err == nil {
		err = RevokeConnector(c.Request.Context(), h.service.db, c.Param("credential_id"))
	}
	adminRespond(c, gin.H{"credential_id": id, "token": token}, err)
}

func registrationWithExpiry(input ConnectorRegistration, expiresAt time.Time) ConnectorRegistration {
	input.ExpiresAt = expiresAt
	return input
}

func (s *Service) listAdminImports(ctx context.Context, page, size int, org, source, status string) ([]AdminImport, int64, error) {
	if s.db == nil {
		return []AdminImport{}, 0, nil
	}
	where := " WHERE 1=1"
	args := []any{}
	for _, item := range []struct{ value, key string }{{org, "organization_id"}, {source, "source_id"}, {status, "status"}} {
		if item.value != "" {
			args = append(args, item.value)
			where += " AND " + item.key + "=$" + strconv.Itoa(len(args))
		}
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM bi_import_batches"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, size, (page-1)*size)
	rows, err := s.db.QueryContext(ctx, "SELECT id,organization_id,source_id,status,received_at,applied_at,checkpoint,data_revision,errors,lease_until,attempt_count,retry_of FROM bi_import_batches"+where+" ORDER BY received_at DESC,id LIMIT $"+strconv.Itoa(len(args)-1)+" OFFSET $"+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := []AdminImport{}
	for rows.Next() {
		var item AdminImport
		var raw []byte
		var checkpoint, revision, retry sql.NullString
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.SourceID, &item.Status, &item.ReceivedAt, &item.AppliedAt, &checkpoint, &revision, &raw, &item.LeaseUntil, &item.AttemptCount, &retry); err != nil {
			return nil, 0, err
		}
		if checkpoint.Valid {
			item.Checkpoint = &checkpoint.String
		}
		if revision.Valid {
			item.DataRevision = &revision.String
		}
		if retry.Valid {
			item.RetryOf = &retry.String
		}
		_ = json.Unmarshal(raw, &item.Errors)
		item.Status = importStatus(item.Status, item.LeaseUntil, s.now(), item.AttemptCount)
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (h *Handler) AdminListImports(c *gin.Context) {
	page, size := adminPagination(c)
	items, total, err := h.service.listAdminImports(c.Request.Context(), page, size, c.Query("organization_id"), c.Query("source_id"), c.Query("status"))
	adminRespond(c, gin.H{"items": items, "total": total, "page": page, "page_size": size}, err)
}

func (s *Service) adminImport(ctx context.Context, id string) (AdminImport, error) {
	var item AdminImport
	var raw []byte
	var checkpoint, revision, retry sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT id,organization_id,source_id,status,received_at,applied_at,checkpoint,data_revision,errors,lease_until,attempt_count,retry_of FROM bi_import_batches WHERE id=$1`, id).Scan(&item.ID, &item.OrganizationID, &item.SourceID, &item.Status, &item.ReceivedAt, &item.AppliedAt, &checkpoint, &revision, &raw, &item.LeaseUntil, &item.AttemptCount, &retry); errors.Is(err, sql.ErrNoRows) {
		return item, ErrNotFound
	} else if err != nil {
		return item, err
	}
	if checkpoint.Valid {
		item.Checkpoint = &checkpoint.String
	}
	if revision.Valid {
		item.DataRevision = &revision.String
	}
	if retry.Valid {
		item.RetryOf = &retry.String
	}
	_ = json.Unmarshal(raw, &item.Errors)
	item.Status = importStatus(item.Status, item.LeaseUntil, s.now(), item.AttemptCount)
	return item, nil
}

func (h *Handler) AdminGetImport(c *gin.Context) {
	item, err := h.service.adminImport(c.Request.Context(), c.Param("batch_id"))
	adminRespond(c, item, err)
}

func (s *Service) retryImport(ctx context.Context, id string, actorID int64, requestID string) (AdminImport, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminImport{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var org, source, status string
	var payload []byte
	var originalAttempt int
	var lease sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT organization_id,source_id,status,payload,attempt_count,lease_until FROM bi_import_batches WHERE id=$1 FOR UPDATE`, id).Scan(&org, &source, &status, &payload, &originalAttempt, &lease); errors.Is(err, sql.ErrNoRows) {
		return AdminImport{}, ErrNotFound
	} else if err != nil {
		return AdminImport{}, err
	}
	if status != "rejected" && status != "validating" && status != "queued" {
		return AdminImport{}, apiError(409, "CONFLICT", "Only failed or pending imports can be retried")
	}
	if status == "queued" || status == "validating" {
		if lease.Valid && !lease.Time.Before(s.now()) {
			return AdminImport{}, apiError(409, "CONFLICT", "Import is still processing")
		}
	}
	newID := randomToken("batch_retry_")
	newHash := tokenHash(newID + ":" + strconv.FormatInt(time.Now().UnixNano(), 10))
	if _, err := tx.ExecContext(ctx, `INSERT INTO bi_import_batches(id,organization_id,source_id,idempotency_hash,payload_hash,payload,status,retry_of) SELECT $1,organization_id,source_id,$2,payload_hash,payload,'queued',$3 FROM bi_import_batches WHERE id=$4`, newID, newHash, id, id); err != nil {
		return AdminImport{}, err
	}
	if err := insertAdminSecurityEvent(ctx, tx, actorID, "import.retry", newID, org, requestID, map[string]any{"retry_of": id}); err != nil {
		return AdminImport{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminImport{}, err
	}
	return s.adminImport(ctx, newID)
}

func (h *Handler) AdminRetryImport(c *gin.Context, actorID int64) {
	item, err := h.service.retryImport(c.Request.Context(), c.Param("batch_id"), actorID, adminRequestID(c))
	adminRespond(c, item, err)
}

func (s *Service) listAdminQuality(ctx context.Context, org string) ([]AdminQuality, error) {
	if s.db == nil {
		return []AdminQuality{}, nil
	}
	where := ""
	args := []any{}
	if org != "" {
		args = append(args, org)
		where = " WHERE organization_id=$1"
	}
	rows, err := s.db.QueryContext(ctx, `SELECT organization_id,source_id,complete_through,to_char(history_start_date,'YYYY-MM-DD'),initial_backfill_complete,last_applied_at FROM bi_connector_sources`+where+` ORDER BY organization_id,source_id`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := []AdminQuality{}
	for rows.Next() {
		var item AdminQuality
		var history sql.NullString
		var applied sql.NullTime
		var backfill bool
		if err := rows.Scan(&item.OrganizationID, &item.SourceID, &item.CompleteThrough, &history, &backfill, &applied); err != nil {
			return nil, err
		}
		if history.Valid {
			item.HistoryStartDate = &history.String
			item.CoverageStart = &history.String
		}
		if item.CompleteThrough != nil {
			value := item.CompleteThrough.UTC().Format("2006-01-02")
			item.CoverageEnd = &value
		}
		switch {
		case !applied.Valid:
			item.Status = "unknown"
			item.Reason = ptr("Source has not published data")
		case !backfill:
			item.Status = "partial"
			item.Reason = ptr("Historical coverage is incomplete")
		default:
			item.Status = "ready"
		}
		if applied.Valid {
			lag := s.now().UTC().Sub(applied.Time.UTC()).Seconds()
			if lag < 0 {
				lag = 0
			}
			item.LatencySeconds = &lag
		}
		item.MissingDimensions = []string{}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (h *Handler) AdminListQuality(c *gin.Context) {
	items, err := h.service.listAdminQuality(c.Request.Context(), c.Query("organization_id"))
	adminRespond(c, gin.H{"items": items, "as_of": h.service.now().UTC()}, err)
}

func (s *Service) listAdminReports(ctx context.Context, page, size int, org, status string) ([]AdminReport, int64, error) {
	if s.db == nil {
		return []AdminReport{}, 0, nil
	}
	where := " WHERE 1=1"
	args := []any{}
	for _, item := range []struct{ value, key string }{{org, "organization_id"}, {status, "status"}} {
		if item.value != "" {
			args = append(args, item.value)
			where += " AND " + item.key + "=$" + strconv.Itoa(len(args))
		}
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM bi_reports"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, size, (page-1)*size)
	rows, err := s.db.QueryContext(ctx, "SELECT id,organization_id,manager_id,title,status,created_at,expires_at,failure_code,generated_at,lease_until,attempt_count,archived_at,retry_of FROM bi_reports"+where+" ORDER BY created_at DESC,id LIMIT $"+strconv.Itoa(len(args)-1)+" OFFSET $"+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := []AdminReport{}
	for rows.Next() {
		var item AdminReport
		var failure, retry sql.NullString
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.ManagerID, &item.Title, &item.Status, &item.CreatedAt, &item.ExpiresAt, &failure, &item.GeneratedAt, &item.LeaseUntil, &item.AttemptCount, &item.ArchivedAt, &retry); err != nil {
			return nil, 0, err
		}
		if failure.Valid {
			item.FailureCode = &failure.String
		}
		if retry.Valid {
			item.RetryOf = &retry.String
		}
		item.Status = reportStatus(item.Status, item.LeaseUntil, s.now())
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (h *Handler) AdminListReports(c *gin.Context) {
	page, size := adminPagination(c)
	items, total, err := h.service.listAdminReports(c.Request.Context(), page, size, c.Query("organization_id"), c.Query("status"))
	adminRespond(c, gin.H{"items": items, "total": total, "page": page, "page_size": size}, err)
}

func (s *Service) adminReport(ctx context.Context, id string) (AdminReport, error) {
	var item AdminReport
	var failure, retry sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT id,organization_id,manager_id,title,status,created_at,expires_at,failure_code,generated_at,lease_until,attempt_count,archived_at,retry_of FROM bi_reports WHERE id=$1`, id).Scan(&item.ID, &item.OrganizationID, &item.ManagerID, &item.Title, &item.Status, &item.CreatedAt, &item.ExpiresAt, &failure, &item.GeneratedAt, &item.LeaseUntil, &item.AttemptCount, &item.ArchivedAt, &retry); errors.Is(err, sql.ErrNoRows) {
		return item, ErrNotFound
	} else if err != nil {
		return item, err
	}
	if failure.Valid {
		item.FailureCode = &failure.String
	}
	if retry.Valid {
		item.RetryOf = &retry.String
	}
	item.Status = reportStatus(item.Status, item.LeaseUntil, s.now())
	return item, nil
}

func (h *Handler) AdminGetReport(c *gin.Context) {
	item, err := h.service.adminReport(c.Request.Context(), c.Param("report_id"))
	adminRespond(c, item, err)
}

func (s *Service) archiveReport(ctx context.Context, id string, actorID int64, requestID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var org string
	if err := tx.QueryRowContext(ctx, `UPDATE bi_reports SET status='archived',archived_at=NOW(),archived_by=$2,lease_owner=NULL,lease_until=NULL WHERE id=$1 AND status<>'archived' RETURNING organization_id`, id, actorID).Scan(&org); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if err := insertAdminSecurityEvent(ctx, tx, actorID, "report.archive", id, org, requestID, nil); err != nil {
		return err
	}
	return tx.Commit()
}

func (h *Handler) AdminArchiveReport(c *gin.Context, actorID int64) {
	err := h.service.archiveReport(c.Request.Context(), c.Param("report_id"), actorID, adminRequestID(c))
	adminRespond(c, gin.H{"archived": err == nil}, err)
}

func (s *Service) retryReport(ctx context.Context, id string, actorID int64, requestID string) (AdminReport, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AdminReport{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var org, manager, title, status string
	var snapshot, scope, sources []byte
	var revision sql.NullInt64
	var leaseUntil sql.NullTime
	var expires time.Time
	if err := tx.QueryRowContext(ctx, `SELECT organization_id,manager_id,title,status,snapshot,scope,source_state,data_revision,expires_at,lease_until FROM bi_reports WHERE id=$1 FOR UPDATE`, id).Scan(&org, &manager, &title, &status, &snapshot, &scope, &sources, &revision, &expires, &leaseUntil); errors.Is(err, sql.ErrNoRows) {
		return AdminReport{}, ErrNotFound
	} else if err != nil {
		return AdminReport{}, err
	}
	if status != "failed" && status != "running" {
		return AdminReport{}, apiError(409, "CONFLICT", "Only failed or timed-out reports can be retried")
	}
	if status == "running" && leaseUntil.Valid && leaseUntil.Time.After(s.now()) {
		return AdminReport{}, apiError(409, "CONFLICT", "Report is still processing")
	}
	newID := randomToken("report_retry_")
	var dataRevision any
	if revision.Valid {
		dataRevision = revision.Int64
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO bi_reports(id,organization_id,manager_id,title,status,created_at,expires_at,snapshot,scope,source_state,data_revision,retry_of) VALUES($1,$2,$3,$4,'queued',NOW(),$5,$6,$7,$8,$9,$10)`, newID, org, manager, title, expires, string(snapshot), string(scope), string(sources), dataRevision, id); err != nil {
		return AdminReport{}, err
	}
	if err := insertAdminSecurityEvent(ctx, tx, actorID, "report.retry", newID, org, requestID, map[string]any{"retry_of": id}); err != nil {
		return AdminReport{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminReport{}, err
	}
	return s.adminReport(ctx, newID)
}

func (h *Handler) AdminRetryReport(c *gin.Context, actorID int64) {
	item, err := h.service.retryReport(c.Request.Context(), c.Param("report_id"), actorID, adminRequestID(c))
	adminRespond(c, item, err)
}

func (s *Service) cleanupRetention(ctx context.Context) (int64, error) {
	policy, err := s.retentionPolicy(ctx)
	if err != nil {
		return 0, err
	}
	now := s.now().UTC()
	var total int64
	statements := []struct {
		query string
		args  []any
	}{{`DELETE FROM bi_refresh_tokens WHERE expires_at <= $1`, []any{now}}, {`DELETE FROM bi_sessions WHERE expires_at <= $1 AND NOT EXISTS(SELECT 1 FROM bi_refresh_tokens r WHERE r.session_id=bi_sessions.id)`, []any{now}}, {`DELETE FROM bi_binding_challenges WHERE expires_at <= $1`, []any{now.AddDate(0, 0, -policy.EphemeralDays)}}, {`DELETE FROM bi_wechat_codes WHERE created_at <= $1`, []any{now.AddDate(0, 0, -policy.EphemeralDays)}}, {`DELETE FROM bi_list_snapshots WHERE expires_at <= $1`, []any{now}}, {`DELETE FROM bi_analysis_contexts WHERE expires_at <= $1`, []any{now}}, {`DELETE FROM bi_command_receipts WHERE expires_at <= $1`, []any{now}}, {`DELETE FROM bi_report_shares WHERE expires_at <= $1`, []any{now}}, {`DELETE FROM bi_report_shares WHERE report_id IN (SELECT id FROM bi_reports WHERE expires_at <= $1)`, []any{now.AddDate(0, policy.ReportMonths*-1, 0)}}, {`DELETE FROM bi_reports WHERE expires_at <= $1`, []any{now.AddDate(0, policy.ReportMonths*-1, 0)}}, {`DELETE FROM bi_security_events WHERE created_at <= $1`, []any{now.AddDate(0, 0, -policy.AuditDays)}}, {`DELETE FROM bi_usage_facts WHERE occurred_at <= $1`, []any{now.AddDate(0, policy.FactMonths*-1, 0)}}}
	for _, statement := range statements {
		result, err := s.db.ExecContext(ctx, statement.query, statement.args...)
		if err != nil {
			return total, err
		}
		count, _ := result.RowsAffected()
		total += count
	}
	return total, nil
}

func (h *Handler) AdminCleanupStatus(c *gin.Context) {
	policy, err := h.service.retentionPolicy(c.Request.Context())
	result := AdminCleanup{Retention: policy}
	if err == nil && h.service.db != nil {
		var lastRun time.Time
		var deleted int64
		if queryErr := h.service.db.QueryRowContext(c.Request.Context(), `SELECT finished_at,deleted_count FROM bi_cleanup_runs WHERE status='succeeded' AND finished_at IS NOT NULL ORDER BY finished_at DESC,id DESC LIMIT 1`).Scan(&lastRun, &deleted); queryErr == nil {
			result.LastRunAt = &lastRun
			result.LastDeleted = deleted
		} else if !errors.Is(queryErr, sql.ErrNoRows) {
			err = queryErr
		}
	}
	adminRespond(c, result, err)
}
func (h *Handler) AdminRunCleanup(c *gin.Context, actorID int64) {
	started := h.service.now().UTC()
	count, err := h.service.cleanupRetention(c.Request.Context())
	status := "succeeded"
	if err != nil {
		status = "failed"
	}
	if h.service.db != nil {
		_, recordErr := h.service.db.ExecContext(c.Request.Context(), `INSERT INTO bi_cleanup_runs(started_at,finished_at,status,deleted_count,error_message,actor_user_id) VALUES($1,$2,$3,$4,$5,$6)`, started, h.service.now().UTC(), status, count, errorMessage(err), actorID)
		if err == nil {
			err = recordErr
		}
	}
	if err == nil {
		err = h.service.insertSecurityEvent(c.Request.Context(), actorID, "cleanup.run", "bi_retention", "", adminRequestID(c), map[string]any{"deleted": count})
	}
	adminRespond(c, gin.H{"deleted": count}, err)
}

func errorMessage(err error) *string {
	if err == nil {
		return nil
	}
	value := err.Error()
	return &value
}

func (s *Service) listAdminAudit(ctx context.Context, page, size int, action, requestID string) ([]AdminAuditEvent, int64, error) {
	if s.db == nil {
		return []AdminAuditEvent{}, 0, nil
	}
	where := " WHERE 1=1"
	args := []any{}
	for _, item := range []struct{ value, key string }{{action, "action"}, {requestID, "request_id"}} {
		if item.value != "" {
			args = append(args, item.value)
			where += " AND " + item.key + "=$" + strconv.Itoa(len(args))
		}
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM bi_security_events"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, size, (page-1)*size)
	rows, err := s.db.QueryContext(ctx, "SELECT id,actor_user_id,organization_id,action,target_id,request_id,metadata,created_at FROM bi_security_events"+where+" ORDER BY created_at DESC,id DESC LIMIT $"+strconv.Itoa(len(args)-1)+" OFFSET $"+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := []AdminAuditEvent{}
	for rows.Next() {
		var item AdminAuditEvent
		var metadata []byte
		var actor sql.NullInt64
		var org sql.NullString
		if err := rows.Scan(&item.ID, &actor, &org, &item.Action, &item.TargetID, &item.RequestID, &metadata, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		if actor.Valid {
			item.ActorUserID = &actor.Int64
		}
		if org.Valid {
			item.OrganizationID = &org.String
		}
		if len(metadata) > 0 {
			_ = json.Unmarshal(metadata, &item.Metadata)
		}
		if item.Metadata == nil {
			item.Metadata = map[string]any{}
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (h *Handler) AdminListAudit(c *gin.Context) {
	page, size := adminPagination(c)
	items, total, err := h.service.listAdminAudit(c.Request.Context(), page, size, c.Query("action"), c.Query("request_id"))
	adminRespond(c, gin.H{"items": items, "total": total, "page": page, "page_size": size}, err)
}

func (s *Service) insertSecurityEvent(ctx context.Context, actorID int64, action, target, org, requestID string, metadata map[string]any) error {
	return insertAdminSecurityEvent(ctx, s.db, actorID, action, target, org, requestID, metadata)
}

func insertAdminSecurityEvent(ctx context.Context, db adminExec, actorID int64, action, target, org, requestID string, metadata map[string]any) error {
	if db == nil {
		return fmt.Errorf("BI database is unavailable")
	}
	raw, _ := json.Marshal(metadata)
	_, err := db.ExecContext(ctx, `INSERT INTO bi_security_events(actor_user_id,organization_id,action,target_id,request_id,metadata) VALUES($1,NULLIF($2,''),$3,$4,$5,$6)`, actorID, org, action, target, requestID, string(raw))
	return err
}
