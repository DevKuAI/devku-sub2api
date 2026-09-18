package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

var desktopConversationLimitScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then redis.call('PEXPIRE', KEYS[1], 60000) end
if count > tonumber(ARGV[1]) then return math.max(1, redis.call('PTTL', KEYS[1])) end
return 0
`)

type desktopConversationLimiter struct {
	rdb   *redis.Client
	limit int
}

func NewDesktopConversationLimiter(rdb *redis.Client, cfg *config.Config) service.DesktopConversationLimiter {
	return &desktopConversationLimiter{rdb: rdb, limit: cfg.Desktop.ConversationMemberPerMinute}
}

func (l *desktopConversationLimiter) Allow(ctx context.Context, memberID string) (time.Duration, error) {
	limit := l.limit
	if limit <= 0 {
		limit = 60
	}
	millis, err := desktopConversationLimitScript.Run(ctx, l.rdb, []string{"desktop_conversation_rate:" + hashLimitValue(memberID)}, limit).Int64()
	if err != nil {
		return 0, service.ErrDesktopAuthStoreUnavailable
	}
	return time.Duration(millis) * time.Millisecond, nil
}
