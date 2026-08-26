package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func desktopLimiterConfig() *config.Config {
	return &config.Config{Desktop: config.DesktopConfig{
		LookupIPPerMinute: 1, LoginIPPerMinute: 10, LoginOrganizationPerMinute: 10,
		LoginPhoneFailureLimit: 2, LoginPhoneFreezeMinutes: 15,
	}}
}

func TestDesktopLoginLimiterUsesInstallationDimension(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	limiter := NewDesktopLoginLimiter(client, desktopLimiterConfig())
	ctx := context.Background()

	retry, err := limiter.AllowLookup(ctx, "192.0.2.1", "installation_one")
	require.NoError(t, err)
	require.Zero(t, retry)

	retry, err = limiter.AllowLookup(ctx, "192.0.2.2", "installation_one")
	require.NoError(t, err)
	require.Positive(t, retry)
}

func TestDesktopLoginLimiterFailsClosedWhenRedisUnavailable(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: 0})
	limiter := NewDesktopLoginLimiter(client, desktopLimiterConfig())
	server.Close()
	t.Cleanup(func() { _ = client.Close() })

	_, err := limiter.AllowLogin(context.Background(), "192.0.2.1", "installation_one", "org", "phone_hash")
	require.ErrorIs(t, err, service.ErrDesktopAuthStoreUnavailable)
}
