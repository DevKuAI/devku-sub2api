package bi

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type Service struct {
	db       *sql.DB
	config   config.BIConfig
	desktop  *service.DesktopService
	wechat   WeChatExchanger
	tokens   tokenManager
	identity []byte
	now      func() time.Time
}

func NewService(db *sql.DB, cfg config.BIConfig, desktop *service.DesktopService, wechat WeChatExchanger) *Service {
	return &Service{db: db, config: cfg, desktop: desktop, wechat: wechat, tokens: newTokenManager(cfg.JWTSecret), identity: decodeSecret(cfg.IdentitySecret), now: time.Now}
}

type LoginOptions struct{ ClientVersion, DisplayName string }

func (s *Service) Login(ctx context.Context, code string, options LoginOptions) (*Session, error) {
	if err := requireClientVersion(options.ClientVersion, s.config.MinClientVersion); err != nil {
		return nil, err
	}
	if len(code) == 0 || len(code) > 256 || utf8.RuneCountInString(options.ClientVersion) > 32 || utf8.RuneCountInString(options.DisplayName) > 80 {
		return nil, invalid("code", "Invalid login fields")
	}
	if s.db == nil || s.wechat == nil {
		return nil, apiError(503, "UPSTREAM_UNAVAILABLE", "Authentication unavailable")
	}
	var inserted string
	err := s.db.QueryRowContext(ctx, `INSERT INTO bi_wechat_codes(code_hash,created_at) VALUES($1,$2) ON CONFLICT DO NOTHING RETURNING code_hash`, tokenHash(s.config.AppID+":"+code), s.now().UTC()).Scan(&inserted)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, invalid("code", "WeChat code has already been used")
	}
	if err != nil {
		return nil, err
	}
	openID, err := s.wechat.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}
	openHash := keyedHash(s.identity, "wechat", s.config.AppID, openID)
	name := strings.TrimSpace(options.DisplayName)
	if name == "" {
		name = "小程序使用者"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var user User
	err = tx.QueryRowContext(ctx, `INSERT INTO bi_users(id,appid,openid_hash,display_name) VALUES($1,$2,$3,$4)
 ON CONFLICT(appid,openid_hash) DO UPDATE SET updated_at=NOW() RETURNING id,display_name,status,auth_version`, randomToken("biu_"), s.config.AppID, openHash, name).Scan(&user.ID, &user.DisplayName, &user.Status, &user.AuthorizationVersion)
	if err != nil {
		return nil, err
	}
	if user.Status != "active" {
		return nil, ErrForbidden
	}
	result, err := s.createSession(ctx, tx, user, options.ClientVersion)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
func (s *Service) createSession(ctx context.Context, tx *sql.Tx, user User, version string) (*Session, error) {
	now := s.now().UTC()
	id := randomToken("bis_")
	refresh := randomToken("bir_")
	_, err := tx.ExecContext(ctx, `INSERT INTO bi_sessions(id,bi_user_id,auth_version,created_at,expires_at,last_seen_at,client_version) VALUES($1,$2,$3,$4,$5,$4,$6)`, id, user.ID, user.AuthorizationVersion, now, now.Add(RefreshTTL), version)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO bi_refresh_tokens(token_hash,session_id,expires_at) VALUES($1,$2,$3)`, tokenHash(refresh), id, now.Add(RefreshTTL))
	if err != nil {
		return nil, err
	}
	return s.sessionResponse(user, id, refresh)
}
func (s *Service) sessionResponse(user User, id, refresh string) (*Session, error) {
	access, err := s.tokens.issue(user.ID, id, s.now())
	if err != nil {
		return nil, err
	}
	return &Session{Result: "authenticated", AccessToken: access, RefreshToken: refresh, TokenType: "Bearer", ExpiresIn: int(AccessTTL.Seconds()), RefreshExpiresIn: int(RefreshTTL.Seconds()), User: user}, nil
}
func (s *Service) Authorize(ctx context.Context, raw string) (Principal, error) {
	claims, err := s.tokens.parse(raw, s.now())
	if err != nil {
		return Principal{}, err
	}
	if s.db == nil {
		return Principal{}, ErrUnauthenticated
	}
	var p Principal
	err = s.db.QueryRowContext(ctx, `SELECT u.id,s.id FROM bi_sessions s JOIN bi_users u ON u.id=s.bi_user_id
 WHERE s.id=$1 AND u.id=$2 AND u.appid=$3 AND u.status='active' AND u.auth_version=s.auth_version AND s.revoked_at IS NULL AND s.expires_at>$4`, claims.SessionID, claims.Subject, s.config.AppID, s.now()).Scan(&p.UserID, &p.SessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrUnauthenticated
	}
	if err != nil {
		return p, err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE bi_sessions SET last_seen_at=$2 WHERE id=$1`, p.SessionID, s.now().UTC())
	return p, err
}
func (s *Service) Me(ctx context.Context, p Principal) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `SELECT id,display_name,status,auth_version FROM bi_users WHERE id=$1 AND status='active'`, p.UserID).Scan(&u.ID, &u.DisplayName, &u.Status, &u.AuthorizationVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrUnauthenticated
	}
	return u, err
}
func (s *Service) Refresh(ctx context.Context, raw, requestID string) (*Session, error) {
	if len(raw) == 0 || len(raw) > 2048 {
		return nil, ErrUnauthenticated
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var u User
	var id string
	var consumed, revoked sql.NullTime
	var expires, refreshExpires time.Time
	var version int64
	err = tx.QueryRowContext(ctx, `SELECT u.id,u.display_name,u.status,u.auth_version,s.id,s.auth_version,r.consumed_at,s.revoked_at,s.expires_at,r.expires_at
 FROM bi_refresh_tokens r JOIN bi_sessions s ON s.id=r.session_id JOIN bi_users u ON u.id=s.bi_user_id
 WHERE r.token_hash=$1 AND u.appid=$2 FOR UPDATE OF u,s,r`, tokenHash(raw), s.config.AppID).Scan(&u.ID, &u.DisplayName, &u.Status, &u.AuthorizationVersion, &id, &version, &consumed, &revoked, &expires, &refreshExpires)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	if consumed.Valid {
		if _, err = tx.ExecContext(ctx, `UPDATE bi_sessions SET revoked_at=COALESCE(revoked_at,$2) WHERE id=$1`, id, now); err != nil {
			return nil, err
		}
		if err = s.audit(ctx, tx, "bi_user", u.ID, 0, "session.refresh_replay", id, requestID); err != nil {
			return nil, err
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return nil, ErrUnauthenticated
	}
	if u.Status != "active" || version != u.AuthorizationVersion || revoked.Valid || !expires.After(now) || !refreshExpires.After(now) {
		return nil, ErrUnauthenticated
	}
	refresh := randomToken("bir_")
	if _, err = tx.ExecContext(ctx, `UPDATE bi_refresh_tokens SET consumed_at=$2 WHERE token_hash=$1`, tokenHash(raw), now); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO bi_refresh_tokens(token_hash,session_id,expires_at) VALUES($1,$2,$3)`, tokenHash(refresh), id, now.Add(RefreshTTL)); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE bi_sessions SET expires_at=$2 WHERE id=$1`, id, now.Add(RefreshTTL)); err != nil {
		return nil, err
	}
	result, err := s.sessionResponse(u, id, refresh)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
func (s *Service) Logout(ctx context.Context, raw, requestID string) error {
	if len(raw) > 2048 {
		return invalid("refresh_token", "Invalid token")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var user, id string
	err = tx.QueryRowContext(ctx, `UPDATE bi_sessions s SET revoked_at=COALESCE(s.revoked_at,$2) FROM bi_refresh_tokens r WHERE r.token_hash=$1 AND r.session_id=s.id RETURNING s.bi_user_id,s.id`, tokenHash(raw), s.now().UTC()).Scan(&user, &id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if err = s.audit(ctx, tx, "bi_user", user, 0, "session.logout", id, requestID); err != nil {
		return err
	}
	return tx.Commit()

}

func decodeSecret(raw string) []byte {
	value, _ := base64.StdEncoding.Strict().DecodeString(raw)
	return value
}
