package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const desktopSessionPrefix = "desktop_session_v2:"

// All timestamps and expiry decisions use Redis time, never a client's clock.
var desktopSessionScript = redis.NewScript(`
local tm = redis.call('TIME')
local now = tonumber(tm[1]) * 1000 + math.floor(tonumber(tm[2]) / 1000)
local op = ARGV[1]
local idle = tonumber(ARGV[2])
if op == 'create' then
  if redis.call('EXISTS', KEYS[1]) == 1 then return redis.error_reply('session already exists') end
  redis.call('HSET', KEYS[1], 'identity', ARGV[3], 'created', now, 'active', now, 'expires', now + idle)
  redis.call('PEXPIREAT', KEYS[1], now + idle)
  return {ARGV[3], tostring(now), tostring(now), tostring(now + idle)}
end
local fields = redis.call('HMGET', KEYS[1], 'identity', 'created', 'active', 'expires')
if not fields[1] then return {} end
if not fields[4] or now >= tonumber(fields[4]) then
  redis.call('DEL', KEYS[1])
  return {}
end
if op == 'touch' then
  now = math.max(now, tonumber(fields[3]))
  redis.call('HSET', KEYS[1], 'active', now, 'expires', now + idle)
  redis.call('PEXPIREAT', KEYS[1], now + idle)
  fields[3] = tostring(now)
  fields[4] = tostring(now + idle)
elseif op == 'delete' then
  redis.call('DEL', KEYS[1])
end
return fields
`)

type desktopSessionStore struct{ rdb *redis.Client }

func NewDesktopSessionStore(rdb *redis.Client) service.DesktopSessionStore {
	return &desktopSessionStore{rdb: rdb}
}

func (s *desktopSessionStore) run(ctx context.Context, hash, op string, identity []byte) (*service.DesktopSession, error) {
	values, err := desktopSessionScript.Run(ctx, s.rdb, []string{desktopSessionPrefix + hash}, op, service.DesktopSessionIdleTimeout.Milliseconds(), string(identity)).Slice()
	if err != nil {
		return nil, service.ErrDesktopAuthStoreUnavailable
	}
	if len(values) == 0 {
		return nil, service.ErrDesktopUnauthenticated
	}
	if len(values) != 4 {
		return nil, service.ErrDesktopAuthStoreUnavailable
	}
	var session service.DesktopSession
	raw, ok := values[0].(string)
	if !ok || json.Unmarshal([]byte(raw), &session) != nil {
		return nil, service.ErrDesktopAuthStoreUnavailable
	}
	times := []*time.Time{&session.CreatedAt, &session.LastActiveAt, &session.IdleExpiresAt}
	for i, target := range times {
		millis, err := toInt64(values[i+1])
		if err != nil {
			return nil, service.ErrDesktopAuthStoreUnavailable
		}
		*target = time.UnixMilli(millis).UTC()
	}
	return &session, nil
}

func (s *desktopSessionStore) Create(ctx context.Context, hash string, session *service.DesktopSession) error {
	raw, err := json.Marshal(session)
	if err != nil {
		return service.ErrDesktopAuthStoreUnavailable
	}
	created, err := s.run(ctx, hash, "create", raw)
	if err == nil {
		*session = *created
	}
	return err
}
func (s *desktopSessionStore) Get(ctx context.Context, hash string) (*service.DesktopSession, error) {
	return s.run(ctx, hash, "get", nil)
}
func (s *desktopSessionStore) Touch(ctx context.Context, hash string) error {
	_, err := s.run(ctx, hash, "touch", nil)
	return err
}
func (s *desktopSessionStore) Delete(ctx context.Context, hash string) error {
	_, err := s.run(ctx, hash, "delete", nil)
	return err
}
