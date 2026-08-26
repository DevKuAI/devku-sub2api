package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/nyaruka/phonenumbers"
	"golang.org/x/text/unicode/norm"
)

func NormalizeDesktopName(name string) (string, error) {
	name = norm.NFC.String(strings.TrimSpace(name))
	if name == "" || len([]rune(name)) > 100 {
		return "", ErrDesktopValidation.WithMetadata(map[string]string{"field": "name"})
	}
	return name, nil
}

func NormalizeDesktopPhone(phone string) (string, error) {
	phone = strings.TrimSpace(phone)
	parsed, err := phonenumbers.Parse(phone, "CN")
	if err != nil || !phonenumbers.IsValidNumber(parsed) {
		return "", ErrDesktopValidation.WithMetadata(map[string]string{"field": "phone"})
	}
	return phonenumbers.Format(parsed, phonenumbers.E164), nil
}

func GenerateDesktopPublicID(prefix string) (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(random), nil
}

func GenerateDesktopRefreshToken() (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(random), nil
}

func HashDesktopOpaqueToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

type DesktopAccessClaims struct {
	OrganizationID      string `json:"organization_id"`
	SessionID           string `json:"sid"`
	MemberVersion       int64  `json:"member_version"`
	OrganizationVersion int64  `json:"organization_version"`
	jwt.RegisteredClaims
}

type DesktopTokenManager struct {
	secret []byte
	ttl    time.Duration
}

func NewDesktopTokenManager(cfg *config.Config) (*DesktopTokenManager, error) {
	if cfg == nil || !cfg.Desktop.Enabled {
		return &DesktopTokenManager{}, nil
	}
	secret, err := base64.StdEncoding.Strict().DecodeString(cfg.Desktop.JWTSecret)
	if err != nil {
		return nil, fmt.Errorf("decode desktop JWT secret: %w", err)
	}
	return &DesktopTokenManager{secret: secret, ttl: time.Duration(cfg.Desktop.AccessTokenTTLMinutes) * time.Minute}, nil
}

func (m *DesktopTokenManager) Issue(member, organization *DesktopMemberOrganizationClaims, sessionID string, now time.Time) (string, error) {
	claims := DesktopAccessClaims{
		OrganizationID:      organization.PublicID,
		SessionID:           sessionID,
		MemberVersion:       member.AuthVersion,
		OrganizationVersion: organization.AuthVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   member.PublicID,
			Issuer:    "devku-sub2api",
			Audience:  jwt.ClaimStrings{"devku-desktop"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

func (m *DesktopTokenManager) Parse(tokenString string) (*DesktopAccessClaims, error) {
	if tokenString == "" || len(tokenString) > 8192 {
		return nil, ErrDesktopUnauthenticated
	}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer("devku-sub2api"),
		jwt.WithAudience("devku-desktop"),
		jwt.WithExpirationRequired(),
	)
	token, err := parser.ParseWithClaims(tokenString, &DesktopAccessClaims{}, func(*jwt.Token) (any, error) { return m.secret, nil })
	if err != nil || !token.Valid {
		return nil, ErrDesktopUnauthenticated.WithCause(err)
	}
	claims, ok := token.Claims.(*DesktopAccessClaims)
	if !ok || claims.Subject == "" || claims.OrganizationID == "" || claims.SessionID == "" {
		return nil, ErrDesktopUnauthenticated
	}
	return claims, nil
}

// DesktopMemberOrganizationClaims keeps token issuance independent from persistence DTOs.
type DesktopMemberOrganizationClaims struct {
	PublicID    string
	AuthVersion int64
}
