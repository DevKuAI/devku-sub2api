package bi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func randomToken(prefix string) string {
	var random [32]byte
	_, _ = rand.Read(random[:])
	return prefix + base64.RawURLEncoding.EncodeToString(random[:])
}

func tokenHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func keyedHash(secret []byte, values ...string) string {
	h := hmac.New(sha256.New, secret)
	for _, value := range values {
		_, _ = h.Write([]byte(value))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

type accessClaims struct {
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

type tokenManager struct{ secret []byte }

func newTokenManager(secret string) tokenManager {
	decoded, _ := base64.StdEncoding.Strict().DecodeString(secret)
	return tokenManager{secret: decoded}
}

func (m tokenManager) issue(managerID, sessionID string, now time.Time) (string, error) {
	claims := accessClaims{SessionID: sessionID, RegisteredClaims: jwt.RegisteredClaims{
		Issuer: TokenIssuer, Subject: managerID, Audience: jwt.ClaimStrings{MobileAudience},
		IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(AccessTTL)),
	}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

func (m tokenManager) parse(raw string, now time.Time) (*accessClaims, error) {
	if len(raw) == 0 || len(raw) > 8192 || len(m.secret) < 32 {
		return nil, ErrUnauthenticated
	}
	claims := new(accessClaims)
	_, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(TokenIssuer),
		jwt.WithAudience(MobileAudience), jwt.WithExpirationRequired(), jwt.WithTimeFunc(func() time.Time { return now }))
	if err != nil || claims.Subject == "" || claims.SessionID == "" {
		return nil, ErrUnauthenticated
	}
	return claims, nil
}

func bearerToken(header string) string {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}
