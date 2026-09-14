package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestTiboResetMonitorServiceCachesUpstreamResponse(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"schemaVersion":1,"timezone":"Asia/Shanghai","checkedAt":"2026-09-14T08:00:00+08:00","events":[]}`))
	}))
	defer server.Close()
	previousEndpoint := tiboResetMonitorEndpoint
	tiboResetMonitorEndpoint = server.URL
	t.Cleanup(func() { tiboResetMonitorEndpoint = previousEndpoint })

	redisServer := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	defer rdb.Close()

	monitor := NewTiboResetMonitorService(rdb)
	if _, err := monitor.Get(context.Background()); err != nil {
		t.Fatalf("first Get() error = %v", err)
	}
	if _, err := monitor.Get(context.Background()); err != nil {
		t.Fatalf("second Get() error = %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1", got)
	}
	if ttl := redisServer.TTL(tiboResetMonitorKey); ttl <= 0 || ttl > tiboResetMonitorTTL {
		t.Fatalf("cache TTL = %s, want between 0 and %s", ttl, tiboResetMonitorTTL)
	}
}
