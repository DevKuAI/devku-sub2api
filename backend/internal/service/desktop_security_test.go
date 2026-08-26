package service

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/unicode/norm"
)

func desktopSecurityConfig() *config.Config {
	secret := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	return &config.Config{Desktop: config.DesktopConfig{
		Enabled:               true,
		JWTSecret:             secret,
		AccessTokenTTLMinutes: 15,
	}}
}

func TestNormalizeDesktopIdentity(t *testing.T) {
	name, err := NormalizeDesktopName("  e\u0301  ")
	require.NoError(t, err)
	require.True(t, norm.NFC.IsNormalString(name))
	require.Equal(t, "é", name)
}

func TestNormalizeDesktopPhoneAcceptsDomesticAndE164Formats(t *testing.T) {
	for _, input := range []string{"13800000000", "+8613800000000"} {
		t.Run(input, func(t *testing.T) {
			phone, normalizeErr := NormalizeDesktopPhone(input)
			require.NoError(t, normalizeErr)
			require.Equal(t, "+8613800000000", phone)
		})
	}
}

func TestDesktopAccessTokenContract(t *testing.T) {
	manager, err := NewDesktopTokenManager(desktopSecurityConfig())
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Second)
	token, err := manager.Issue(
		&DesktopMemberOrganizationClaims{PublicID: "mem_one", AuthVersion: 2},
		&DesktopMemberOrganizationClaims{PublicID: "org_one", AuthVersion: 3},
		"session_one",
		now,
	)
	require.NoError(t, err)
	claims, err := manager.Parse(token)
	require.NoError(t, err)
	require.Equal(t, "mem_one", claims.Subject)
	require.Equal(t, int64(2), claims.MemberVersion)
	require.Equal(t, int64(3), claims.OrganizationVersion)
	require.WithinDuration(t, now.Add(15*time.Minute), claims.ExpiresAt.Time, time.Second)
}
