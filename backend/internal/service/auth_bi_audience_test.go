package service

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestWebAudienceIsAddedWithoutRejectingLegacyWebTokens(t *testing.T) {
	s := &AuthService{cfg: &config.Config{JWT: config.JWTConfig{Secret: "test-web-secret", AccessTokenExpireMinutes: 15}}}
	raw, err := s.generateAccessToken(&User{ID: 5, Email: "test@example.com"}, "sid", "")
	require.NoError(t, err)
	claims, err := s.ValidateToken(raw)
	require.NoError(t, err)
	require.Equal(t, jwt.ClaimStrings{"sub2api-web"}, claims.Audience)
	require.Equal(t, "devku-sub2api", claims.Issuer)
	legacy := &JWTClaims{UserID: 5, RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	raw, err = jwt.NewWithClaims(jwt.SigningMethodHS256, legacy).SignedString([]byte(s.cfg.JWT.Secret))
	require.NoError(t, err)
	_, err = s.ValidateToken(raw)
	require.NoError(t, err)
}
