package bi

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"
)

var bindingCodePattern = regexp.MustCompile(`^[A-Z0-9]{8}$`)

func ensureManager(ctx context.Context, tx *sql.Tx, userID int64) (string, error) {
	var managerID string
	err := tx.QueryRowContext(ctx, `INSERT INTO bi_managers(id,user_id) VALUES($1,$2)
		ON CONFLICT(user_id) DO UPDATE SET user_id=EXCLUDED.user_id RETURNING id`, randomToken("bim_"), userID).Scan(&managerID)
	return managerID, err
}

func (s *Service) ApproveBinding(ctx context.Context, userID int64, code, requestID string) error {
	if !bindingCodePattern.MatchString(code) {
		return invalid("user_code", "Expected an eight-character binding code")
	}
	if _, err := s.activeUser(ctx, userID); err != nil {
		return err
	}
	codeHash := keyedHash(s.identity, "binding-code", code)
	var openHash string
	err := s.db.QueryRowContext(ctx, `SELECT openid_hash FROM bi_binding_challenges WHERE code_hash=$1 AND appid=$2`,
		codeHash, s.config.AppID).Scan(&openHash)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrBindingExpired
	}
	if err != nil {
		return err
	}
	return s.identityTx(ctx, openHash, func(tx *sql.Tx) error {
		var status string
		var expires time.Time
		if err := tx.QueryRowContext(ctx, `SELECT status,expires_at FROM bi_binding_challenges WHERE code_hash=$1 FOR UPDATE`,
			codeHash).Scan(&status, &expires); err != nil {
			return err
		}
		if !expires.After(s.now()) || (status != "pending" && status != "approved") {
			return ErrBindingExpired
		}
		managerID, err := ensureManager(ctx, tx, userID)
		if err != nil {
			return err
		}
		b, err := s.activeBinding(ctx, tx, openHash)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil && b.ManagerID != managerID {
			return ErrBindingConflict
		}
		if errors.Is(err, sql.ErrNoRows) {
			b.ID = randomToken("bib_")
			if _, err := tx.ExecContext(ctx, `INSERT INTO bi_wechat_bindings(id,appid,openid_hash,manager_id,created_at) VALUES($1,$2,$3,$4,$5)`,
				b.ID, s.config.AppID, openHash, managerID, s.now().UTC()); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE bi_binding_challenges SET status='approved',binding_id=$1 WHERE code_hash=$2`, b.ID, codeHash); err != nil {
			return err
		}
		return recordSecurityEvent(ctx, tx, managerID, "binding.approve", b.ID, requestID)
	})
}

func (s *Service) ExchangeBinding(ctx context.Context, ticket, code string) (*Session, error) {
	if len(ticket) < 32 || len(ticket) > 512 {
		return nil, invalid("binding_ticket", "Invalid binding ticket")
	}
	var openHash, status string
	var expires time.Time
	err := s.db.QueryRowContext(ctx, `SELECT openid_hash,status,expires_at FROM bi_binding_challenges WHERE ticket_hash=$1 AND appid=$2`,
		tokenHash(ticket), s.config.AppID).Scan(&openHash, &status, &expires)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (!expires.After(s.now()) || (status != "pending" && status != "approved"))) {
		return nil, ErrBindingExpired
	}
	if err != nil {
		return nil, err
	}
	identity, err := s.exchangeIdentity(ctx, code)
	if err != nil {
		return nil, err
	}
	if identity != openHash {
		return nil, ErrBindingConflict
	}
	var session *Session
	err = s.identityTx(ctx, openHash, func(tx *sql.Tx) error {
		var bindingID sql.NullString
		if err := tx.QueryRowContext(ctx, `SELECT status,expires_at,binding_id FROM bi_binding_challenges WHERE ticket_hash=$1 FOR UPDATE`,
			tokenHash(ticket)).Scan(&status, &expires, &bindingID); err != nil {
			return err
		}
		if !expires.After(s.now()) || status == "revoked" || status == "consumed" {
			return ErrBindingExpired
		}
		if status == "pending" {
			return ErrPendingBinding
		}
		b, err := s.activeBinding(ctx, tx, openHash)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && b.ID != bindingID.String) {
			return ErrBindingExpired
		}
		if err != nil {
			return err
		}
		session, err = s.createSession(ctx, tx, b)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE bi_binding_challenges SET status='consumed' WHERE ticket_hash=$1`, tokenHash(ticket))
		return err
	})
	return session, err
}

func (s *Service) RevokeBinding(ctx context.Context, userID int64, bindingID, requestID string) error {
	var openHash, managerID string
	err := s.db.QueryRowContext(ctx, `SELECT b.openid_hash,b.manager_id FROM bi_wechat_bindings b JOIN bi_managers m ON m.id=b.manager_id
		WHERE b.id=$1 AND m.user_id=$2 AND b.appid=$3`, bindingID, userID, s.config.AppID).Scan(&openHash, &managerID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return s.identityTx(ctx, openHash, func(tx *sql.Tx) error {
		now := s.now().UTC()
		var revoked sql.NullTime
		if err := tx.QueryRowContext(ctx, `SELECT revoked_at FROM bi_wechat_bindings WHERE id=$1 FOR UPDATE`, bindingID).Scan(&revoked); err != nil {
			return err
		}
		if revoked.Valid {
			return nil
		}
		if _, err := tx.ExecContext(ctx, `UPDATE bi_wechat_bindings SET revoked_at=$2 WHERE id=$1`, bindingID, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE bi_sessions SET revoked_at=COALESCE(revoked_at,$2) WHERE binding_id=$1`, bindingID, now); err != nil {
			return err
		}
		// Pending tickets for the same identity must not survive an explicit unlink.
		if _, err := tx.ExecContext(ctx, `UPDATE bi_binding_challenges SET status='revoked'
			WHERE appid=$1 AND openid_hash=$2 AND status IN ('pending','approved')`, s.config.AppID, openHash); err != nil {
			return err
		}
		return recordSecurityEvent(ctx, tx, managerID, "binding.revoke", bindingID, requestID)
	})
}
