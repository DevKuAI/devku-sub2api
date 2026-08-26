package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestDesktopRefreshStoreConcurrentRotateAllowsOneAndReplayRevokesFamily(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewDesktopRefreshStore(client)
	ctx := context.Background()
	expiresAt := time.Now().Add(24 * time.Hour).Truncate(time.Second)

	require.NoError(t, store.Create(ctx, "old", service.DesktopRefreshSession{
		FamilyID: "family_one", MemberPublicID: "member_one", AbsoluteExpiresAt: expiresAt,
	}))

	type rotateResult struct {
		status  service.DesktopRefreshRotateResult
		session *service.DesktopRefreshSession
		err     error
	}
	results := make(chan rotateResult, 2)
	var start sync.WaitGroup
	start.Add(1)
	for _, next := range []string{"new_one", "new_two"} {
		go func(newHash string) {
			start.Wait()
			status, session, err := store.Rotate(ctx, "old", newHash, time.Now())
			results <- rotateResult{status: status, session: session, err: err}
		}(next)
	}
	start.Done()

	seen := map[service.DesktopRefreshRotateResult]int{}
	for range 2 {
		result := <-results
		require.NoError(t, result.err)
		seen[result.status]++
		if result.status == service.DesktopRefreshRotated {
			require.Equal(t, expiresAt, result.session.AbsoluteExpiresAt)
		}
	}
	require.Equal(t, 1, seen[service.DesktopRefreshRotated])
	require.Equal(t, 1, seen[service.DesktopRefreshReplayed])

	for _, hash := range []string{"new_one", "new_two"} {
		status, _, err := store.Rotate(ctx, hash, "later", time.Now())
		require.NoError(t, err)
		require.Equal(t, service.DesktopRefreshUnknown, status)
	}
}

func TestDesktopRefreshStoreFailsClosedWhenRedisUnavailable(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: 0})
	store := NewDesktopRefreshStore(client)
	server.Close()
	t.Cleanup(func() { _ = client.Close() })

	_, _, err := store.Rotate(context.Background(), "old", "new", time.Now())
	require.ErrorIs(t, err, service.ErrDesktopAuthStoreUnavailable)
}
