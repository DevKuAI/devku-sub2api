package bi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type sqlExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (s *Service) audit(ctx context.Context, q sqlExecutor, actorType, actor string, org int64, action, target, requestID string) error {
	var organization any
	if org > 0 {
		organization = org
	}
	_, err := q.ExecContext(ctx, `INSERT INTO bi_security_events(actor_type,actor_id,organization_id,action,target_id,request_id) VALUES($1,$2,$3,$4,$5,$6)`, actorType, actor, organization, action, target, requestID)
	return err
}
func validateCapabilities(caps []string) error {
	if len(caps) == 0 || len(caps) > len(Capabilities) {
		return invalid("capabilities", "Select at least one supported capability")
	}
	for _, c := range caps {
		if !slices.Contains(Capabilities, c) {
			return invalid("capabilities", "Unknown capability")
		}
	}
	for c, deps := range map[string][]string{"conversations:read": {"members:read"}, "reports:read": {"members:read"}, "reports:copy": {"members:read", "reports:read"}} {
		if slices.Contains(caps, c) {
			for _, dep := range deps {
				if !slices.Contains(caps, dep) {
					return invalid("capabilities", c+" requires "+dep)
				}
			}
		}
	}
	return nil
}
func (s *Service) AuthorizeOrganization(ctx context.Context, p Principal, org string, caps ...string) (Access, error) {
	var a Access
	var raw []byte
	err := s.db.QueryRowContext(ctx, `SELECT a.bi_user_id,u.display_name,o.public_id,o.name,a.capabilities,a.status,a.revision,a.organization_alias,o.id
 FROM bi_access a JOIN bi_users u ON u.id=a.bi_user_id JOIN desktop_organizations o ON o.id=a.organization_id
 WHERE a.bi_user_id=$1 AND o.public_id=$2 AND a.status='active' AND u.status='active' AND o.status='active' AND o.deleted_at IS NULL AND ($3='' OR EXISTS(SELECT 1 FROM bi_sessions sess WHERE sess.id=$3 AND sess.bi_user_id=u.id AND sess.auth_version=u.auth_version AND sess.revoked_at IS NULL AND sess.expires_at>$4))`, p.UserID, org, p.SessionID, s.now()).Scan(&a.UserID, &a.DisplayName, &a.OrganizationID, &a.Name, &raw, &a.Status, &a.Revision, &a.OrganizationAlias, &a.InternalOrganizationID)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	if err != nil {
		return a, err
	}
	if err = json.Unmarshal(raw, &a.Capabilities); err != nil {
		return a, err
	}
	a.Scope = "organization"
	for _, cap := range caps {
		if !slices.Contains(a.Capabilities, cap) {
			return a, ErrForbidden
		}
	}
	return a, nil
}
func (s *Service) Organizations(ctx context.Context, p Principal, page, size int) (Page[Access], error) {
	where := ` FROM bi_access a JOIN bi_users u ON u.id=a.bi_user_id JOIN desktop_organizations o ON o.id=a.organization_id WHERE a.bi_user_id=$1 AND a.status='active' AND u.status='active' AND o.status='active' AND o.deleted_at IS NULL`
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*)`+where, p.UserID).Scan(&total); err != nil {
		return Page[Access]{}, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT u.id,u.display_name,o.public_id,o.name,a.capabilities,a.status,a.revision,a.organization_alias`+where+` ORDER BY o.name,o.public_id LIMIT $2 OFFSET $3`, p.UserID, size, (page-1)*size)
	if err != nil {
		return Page[Access]{}, err
	}
	defer func() { _ = rows.Close() }()
	items := []Access{}
	for rows.Next() {
		var a Access
		var raw []byte
		if err = rows.Scan(&a.UserID, &a.DisplayName, &a.OrganizationID, &a.Name, &raw, &a.Status, &a.Revision, &a.OrganizationAlias); err != nil {
			return Page[Access]{}, err
		}
		if err = json.Unmarshal(raw, &a.Capabilities); err != nil {
			return Page[Access]{}, err
		}
		a.Scope = "organization"
		items = append(items, a)
	}
	return pageResult(items, page, size, total), rows.Err()
}
func (s *Service) CreateInvitation(ctx context.Context, org string, caps []string, note string, actor int64, requestID string, recipientNames ...string) (IssuedInvitation, error) {
	result := IssuedInvitation{}
	recipient := ""
	if len(recipientNames) > 0 {
		recipient = strings.TrimSpace(recipientNames[0])
	}
	if utf8.RuneCountInString(recipient) > 80 {
		return result, invalid("recipient_name", "Recipient name exceeds 80 characters")
	}
	if err := validateCapabilities(caps); err != nil {
		return result, err
	}
	if utf8.RuneCountInString(note) > 100 {
		return result, invalid("note", "Note must not exceed 100 characters")
	}
	caps = slices.Clone(caps)
	slices.Sort(caps)
	caps = slices.Compact(caps)
	raw, _ := json.Marshal(caps)
	token := randomToken("bii_")
	id := randomToken("biv_")
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback() }()
	var internalID int64
	var name string
	err = tx.QueryRowContext(ctx, `SELECT o.id,o.name FROM desktop_organizations o JOIN users issuer ON issuer.id=$2 WHERE o.public_id=$1 AND o.status='active' AND o.deleted_at IS NULL AND issuer.status='active' AND issuer.deleted_at IS NULL AND (issuer.role='admin' OR o.gateway_user_id=issuer.id) FOR SHARE OF o,issuer`, org, actor).Scan(&internalID, &name)
	if errors.Is(err, sql.ErrNoRows) {
		return result, ErrNotFound
	}
	if err != nil {
		return result, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO bi_invitations(id,organization_id,token_hash,capabilities,note,expires_at,created_by,recipient_name) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, internalID, tokenHash(token), string(raw), strings.TrimSpace(note), now.Add(InvitationTTL), actor, recipient)
	if err != nil {
		return result, err
	}
	if err = s.audit(ctx, tx, "web_user", strconv.FormatInt(actor, 10), internalID, "invitation.create", id, requestID); err != nil {
		return result, err
	}
	if err = tx.Commit(); err != nil {
		return result, err
	}
	result.Invitation = Invitation{ID: id, OrganizationID: org, OrganizationName: name, Capabilities: caps, Note: note, RecipientName: recipient, Status: "pending", ExpiresAt: now.Add(InvitationTTL), CreatedAt: now}
	result.Token = token
	result.Path = "/pages/settings/index?invite=" + token
	return result, nil
}
func scanInvitation(row interface{ Scan(...any) error }) (Invitation, error) {
	var inv Invitation
	var raw []byte
	err := row.Scan(&inv.ID, &inv.OrganizationID, &inv.OrganizationName, &raw, &inv.Note, &inv.Status, &inv.ExpiresAt, &inv.CreatedAt, &inv.ClaimedBy, &inv.ClaimedAt, &inv.RecipientName)
	if err != nil {
		return inv, err
	}
	err = json.Unmarshal(raw, &inv.Capabilities)
	return inv, err
}

const invitationColumns = `i.id,o.public_id,o.name,i.capabilities,i.note,i.status,i.expires_at,i.created_at,i.claimed_by,i.claimed_at,i.recipient_name`

func (s *Service) PreviewInvitation(ctx context.Context, p Principal, token string) (Invitation, error) {
	if len(token) < 32 || len(token) > 128 {
		return Invitation{}, ErrNotFound
	}
	inv, err := scanInvitation(s.db.QueryRowContext(ctx, `SELECT `+invitationColumns+` FROM bi_invitations i JOIN desktop_organizations o ON o.id=i.organization_id
 JOIN users issuer ON issuer.id=i.created_by WHERE i.token_hash=$1 AND o.status='active' AND o.deleted_at IS NULL
 AND issuer.status='active' AND issuer.deleted_at IS NULL AND (issuer.role='admin' OR o.gateway_user_id=issuer.id)`, tokenHash(token)))
	if errors.Is(err, sql.ErrNoRows) {
		return inv, ErrNotFound
	}
	if err != nil {
		return inv, err
	}
	if inv.Status == "used" && inv.ClaimedBy != nil && *inv.ClaimedBy == p.UserID {
		return inv, nil
	}
	if inv.Status != "pending" || !inv.ExpiresAt.After(s.now()) {
		return inv, ErrNotFound
	}
	return inv, nil
}
func (s *Service) AcceptInvitation(ctx context.Context, p Principal, token, requestID string) (Access, error) {
	if len(token) < 32 || len(token) > 128 {
		return Access{}, ErrNotFound
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Access{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var active bool
	err = tx.QueryRowContext(ctx, `SELECT status='active' FROM bi_users WHERE id=$1 FOR SHARE`, p.UserID).Scan(&active)
	if err != nil {
		return Access{}, err
	}
	if !active {
		return Access{}, ErrForbidden
	}
	var orgID int64
	var org string
	var caps []byte
	var expires time.Time
	var status, id, recipient string
	var claimed *string
	err = tx.QueryRowContext(ctx, `SELECT i.id,i.organization_id,o.public_id,i.capabilities,i.expires_at,i.status,i.claimed_by,i.recipient_name
 FROM bi_invitations i JOIN desktop_organizations o ON o.id=i.organization_id JOIN users issuer ON issuer.id=i.created_by
 WHERE i.token_hash=$1 AND o.status='active' AND o.deleted_at IS NULL AND issuer.status='active' AND issuer.deleted_at IS NULL
 AND (issuer.role='admin' OR o.gateway_user_id=issuer.id) FOR UPDATE OF i FOR SHARE OF o,issuer`, tokenHash(token)).Scan(&id, &orgID, &org, &caps, &expires, &status, &claimed, &recipient)
	if errors.Is(err, sql.ErrNoRows) {
		return Access{}, ErrNotFound
	}
	if err != nil {
		return Access{}, err
	}
	if status == "used" && claimed != nil && *claimed == p.UserID {
		_ = tx.Rollback()
		return s.AuthorizeOrganization(ctx, p, org)
	}
	if status != "pending" || !expires.After(s.now()) {
		return Access{}, ErrNotFound
	}
	var capabilities []string
	if err = json.Unmarshal(caps, &capabilities); err != nil {
		return Access{}, err
	}
	if err = validateCapabilities(capabilities); err != nil {
		return Access{}, err
	}
	var current string
	err = tx.QueryRowContext(ctx, `SELECT status FROM bi_access WHERE bi_user_id=$1 AND organization_id=$2 FOR UPDATE`, p.UserID, orgID).Scan(&current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Access{}, err
	}
	if current == "revoked" {
		return Access{}, ErrForbidden
	}
	if current == "active" {
		_ = tx.Rollback()
		return s.AuthorizeOrganization(ctx, p, org)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO bi_access(bi_user_id,organization_id,capabilities,organization_alias) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, p.UserID, orgID, string(caps), recipient); err != nil {
		return Access{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE bi_invitations SET status='used',claimed_by=$2,claimed_at=$3 WHERE id=$1`, id, p.UserID, s.now().UTC()); err != nil {
		return Access{}, err
	}
	if err = s.audit(ctx, tx, "bi_user", p.UserID, orgID, "invitation.accept", id, requestID); err != nil {
		return Access{}, err
	}
	if err = tx.Commit(); err != nil {
		return Access{}, err
	}
	return s.AuthorizeOrganization(ctx, p, org)
}
func (s *Service) UpdateAccess(ctx context.Context, org, userID string, caps []string, status string, revision, actor int64, requestID string, aliases ...string) (Access, error) {
	var alias any
	if len(aliases) > 0 {
		value := strings.TrimSpace(aliases[0])
		if utf8.RuneCountInString(value) > 80 {
			return Access{}, invalid("organization_alias", "Organization alias exceeds 80 characters")
		}
		alias = value
	}
	if status != "active" && status != "revoked" {
		return Access{}, invalid("status", "Invalid access status")
	}
	if err := validateCapabilities(caps); err != nil {
		return Access{}, err
	}
	raw, _ := json.Marshal(slices.Compact(slices.Sorted(slices.Values(caps))))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Access{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var internal int64
	var currentAlias string
	err = tx.QueryRowContext(ctx, `UPDATE bi_access a SET capabilities=$3,status=$4,revision=a.revision+1,updated_at=NOW(),organization_alias=COALESCE($6,organization_alias)
 FROM desktop_organizations o WHERE a.organization_id=o.id AND o.public_id=$1 AND a.bi_user_id=$2 AND a.revision=$5 AND o.deleted_at IS NULL RETURNING o.id,a.organization_alias`, org, userID, string(raw), status, revision, alias).Scan(&internal, &currentAlias)
	if errors.Is(err, sql.ErrNoRows) {
		return Access{}, apiError(409, "CONFLICT", "Access changed; reload before saving")
	}
	if err != nil {
		return Access{}, err
	}
	if err = s.audit(ctx, tx, "web_user", strconv.FormatInt(actor, 10), internal, "access.update", userID, requestID); err != nil {
		return Access{}, err
	}
	if err = tx.Commit(); err != nil {
		return Access{}, err
	}
	return Access{UserID: userID, OrganizationID: org, Scope: "organization", Capabilities: caps, Status: status, Revision: revision + 1, OrganizationAlias: currentAlias}, nil
}
func (s *Service) SetUserStatus(ctx context.Context, userID, status string, actor int64, requestID string) error {
	if status != "active" && status != "disabled" {
		return invalid("status", "Invalid user status")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE bi_users SET status=$2,auth_version=auth_version+1,updated_at=NOW() WHERE id=$1`, userID, status)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	if _, err = tx.ExecContext(ctx, `UPDATE bi_sessions SET revoked_at=COALESCE(revoked_at,$2) WHERE bi_user_id=$1`, userID, s.now()); err != nil {
		return err
	}
	if err = s.audit(ctx, tx, "web_user", strconv.FormatInt(actor, 10), 0, "identity."+status, userID, requestID); err != nil {
		return err
	}
	return tx.Commit()
}
