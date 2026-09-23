package bi

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"time"
)

func (s *Service) refreshIdentity(ctx context.Context, token string) (string, error) {
	if len(token) == 0 || len(token) > 2048 {
		return "", ErrUnauthenticated
	}
	var openHash string
	err := s.db.QueryRowContext(ctx, `SELECT b.openid_hash FROM bi_refresh_tokens r JOIN bi_sessions s ON s.id=r.session_id
		JOIN bi_wechat_bindings b ON b.id=s.binding_id WHERE r.token_hash=$1 AND b.appid=$2`, tokenHash(token), s.config.AppID).Scan(&openHash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrUnauthenticated
	}
	return openHash, err
}

func (s *Service) Refresh(ctx context.Context, raw, requestID string) (*Session, error) {
	openHash, err := s.refreshIdentity(ctx, raw)
	if err != nil {
		return nil, err
	}
	var result *Session
	var rejected error
	err = s.identityTx(ctx, openHash, func(tx *sql.Tx) error {
		var sessionID, managerID, credential string
		var userID int64
		var consumed, revoked, bindingRevoked sql.NullTime
		var expires, tokenExpires time.Time
		err := tx.QueryRowContext(ctx, `SELECT s.id,b.manager_id,m.user_id,s.credential_version,r.consumed_at,s.revoked_at,b.revoked_at,s.expires_at,r.expires_at
			FROM bi_refresh_tokens r JOIN bi_sessions s ON s.id=r.session_id JOIN bi_wechat_bindings b ON b.id=s.binding_id
			JOIN bi_managers m ON m.id=b.manager_id WHERE r.token_hash=$1 FOR UPDATE OF s,r`, tokenHash(raw)).
			Scan(&sessionID, &managerID, &userID, &credential, &consumed, &revoked, &bindingRevoked, &expires, &tokenExpires)
		if err != nil {
			return err
		}
		now := s.now().UTC()
		if revoked.Valid || bindingRevoked.Valid || !expires.After(now) || !tokenExpires.After(now) {
			return ErrUnauthenticated
		}
		if consumed.Valid {
			if _, err := tx.ExecContext(ctx, `UPDATE bi_sessions SET revoked_at=$2 WHERE id=$1`, sessionID, now); err != nil {
				return err
			}
			rejected = ErrUnauthenticated
			return recordSecurityEvent(ctx, tx, managerID, "session.refresh_replay", sessionID, requestID)
		}
		u, err := s.activeUser(ctx, userID)
		if err != nil {
			return err
		}
		if subtle.ConstantTimeCompare([]byte(credential), []byte(s.credentialVersion(u))) != 1 {
			return ErrUnauthenticated
		}
		refresh := randomToken("bir_")
		if _, err := tx.ExecContext(ctx, `UPDATE bi_refresh_tokens SET consumed_at=$2 WHERE token_hash=$1`, tokenHash(raw), now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO bi_refresh_tokens(token_hash,session_id,expires_at) VALUES($1,$2,$3)`,
			tokenHash(refresh), sessionID, now.Add(RefreshTTL)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE bi_sessions SET expires_at=$2 WHERE id=$1`, sessionID, now.Add(RefreshTTL)); err != nil {
			return err
		}
		result, err = s.sessionResponse(ctx, tx, managerID, sessionID, refresh, u)
		return err
	})
	if err != nil {
		return nil, err
	}
	if rejected != nil {
		return nil, rejected
	}
	return result, nil
}

func (s *Service) Logout(ctx context.Context, raw, requestID string) error {
	openHash, err := s.refreshIdentity(ctx, raw)
	if errors.Is(err, ErrUnauthenticated) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.identityTx(ctx, openHash, func(tx *sql.Tx) error {
		var sessionID, managerID string
		err := tx.QueryRowContext(ctx, `UPDATE bi_sessions s SET revoked_at=COALESCE(s.revoked_at,$2)
			FROM bi_refresh_tokens r,bi_wechat_bindings b WHERE r.token_hash=$1 AND s.id=r.session_id AND b.id=s.binding_id
			RETURNING s.id,b.manager_id`, tokenHash(raw), s.now().UTC()).Scan(&sessionID, &managerID)
		if err != nil {
			return err
		}
		return recordSecurityEvent(ctx, tx, managerID, "session.logout", sessionID, requestID)
	})
}
