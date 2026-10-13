package bi

import (
	"encoding/base64"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

func TestIndependentTokenAudienceAndCapabilityDependencies(t *testing.T) {
	now := time.Now()
	tm := newTokenManager(base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", 32))))
	raw, e := tm.issue("biu_independent", "session", now)
	require.NoError(t, e)
	claims, e := tm.parse(raw, now)
	require.NoError(t, e)
	require.Equal(t, "biu_independent", claims.Subject)
	require.Equal(t, []string{MobileAudience}, []string(claims.Audience))
	_, e = tm.parse(raw, now.Add(AccessTTL))
	require.Error(t, e)
	for _, caps := range [][]string{{}, {"reports:copy"}, {"conversations:read"}, {"reports:read"}, {"unknown"}} {
		require.Error(t, validateCapabilities(caps))
	}
	require.NoError(t, validateCapabilities([]string{"analytics:read"}))
	require.NoError(t, validateCapabilities([]string{"members:read", "reports:read", "reports:copy"}))
}
