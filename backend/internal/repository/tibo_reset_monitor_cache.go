package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const tiboResetMonitorCacheKey = "sub2api:tibo:codex-resets:v1"

type tiboResetMonitorCache struct{ rdb *redis.Client }

func NewTiboResetMonitorCache(rdb *redis.Client) service.TiboResetMonitorCache {
	return &tiboResetMonitorCache{rdb: rdb}
}

func (c *tiboResetMonitorCache) Read(ctx context.Context) ([]byte, error) {
	raw, err := c.rdb.Get(ctx, tiboResetMonitorCacheKey).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return raw, err
}

func (c *tiboResetMonitorCache) Write(ctx context.Context, value []byte, ttl time.Duration) error {
	return c.rdb.Set(ctx, tiboResetMonitorCacheKey, value, ttl).Err()
}
