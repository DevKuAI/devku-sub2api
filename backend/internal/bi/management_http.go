package bi

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func (h *Handler) CreateInvitation(c *gin.Context) {
	var in struct {
		Capabilities []string `json:"capabilities"`
		Note         string   `json:"note"`
	}
	if !decodeRequest(c, &in) {
		return
	}
	v, e := h.service.CreateInvitation(c.Request.Context(), managedOrganization(c), in.Capabilities, in.Note, actor(c), c.GetString("bi.request_id"))
	adminRespond(c, v, e)
}
func (h *Handler) ListInvitations(c *gin.Context) {
	page, size, ok := queryPage(c)
	if !ok {
		return
	}
	org := managedOrganization(c)
	ctx := c.Request.Context()
	var total int64
	err := h.service.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bi_invitations i JOIN desktop_organizations o ON o.id=i.organization_id WHERE o.public_id=$1`, org).Scan(&total)
	if err != nil {
		WriteError(c, err)
		return
	}
	rows, err := h.service.db.QueryContext(ctx, `SELECT `+invitationColumns+` FROM bi_invitations i JOIN desktop_organizations o ON o.id=i.organization_id WHERE o.public_id=$1 ORDER BY i.created_at DESC,i.id LIMIT $2 OFFSET $3`, org, size, (page-1)*size)
	if err != nil {
		WriteError(c, err)
		return
	}
	defer func() { _ = rows.Close() }()
	items := []Invitation{}
	for rows.Next() {
		v, e := scanInvitation(rows)
		if e != nil {
			WriteError(c, e)
			return
		}
		if v.Status == "pending" && !v.ExpiresAt.After(h.service.now()) {
			v.Status = "expired"
		}
		items = append(items, v)
	}
	adminRespond(c, pageResult(items, page, size, total), rows.Err())
}
func (h *Handler) RevokeInvitation(c *gin.Context) {
	ctx := c.Request.Context()
	tx, e := h.service.db.BeginTx(ctx, nil)
	if e != nil {
		WriteError(c, e)
		return
	}
	defer func() { _ = tx.Rollback() }()
	var org int64
	e = tx.QueryRowContext(ctx, `UPDATE bi_invitations i SET status='revoked' FROM desktop_organizations o WHERE o.id=i.organization_id AND o.public_id=$1 AND i.id=$2 AND i.status='pending' RETURNING o.id`, managedOrganization(c), c.Param("invitation_id")).Scan(&org)
	if e == sql.ErrNoRows {
		adminRespond(c, gin.H{"ok": true}, nil)
		return
	}
	if e != nil {
		WriteError(c, e)
		return
	}
	e = h.service.audit(ctx, tx, "web_user", strconv.FormatInt(actor(c), 10), org, "invitation.revoke", c.Param("invitation_id"), c.GetString("bi.request_id"))
	if e == nil {
		e = tx.Commit()
	}
	adminRespond(c, gin.H{"ok": true}, e)
}
func (h *Handler) ListAccess(c *gin.Context) {
	page, size, ok := queryPage(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	org := managedOrganization(c)
	var total int64
	err := h.service.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bi_access a JOIN desktop_organizations o ON o.id=a.organization_id WHERE o.public_id=$1`, org).Scan(&total)
	if err != nil {
		WriteError(c, err)
		return
	}
	rows, err := h.service.db.QueryContext(ctx, `SELECT u.id,u.display_name,u.status,a.capabilities,a.status,a.revision,a.updated_at,(SELECT MAX(last_seen_at) FROM bi_sessions s WHERE s.bi_user_id=u.id) FROM bi_access a JOIN bi_users u ON u.id=a.bi_user_id JOIN desktop_organizations o ON o.id=a.organization_id WHERE o.public_id=$1 ORDER BY u.display_name,u.id LIMIT $2 OFFSET $3`, org, size, (page-1)*size)
	if err != nil {
		WriteError(c, err)
		return
	}
	defer func() { _ = rows.Close() }()
	items := []gin.H{}
	for rows.Next() {
		var id, name, userStatus, status string
		var raw []byte
		var revision int64
		var updated time.Time
		var last *time.Time
		if err = rows.Scan(&id, &name, &userStatus, &raw, &status, &revision, &updated, &last); err != nil {
			WriteError(c, err)
			return
		}
		var caps []string
		if err = json.Unmarshal(raw, &caps); err != nil {
			WriteError(c, err)
			return
		}
		items = append(items, gin.H{"bi_user_id": id, "display_name": name, "user_status": userStatus, "status": status, "scope": "organization", "capabilities": caps, "revision": revision, "updated_at": updated, "last_seen_at": last})
	}
	adminRespond(c, pageResult(items, page, size, total), rows.Err())
}
func (h *Handler) UpdateAccess(c *gin.Context) {
	var in struct {
		Capabilities []string `json:"capabilities"`
		Status       string   `json:"status"`
		Revision     *int64   `json:"expected_revision"`
	}
	if !decodeRequest(c, &in) {
		return
	}
	if in.Revision == nil || *in.Revision < 1 {
		WriteError(c, invalid("expected_revision", "Current revision required"))
		return
	}
	v, e := h.service.UpdateAccess(c.Request.Context(), managedOrganization(c), c.Param("bi_user_id"), in.Capabilities, in.Status, *in.Revision, actor(c), c.GetString("bi.request_id"))
	adminRespond(c, v, e)
}
func (h *Handler) SetUserStatus(c *gin.Context) {
	var in struct {
		Status string `json:"status"`
	}
	if !decodeRequest(c, &in) {
		return
	}
	e := h.service.SetUserStatus(c.Request.Context(), c.Param("bi_user_id"), in.Status, actor(c), c.GetString("bi.request_id"))
	adminRespond(c, gin.H{"ok": true}, e)
}
func (h *Handler) OperationsOverview(c *gin.Context) {
	counts := gin.H{}
	for name, query := range map[string]string{"users": `SELECT COUNT(*) FROM bi_users WHERE status='active'`, "invitations": `SELECT COUNT(*) FROM bi_invitations WHERE status='pending' AND expires_at>NOW()`, "sessions": `SELECT COUNT(*) FROM bi_sessions s JOIN bi_users u ON u.id=s.bi_user_id WHERE s.revoked_at IS NULL AND s.expires_at>NOW() AND u.status='active' AND u.auth_version=s.auth_version`, "access": `SELECT COUNT(*) FROM bi_access WHERE status='active'`, "daily_running": `SELECT COUNT(*) FROM desktop_daily_report_tasks WHERE status IN ('pending','waiting','running')`, "period_running": `SELECT COUNT(*) FROM desktop_period_report_tasks WHERE status IN ('pending','waiting','running')`} {
		var n int64
		if h.service.db != nil {
			if e := h.service.db.QueryRowContext(c.Request.Context(), query).Scan(&n); e != nil {
				WriteError(c, e)
				return
			}
		}
		counts[name] = n
	}
	published := false
	if h.settings != nil {
		notice, e := h.settings.GetBIPrivacyNotice(c.Request.Context())
		published = e == nil && notice.Published() && notice.URL != ""
	}
	adminRespond(c, gin.H{"bi_enabled": h.service.config.Enabled, "desktop_enabled": h.desktopEnabled, "config": gin.H{"appid": h.service.config.AppID, "app_secret_configured": h.service.config.AppSecret != "", "jwt_secret_configured": h.service.config.JWTSecret != "", "identity_secret_configured": h.service.config.IdentitySecret != "", "min_client_version": h.service.config.MinClientVersion}, "privacy_notice_published": published, "identity_mode": "independent_wechat", "capabilities": Capabilities, "counts": counts}, nil)
}
func (h *Handler) OperationsOrganizations(c *gin.Context) {
	page, size, ok := queryPage(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	var total int64
	e := h.service.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM desktop_organizations WHERE deleted_at IS NULL AND ($1='' OR strpos(lower(name),lower($1))>0)`, c.Query("search")).Scan(&total)
	if e != nil {
		WriteError(c, e)
		return
	}
	rows, e := h.service.db.QueryContext(ctx, `SELECT o.public_id,o.name,o.status,o.conversation_reporting_enabled,o.conversation_summary_enabled,o.analysis_model,o.weekly_summary_enabled,o.monthly_summary_enabled,(SELECT COUNT(*) FROM bi_access a WHERE a.organization_id=o.id AND a.status='active'),(SELECT MAX(c.received_at) FROM desktop_conversation_records c WHERE c.organization_id=o.id) FROM desktop_organizations o WHERE o.deleted_at IS NULL AND ($1='' OR strpos(lower(o.name),lower($1))>0) ORDER BY o.name,o.id LIMIT $2 OFFSET $3`, c.Query("search"), size, (page-1)*size)
	if e != nil {
		WriteError(c, e)
		return
	}
	defer func() { _ = rows.Close() }()
	items := []gin.H{}
	for rows.Next() {
		var id, name, status, model string
		var reporting, summary, week, month bool
		var users int64
		var last *time.Time
		if e = rows.Scan(&id, &name, &status, &reporting, &summary, &model, &week, &month, &users, &last); e != nil {
			WriteError(c, e)
			return
		}
		items = append(items, gin.H{"id": id, "name": name, "status": status, "reporting_enabled": reporting, "summary_enabled": summary, "weekly_summary_enabled": week, "monthly_summary_enabled": month, "analysis_model": model, "users": users, "last_received_at": last})
	}
	adminRespond(c, pageResult(items, page, size, total), rows.Err())
}
func (h *Handler) OperationsUsers(c *gin.Context) {
	page, size, ok := queryPage(c)
	if !ok {
		return
	}
	rows, e := h.service.db.QueryContext(c.Request.Context(), `SELECT u.id,u.display_name,u.status,u.auth_version,u.created_at,(SELECT MAX(s.last_seen_at) FROM bi_sessions s WHERE s.bi_user_id=u.id) FROM bi_users u ORDER BY u.created_at DESC,u.id LIMIT $1 OFFSET $2`, size, (page-1)*size)
	if e != nil {
		WriteError(c, e)
		return
	}
	defer func() { _ = rows.Close() }()
	items := []gin.H{}
	for rows.Next() {
		var id, name, status string
		var version int64
		var created time.Time
		var last *time.Time
		if e = rows.Scan(&id, &name, &status, &version, &created, &last); e != nil {
			WriteError(c, e)
			return
		}
		items = append(items, gin.H{"id": id, "display_name": name, "status": status, "authorization_version": version, "created_at": created, "last_seen_at": last})
	}
	if e = rows.Err(); e != nil {
		WriteError(c, e)
		return
	}
	var total int64
	e = h.service.db.QueryRowContext(c.Request.Context(), `SELECT COUNT(*) FROM bi_users`).Scan(&total)
	adminRespond(c, pageResult(items, page, size, total), e)
}
func (h *Handler) OperationsAudit(c *gin.Context) {
	page, size, ok := queryPage(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	filter := ` FROM bi_security_events e LEFT JOIN desktop_organizations o ON o.id=e.organization_id WHERE ($1='' OR o.public_id=$1) AND ($2='' OR e.request_id=$2)`
	args := []any{c.Query("organization_id"), c.Query("request_id")}
	var total int64
	if e := h.service.db.QueryRowContext(ctx, `SELECT COUNT(*)`+filter, args...).Scan(&total); e != nil {
		WriteError(c, e)
		return
	}
	args = append(args, size, (page-1)*size)
	rows, e := h.service.db.QueryContext(ctx, `SELECT e.id,e.actor_type,e.actor_id,o.public_id,e.action,e.target_id,e.request_id,e.created_at`+filter+` ORDER BY e.id DESC LIMIT $3 OFFSET $4`, args...)
	if e != nil {
		WriteError(c, e)
		return
	}
	defer func() { _ = rows.Close() }()
	items := []gin.H{}
	for rows.Next() {
		var id int64
		var actorType, actorID, action, target, request string
		var org *string
		var created time.Time
		if e = rows.Scan(&id, &actorType, &actorID, &org, &action, &target, &request, &created); e != nil {
			WriteError(c, e)
			return
		}
		items = append(items, gin.H{"id": id, "actor_type": actorType, "actor_id": actorID, "organization_id": org, "action": action, "target_id": target, "request_id": request, "created_at": created})
	}
	adminRespond(c, pageResult(items, page, size, total), rows.Err())
}
