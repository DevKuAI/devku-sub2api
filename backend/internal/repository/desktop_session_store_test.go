package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestDesktopSessionSlidingExpiryUsesRedisTime(t *testing.T) {
	server := miniredis.RunT(t)
	now := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	server.SetTime(now)
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := NewDesktopSessionStore(rdb)
	ctx := context.Background()
	session := &service.DesktopSession{SessionID: "session_one", MemberPublicID: "mem_one", InstallationID: "device_one", MemberVersion: 1, OrganizationVersion: 1}
	require.NoError(t, store.Create(ctx, "hash_one", session))
	require.Equal(t, now, session.LastActiveAt)
	require.Equal(t, service.DesktopSessionIdleTimeout, server.TTL(desktopSessionPrefix+"hash_one"))
	for range 4 {
		now = now.Add(14 * 24 * time.Hour)
		server.SetTime(now)
		server.FastForward(14 * 24 * time.Hour)
		require.NoError(t, store.Touch(ctx, "hash_one"))
		actual, err := store.Get(ctx, "hash_one")
		require.NoError(t, err)
		require.Equal(t, now.Add(service.DesktopSessionIdleTimeout), actual.IdleExpiresAt)
	}
	// A backwards clock must not decrease the recorded activity or expiry.
	server.SetTime(now.Add(-time.Minute))
	require.NoError(t, store.Touch(ctx, "hash_one"))
	actual, err := store.Get(ctx, "hash_one")
	require.NoError(t, err)
	require.Equal(t, now, actual.LastActiveAt)
	// Check the logical boundary even if Redis has not yet evicted the key.
	server.SetTime(actual.IdleExpiresAt)
	require.ErrorIs(t, store.Touch(ctx, "hash_one"), service.ErrDesktopUnauthenticated)
	require.False(t, server.Exists(desktopSessionPrefix+"hash_one"))
}

func TestDesktopSessionConcurrentTouchAndRevocationNeverResurrect(t *testing.T) {
	server := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := NewDesktopSessionStore(rdb)
	ctx := context.Background()
	require.NoError(t, store.Create(ctx, "one", &service.DesktopSession{SessionID: "one"}))
	require.NoError(t, store.Create(ctx, "other", &service.DesktopSession{SessionID: "other"}))
	start := make(chan struct{})
	errors := make(chan error, 32)
	var workers sync.WaitGroup
	for range 32 {
		workers.Add(1)
		go func() { defer workers.Done(); <-start; errors <- store.Touch(ctx, "one") }()
	}
	close(start)
	require.NoError(t, store.Delete(ctx, "one"))
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			require.ErrorIs(t, err, service.ErrDesktopUnauthenticated)
		}
	}
	require.ErrorIs(t, store.Touch(ctx, "one"), service.ErrDesktopUnauthenticated)
	require.False(t, server.Exists(desktopSessionPrefix+"one"))
	_, err := store.Get(ctx, "other")
	require.NoError(t, err)
}

func TestDesktopSessionStoreFailsClosed(t *testing.T) {
	server := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })
	store := NewDesktopSessionStore(rdb)
	_, err := store.Get(context.Background(), "unknown")
	require.ErrorIs(t, err, service.ErrDesktopUnauthenticated)
	server.Close()
	_, err = store.Get(context.Background(), "unknown")
	require.ErrorIs(t, err, service.ErrDesktopAuthStoreUnavailable)
	require.ErrorIs(t, store.Touch(context.Background(), "unknown"), service.ErrDesktopAuthStoreUnavailable)
}

func TestDesktopConversationLimiterIsAtomicIndependentAndExpires(t *testing.T) {
	server := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	limiter := NewDesktopConversationLimiter(rdb, &config.Config{Desktop: config.DesktopConfig{ConversationMemberPerMinute: 60}})
	var workers sync.WaitGroup
	allowed := make(chan bool, 80)
	for range 80 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			retry, err := limiter.Allow(context.Background(), "mem_one")
			allowed <- err == nil && retry == 0
		}()
	}
	workers.Wait()
	close(allowed)
	count := 0
	for value := range allowed {
		if value {
			count++
		}
	}
	require.Equal(t, 60, count)
	retry, err := limiter.Allow(context.Background(), "mem_other")
	require.NoError(t, err)
	require.Zero(t, retry)
	for _, key := range server.Keys() {
		require.Contains(t, key, "desktop_conversation_rate:")
	}
	server.FastForward(time.Minute)
	retry, err = limiter.Allow(context.Background(), "mem_one")
	require.NoError(t, err)
	require.Zero(t, retry)
}
