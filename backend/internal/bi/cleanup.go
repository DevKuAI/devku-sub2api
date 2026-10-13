package bi

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type Retention struct {
	AuditDays     int `json:"audit_days"`
	EphemeralDays int `json:"ephemeral_days"`
}

func (s *Service) retention(ctx context.Context) (Retention, error) {
	v := Retention{AuditDays: 180, EphemeralDays: 7}
	if s.db == nil {
		return v, nil
	}
	e := s.db.QueryRowContext(ctx, `SELECT audit_days,ephemeral_days FROM bi_retention_policies WHERE id=TRUE`).Scan(&v.AuditDays, &v.EphemeralDays)
	return v, e
}
func (s *Service) cleanup(ctx context.Context) (int64, error) {
	policy, e := s.retention(ctx)
	if e != nil {
		return 0, e
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return 0, e
	}
	defer func() { _ = tx.Rollback() }()
	now := s.now().UTC()
	var total int64
	for _, item := range []struct {
		query  string
		before time.Time
	}{
		{`DELETE FROM bi_refresh_tokens WHERE token_hash IN(SELECT token_hash FROM bi_refresh_tokens WHERE expires_at <= $1 LIMIT 1000)`, now},
		{`DELETE FROM bi_sessions WHERE id IN(SELECT id FROM bi_sessions WHERE expires_at <= $1 AND NOT EXISTS(SELECT 1 FROM bi_refresh_tokens r WHERE r.session_id=bi_sessions.id) LIMIT 1000)`, now},
		{`DELETE FROM bi_invitations WHERE id IN(SELECT id FROM bi_invitations WHERE expires_at<$1 LIMIT 1000)`, now.AddDate(0, 0, -policy.EphemeralDays)},
		{`DELETE FROM bi_cleanup_runs WHERE id IN(SELECT id FROM bi_cleanup_runs WHERE finished_at<$1 LIMIT 1000)`, now.AddDate(0, 0, -policy.AuditDays)},
		{`DELETE FROM bi_wechat_codes WHERE code_hash IN(SELECT code_hash FROM bi_wechat_codes WHERE created_at<$1 LIMIT 1000)`, now.AddDate(0, 0, -policy.EphemeralDays)},
		{`DELETE FROM bi_security_events WHERE id IN(SELECT id FROM bi_security_events WHERE created_at<$1 LIMIT 1000)`, now.AddDate(0, 0, -policy.AuditDays)},
	} {
		r, err := tx.ExecContext(ctx, item.query, item.before)
		if err != nil {
			return total, err
		}
		n, _ := r.RowsAffected()
		total += n
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO bi_cleanup_runs(deleted_count,status) VALUES($1,'succeeded')`, total); e != nil {
		return total, e
	}
	return total, tx.Commit()
}
func (h *Handler) Start() {
	h.workerMu.Lock()
	defer h.workerMu.Unlock()
	if h.service.db == nil || h.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	h.done = make(chan struct{})
	go func() {
		defer close(h.done)
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanupCtx, stop := context.WithTimeout(ctx, 20*time.Second)
				if _, err := h.service.cleanup(cleanupCtx); err != nil {
					slog.ErrorContext(cleanupCtx, "BI identity cleanup failed", "error_type", fmt.Sprintf("%T", err))
				}
				stop()
			}
		}
	}()
}
func (h *Handler) Stop() {
	h.workerMu.Lock()
	defer h.workerMu.Unlock()
	if h.cancel != nil {
		h.cancel()
		<-h.done
	}
}
func (h *Handler) GetRetention(c *gin.Context) {
	v, e := h.service.retention(c.Request.Context())
	adminRespond(c, v, e)
}
func (h *Handler) SaveRetention(c *gin.Context) {
	var in Retention
	if !decodeRequest(c, &in) {
		return
	}
	if in.AuditDays < 1 || in.AuditDays > 3650 || in.EphemeralDays < 1 || in.EphemeralDays > 365 {
		WriteError(c, invalid("retention", "Invalid retention period"))
		return
	}
	ctx := c.Request.Context()
	tx, e := h.service.db.BeginTx(ctx, nil)
	if e != nil {
		WriteError(c, e)
		return
	}
	defer func() { _ = tx.Rollback() }()
	_, e = tx.ExecContext(ctx, `UPDATE bi_retention_policies SET audit_days=$1,ephemeral_days=$2 WHERE id=TRUE`, in.AuditDays, in.EphemeralDays)
	if e == nil {
		e = h.service.audit(ctx, tx, "web_user", strconv.FormatInt(actor(c), 10), 0, "retention.update", "bi_identity", c.GetString("bi.request_id"))
	}
	if e == nil {
		e = tx.Commit()
	}
	adminRespond(c, in, e)
}
func (h *Handler) CleanupStatus(c *gin.Context) {
	var finished *time.Time
	var deleted int64
	e := h.service.db.QueryRowContext(c.Request.Context(), `SELECT finished_at,deleted_count FROM bi_cleanup_runs ORDER BY id DESC LIMIT 1`).Scan(&finished, &deleted)
	if e == sql.ErrNoRows {
		e = nil
	}
	adminRespond(c, gin.H{"last_run_at": finished, "deleted_count": deleted}, e)
}
func (h *Handler) RunCleanup(c *gin.Context) {
	n, e := h.service.cleanup(c.Request.Context())
	if e == nil {
		e = h.service.audit(c.Request.Context(), h.service.db, "web_user", strconv.FormatInt(actor(c), 10), 0, "cleanup.run", "bi_identity", c.GetString("bi.request_id"))
	}
	adminRespond(c, gin.H{"deleted_count": n}, e)
}
