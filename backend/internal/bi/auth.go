package bi

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type UserReader interface {
	GetByID(context.Context, int64) (*service.User, error)
}

type Service struct {
	db       *sql.DB
	config   config.BIConfig
	users    UserReader
	wechat   WeChatExchanger
	tokens   tokenManager
	identity []byte
	now      func() time.Time
}

func NewService(db *sql.DB, cfg config.BIConfig, users UserReader, wechat WeChatExchanger) *Service {
	identity, _ := base64.StdEncoding.Strict().DecodeString(cfg.IdentitySecret)
	return &Service{db: db, config: cfg, users: users, wechat: wechat,
		tokens: newTokenManager(cfg.JWTSecret), identity: identity, now: time.Now}
}

type binding struct {
	ID        string
	ManagerID string
	UserID    int64
	OpenHash  string
}

type LoginOptions struct {
	ClientVersion string
	DeviceID      string
	Platform      string
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (s *Service) identityTx(ctx context.Context, openHash string, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// All binding/session mutations acquire the same identity lock first.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, s.config.AppID+":"+openHash); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) exchangeIdentity(ctx context.Context, code string) (string, error) {
	if len(code) == 0 || len(code) > 256 {
		return "", invalid("code", "Expected a fresh WeChat login code")
	}
	var inserted string
	err := s.db.QueryRowContext(ctx, `INSERT INTO bi_wechat_codes(code_hash, created_at) VALUES($1, $2)
		ON CONFLICT DO NOTHING RETURNING code_hash`, tokenHash(s.config.AppID+":"+code), s.now().UTC()).Scan(&inserted)
	if errors.Is(err, sql.ErrNoRows) {
		return "", invalid("code", "WeChat code has already been used")
	}
	if err != nil {
		return "", err
	}
	openID, err := s.wechat.Exchange(ctx, code)
	if err != nil {
		return "", err
	}
	return keyedHash(s.identity, s.config.AppID, openID), nil
}

func (s *Service) Login(ctx context.Context, code string) (any, error) {
	return s.LoginWithOptions(ctx, code, LoginOptions{})
}

func (s *Service) LoginWithOptions(ctx context.Context, code string, options LoginOptions) (any, error) {
	openHash, err := s.exchangeIdentity(ctx, code)
	if err != nil {
		return nil, err
	}
	var result any
	err = s.identityTx(ctx, openHash, func(tx *sql.Tx) error {
		b, err := s.activeBinding(ctx, tx, openHash)
		if err == nil {
			result, err = s.createSession(ctx, tx, b, options)
			return err
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		ticket := randomToken("btk_")
		code := strings.ToUpper(rand.Text()[:8])
		now := s.now().UTC()
		_, err = tx.ExecContext(ctx, `INSERT INTO bi_binding_challenges
			(ticket_hash, code_hash, appid, openid_hash, expires_at, created_at) VALUES($1,$2,$3,$4,$5,$6)`,
			tokenHash(ticket), keyedHash(s.identity, "binding-code", code), s.config.AppID, openHash, now.Add(ChallengeTTL), now)
		if err != nil {
			return err
		}
		result = BindingChallenge{Result: "binding_required", BindingTicket: ticket, UserCode: code,
			ExpiresAt: now.Add(ChallengeTTL), VerificationPath: "/bi/bind"}
		return nil
	})
	return result, err
}

func (s *Service) activeBinding(ctx context.Context, q queryer, openHash string) (binding, error) {
	var b binding
	err := q.QueryRowContext(ctx, `SELECT b.id,b.manager_id,m.user_id,b.openid_hash FROM bi_wechat_bindings b
		JOIN bi_managers m ON m.id=b.manager_id WHERE b.appid=$1 AND b.openid_hash=$2 AND b.revoked_at IS NULL`,
		s.config.AppID, openHash).Scan(&b.ID, &b.ManagerID, &b.UserID, &b.OpenHash)
	return b, err
}

func (s *Service) activeUser(ctx context.Context, userID int64) (*service.User, error) {
	u, err := s.users.GetByID(ctx, userID)
	if errors.Is(err, service.ErrUserNotFound) || (err == nil && (u == nil || !u.IsActive() || u.DeletedAt != nil)) {
		return nil, ErrUnauthenticated
	}
	return u, err
}

func (s *Service) credentialVersion(user *service.User) string {
	return keyedHash(s.identity, "credentials", user.Email, user.PasswordHash)
}

func (s *Service) createSession(ctx context.Context, tx *sql.Tx, b binding, options ...LoginOptions) (*Session, error) {
	u, err := s.activeUser(ctx, b.UserID)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	sessionID := randomToken("bis_")
	refresh := randomToken("bir_")
	var deviceHash, platform, clientVersion any
	if len(options) > 0 {
		if options[0].DeviceID != "" {
			deviceHash = keyedHash(s.identity, "device", options[0].DeviceID)
		}
		platform = truncateIdentityField(options[0].Platform, 32)
		clientVersion = truncateIdentityField(options[0].ClientVersion, 32)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO bi_sessions(id,binding_id,credential_version,created_at,expires_at,device_id_hash,device_platform,client_version,last_seen_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$4)`, sessionID, b.ID, s.credentialVersion(u), now, now.Add(RefreshTTL), deviceHash, platform, clientVersion); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO bi_refresh_tokens(token_hash,session_id,expires_at) VALUES($1,$2,$3)`,
		tokenHash(refresh), sessionID, now.Add(RefreshTTL)); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE bi_wechat_bindings SET last_login_at=$2 WHERE id=$1`, b.ID, now); err != nil {
		return nil, err
	}
	return s.sessionResponse(ctx, tx, b.ManagerID, sessionID, refresh, u)
}

func truncateIdentityField(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) > limit {
		return value[:limit]
	}
	return value
}

func (s *Service) sessionResponse(ctx context.Context, q queryer, managerID, sessionID, refresh string, u *service.User) (*Session, error) {
	user, err := s.userDTO(ctx, q, managerID, u)
	if err != nil {
		return nil, err
	}
	access, err := s.tokens.issue(managerID, sessionID, s.now().UTC())
	if err != nil {
		return nil, err
	}
	return &Session{Result: "authenticated", AccessToken: access, RefreshToken: refresh, TokenType: "Bearer",
		ExpiresIn: int(AccessTTL.Seconds()), RefreshExpiresIn: int(RefreshTTL.Seconds()), User: user}, nil
}

func (s *Service) userDTO(ctx context.Context, q queryer, managerID string, u *service.User) (User, error) {
	var version int64
	if err := q.QueryRowContext(ctx, `SELECT authorization_version FROM bi_managers WHERE id=$1`, managerID).Scan(&version); err != nil {
		return User{}, err
	}
	organizations, err := s.organizations(ctx, q, managerID)
	if err != nil {
		return User{}, err
	}
	capabilities := []string{}
	for _, org := range organizations {
		capabilities = append(capabilities, org.Capabilities...)
	}
	slices.Sort(capabilities)
	name := u.Username
	if name == "" {
		name = "管理者"
	}
	return User{ID: managerID, DisplayName: name, Capabilities: slices.Compact(capabilities),
		AuthorizationVersion: fmt.Sprintf("acl_%d", version)}, nil
}

func (s *Service) organizations(ctx context.Context, q queryer, managerID string) ([]Organization, error) {
	rows, err := q.QueryContext(ctx, `SELECT o.id,d.name,d.status,g.role,g.all_teams,g.team_ids,g.capabilities
		FROM bi_manager_grants g JOIN bi_organizations o ON o.id=g.organization_id
		JOIN desktop_organizations d ON d.id=o.desktop_organization_id
		WHERE g.manager_id=$1 AND g.revoked_at IS NULL AND d.deleted_at IS NULL AND d.status='active' ORDER BY o.id`, managerID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []Organization{}
	for rows.Next() {
		var org Organization
		var teams, caps []byte
		if err := rows.Scan(&org.ID, &org.Name, &org.Status, &org.Role, &org.AllTeams, &teams, &caps); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(teams, &org.TeamIDs); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(caps, &org.Capabilities); err != nil {
			return nil, err
		}
		result = append(result, org)
	}
	return result, rows.Err()
}

func (s *Service) Authorize(ctx context.Context, raw string) (Principal, error) {
	claims, err := s.tokens.parse(raw, s.now())
	if err != nil {
		return Principal{}, err
	}
	var p Principal
	var credentials string
	err = s.db.QueryRowContext(ctx, `SELECT b.manager_id,m.user_id,b.id,s.id,s.credential_version FROM bi_sessions s
		JOIN bi_wechat_bindings b ON b.id=s.binding_id JOIN bi_managers m ON m.id=b.manager_id
		WHERE s.id=$1 AND b.manager_id=$2 AND b.appid=$3 AND s.revoked_at IS NULL AND b.revoked_at IS NULL AND s.expires_at>$4`,
		claims.SessionID, claims.Subject, s.config.AppID, s.now().UTC()).Scan(&p.ManagerID, &p.UserID, &p.BindingID, &p.SessionID, &credentials)
	if errors.Is(err, sql.ErrNoRows) {
		return Principal{}, ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, err
	}
	u, err := s.activeUser(ctx, p.UserID)
	if err != nil {
		return Principal{}, err
	}
	if subtle.ConstantTimeCompare([]byte(credentials), []byte(s.credentialVersion(u))) != 1 {
		return Principal{}, ErrUnauthenticated
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE bi_sessions SET last_seen_at=$2 WHERE id=$1`, p.SessionID, s.now().UTC()); err != nil {
		return Principal{}, err
	}
	return p, nil
}

func (s *Service) Me(ctx context.Context, p Principal) (User, error) {
	u, err := s.activeUser(ctx, p.UserID)
	if err != nil {
		return User{}, err
	}
	return s.userDTO(ctx, s.db, p.ManagerID, u)
}

func recordSecurityEvent(ctx context.Context, tx *sql.Tx, managerID, action, target, requestID string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO bi_security_events(manager_id,action,target_id,request_id) VALUES($1,$2,$3,$4)`,
		managerID, action, target, requestID)
	return err
}
