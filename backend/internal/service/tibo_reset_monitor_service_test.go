package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type tiboResetMonitorMemoryCache struct{ value []byte }

func (c *tiboResetMonitorMemoryCache) Read(context.Context) ([]byte, error) { return c.value, nil }
func (c *tiboResetMonitorMemoryCache) Write(_ context.Context, value []byte, _ time.Duration) error {
	c.value = append([]byte(nil), value...)
	return nil
}

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

	cache := &tiboResetMonitorMemoryCache{}
	monitor := NewTiboResetMonitorService(cache)
	if _, err := monitor.Get(context.Background()); err != nil {
		t.Fatalf("first Get() error = %v", err)
	}
	if _, err := monitor.Get(context.Background()); err != nil {
		t.Fatalf("second Get() error = %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1", got)
	}
	if len(cache.value) == 0 {
		t.Fatal("cache is empty after successful fetch")
	}
}
