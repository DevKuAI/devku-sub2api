package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestTiboResetMonitorCacheRoundTrip(t *testing.T) {
	redisServer := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cache := NewTiboResetMonitorCache(rdb)
	ctx := context.Background()
	if value, err := cache.Read(ctx); err != nil || value != nil {
		t.Fatalf("initial Read() = %q, %v", value, err)
	}
	if err := cache.Write(ctx, []byte(`{"schemaVersion":1}`), time.Hour); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	value, err := cache.Read(ctx)
	if err != nil {
		t.Fatalf("cached Read() error = %v", err)
	}
	if string(value) != `{"schemaVersion":1}` {
		t.Fatalf("cached value = %q", value)
	}
}
