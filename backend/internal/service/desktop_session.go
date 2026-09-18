package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

const DesktopSessionIdleTimeout = 15 * 24 * time.Hour

var (
	ErrDesktopAuthVersionUnsupported = infraerrors.New(http.StatusUnprocessableEntity, "AUTH_VERSION_UNSUPPORTED", "unsupported desktop authentication version")
	ErrDesktopSessionRequired        = infraerrors.Unauthorized("SESSION_AUTH_REQUIRED", "a v2 desktop session is required")
	ErrDesktopInstallationMismatch   = infraerrors.Forbidden("SESSION_INSTALLATION_MISMATCH", "installation does not match the session")
)

// DesktopSession never contains the raw bearer credential.
type DesktopSession struct {
	SessionID            string
	MemberPublicID       string
	OrganizationPublicID string
	InstallationID       string
	MemberVersion        int64
	OrganizationVersion  int64
	CreatedAt            time.Time
	LastActiveAt         time.Time
	IdleExpiresAt        time.Time
}

type DesktopSessionStore interface {
	Create(context.Context, string, *DesktopSession) error
	Get(context.Context, string) (*DesktopSession, error)
	Touch(context.Context, string) error
	Delete(context.Context, string) error
}

type DesktopSessionLogin struct {
	AuthVersion        int       `json:"auth_version"`
	TokenType          string    `json:"token_type"`
	AccessToken        string    `json:"access_token"`
	SessionID          string    `json:"session_id"`
	IdleTimeoutSeconds int       `json:"idle_timeout_seconds"`
	IdleExpiresAt      time.Time `json:"idle_expires_at"`
}

type DesktopAuthorization struct {
	Member    *DesktopAuthorizedMember
	Session   *DesktopSession
	TokenHash string
}

func ValidateDesktopInstallationID(value string) error {
	parsed, err := uuid.Parse(value)
	if err != nil || parsed.String() != value {
		return ErrDesktopValidation.WithMetadata(map[string]string{"field": "X-Installation-ID"})
	}
	return nil
}

func IsDesktopSessionToken(token string) bool { return strings.HasPrefix(token, "dks_") }

func (s *DesktopService) LoginV2(ctx context.Context, organizationCode, name, phone, ip, installationID string) (*DesktopSessionLogin, error) {
	if err := ValidateDesktopInstallationID(installationID); err != nil {
		return nil, err
	}
	authorized, err := s.authenticateLogin(ctx, organizationCode, name, phone, ip, installationID)
	if err != nil {
		return nil, err
	}
	if s.sessions == nil {
		return nil, ErrDesktopAuthStoreUnavailable
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return nil, ErrDesktopAuthStoreUnavailable
	}
	token := "dks_" + base64.RawURLEncoding.EncodeToString(random)
	sessionID, err := GenerateDesktopPublicID("session")
	if err != nil {
		return nil, ErrDesktopAuthStoreUnavailable
	}
	session := &DesktopSession{
		SessionID: sessionID, MemberPublicID: authorized.Member.PublicID,
		OrganizationPublicID: authorized.Organization.PublicID, InstallationID: installationID,
		MemberVersion: authorized.Member.AuthVersion, OrganizationVersion: authorized.Organization.AuthVersion,
	}
	if err := s.sessions.Create(ctx, HashDesktopOpaqueToken(token), session); err != nil {
		return nil, err
	}
	return &DesktopSessionLogin{AuthVersion: 2, TokenType: "Bearer", AccessToken: token,
		SessionID: sessionID, IdleTimeoutSeconds: int(DesktopSessionIdleTimeout.Seconds()), IdleExpiresAt: session.IdleExpiresAt}, nil
}

// AuthorizeRequest only authenticates. Business handlers touch after validation.
func (s *DesktopService) AuthorizeRequest(ctx context.Context, token, installationID string) (*DesktopAuthorization, error) {
	if !IsDesktopSessionToken(token) {
		member, _, err := s.Authorize(ctx, token)
		if err != nil {
			return nil, err
		}
		return &DesktopAuthorization{Member: member}, nil
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(strings.TrimPrefix(token, "dks_"))
	if err != nil || len(raw) != 32 {
		return nil, ErrDesktopUnauthenticated
	}
	if err := ValidateDesktopInstallationID(installationID); err != nil {
		return nil, err
	}
	if s.sessions == nil {
		return nil, ErrDesktopAuthStoreUnavailable
	}
	hash := HashDesktopOpaqueToken(token)
	session, err := s.sessions.Get(ctx, hash)
	if err != nil {
		return nil, err
	}
	if session.InstallationID != installationID {
		return nil, ErrDesktopInstallationMismatch
	}
	member, err := s.repo.GetAuthorizedMember(ctx, session.MemberPublicID)
	if err != nil || !desktopAuthorizationCurrent(member) || member.Organization.PublicID != session.OrganizationPublicID ||
		member.Member.AuthVersion != session.MemberVersion || member.Organization.AuthVersion != session.OrganizationVersion {
		return nil, ErrDesktopMembershipRevoked
	}
	return &DesktopAuthorization{Member: member, Session: session, TokenHash: hash}, nil
}

func (s *DesktopService) TouchSession(ctx context.Context, auth *DesktopAuthorization) error {
	if auth == nil {
		return ErrDesktopUnauthenticated
	}
	if auth.Session == nil {
		return nil
	}
	return s.sessions.Touch(ctx, auth.TokenHash)
}

func (s *DesktopService) LogoutAuthorized(ctx context.Context, auth *DesktopAuthorization, token string) error {
	if auth == nil {
		return ErrDesktopUnauthenticated
	}
	if auth.Session == nil {
		return s.Logout(ctx, token)
	}
	return s.sessions.Delete(ctx, auth.TokenHash)
}
