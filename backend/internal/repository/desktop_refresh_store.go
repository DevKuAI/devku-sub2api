package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	desktopRefreshTokenPrefix  = "desktop_refresh_token:"
	desktopRefreshUsedPrefix   = "desktop_refresh_used:"
	desktopMemberSessionPrefix = "desktop_member_sessions:"
	desktopSessionFamilyPrefix = "desktop_session_family:"
)

var desktopRefreshRotateScript = redis.NewScript(`
local old = KEYS[1]
local used = KEYS[2]
local token_prefix = ARGV[1]
local used_prefix = ARGV[2]
local family_prefix = ARGV[3]
local member_prefix = ARGV[4]

local replay_family = redis.call('GET', used)
if replay_family then
  local family_key = family_prefix .. replay_family
  local member = redis.call('HGET', family_key, 'member')
  local fields = redis.call('HKEYS', family_key)
  for _, token_hash in ipairs(fields) do
    if token_hash ~= 'member' and token_hash ~= 'expires_at' then
      redis.call('DEL', token_prefix .. token_hash)
    end
  end
  redis.call('DEL', family_key)
  if member then redis.call('SREM', member_prefix .. member, replay_family) end
  return {2, replay_family, member or '', '0'}
end

if redis.call('EXISTS', old) == 0 then return {0, '', '', '0'} end
local family = redis.call('HGET', old, 'family')
local member = redis.call('HGET', old, 'member')
local expires_at = redis.call('HGET', old, 'expires_at')
if not family or not member or not expires_at then return {0, '', '', '0'} end

local ttl = tonumber(expires_at) - tonumber(ARGV[6])
if ttl <= 0 then
  redis.call('DEL', old)
  return {0, '', '', '0'}
end

local family_key = family_prefix .. family
redis.call('DEL', old)
redis.call('SET', used, family, 'EX', ttl)
redis.call('HDEL', family_key, ARGV[5])
local new_hash = ARGV[7]
local new_key = token_prefix .. new_hash
redis.call('HSET', new_key, 'family', family, 'member', member, 'expires_at', expires_at)
redis.call('EXPIRE', new_key, ttl)
redis.call('HSET', family_key, new_hash, '1')
redis.call('EXPIRE', family_key, ttl)
redis.call('SADD', member_prefix .. member, family)
redis.call('EXPIRE', member_prefix .. member, ttl)
return {1, family, member, expires_at}
`)

type desktopRefreshStore struct {
	rdb *redis.Client
}

func NewDesktopRefreshStore(rdb *redis.Client) service.DesktopRefreshStore {
	return &desktopRefreshStore{rdb: rdb}
}

func (s *desktopRefreshStore) Create(ctx context.Context, tokenHash string, session service.DesktopRefreshSession) error {
	ttl := time.Until(session.AbsoluteExpiresAt)
	if ttl <= 0 {
		return service.ErrDesktopRefreshInvalid
	}
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, desktopRefreshTokenPrefix+tokenHash,
		"family", session.FamilyID,
		"member", session.MemberPublicID,
		"expires_at", session.AbsoluteExpiresAt.Unix(),
	)
	pipe.Expire(ctx, desktopRefreshTokenPrefix+tokenHash, ttl)
	pipe.HSet(ctx, desktopSessionFamilyPrefix+session.FamilyID,
		"member", session.MemberPublicID,
		"expires_at", session.AbsoluteExpiresAt.Unix(),
		tokenHash, "1",
	)
	pipe.Expire(ctx, desktopSessionFamilyPrefix+session.FamilyID, ttl)
	pipe.SAdd(ctx, desktopMemberSessionPrefix+session.MemberPublicID, session.FamilyID)
	pipe.Expire(ctx, desktopMemberSessionPrefix+session.MemberPublicID, ttl)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return service.ErrDesktopAuthStoreUnavailable.WithCause(err)
	}
	return nil
}

func (s *desktopRefreshStore) Rotate(ctx context.Context, oldTokenHash, newTokenHash string, now time.Time) (service.DesktopRefreshRotateResult, *service.DesktopRefreshSession, error) {
	value, err := desktopRefreshRotateScript.Run(ctx, s.rdb,
		[]string{desktopRefreshTokenPrefix + oldTokenHash, desktopRefreshUsedPrefix + oldTokenHash},
		desktopRefreshTokenPrefix, desktopRefreshUsedPrefix, desktopSessionFamilyPrefix, desktopMemberSessionPrefix,
		oldTokenHash, now.Unix(), newTokenHash,
	).Slice()
	if err != nil {
		return service.DesktopRefreshUnknown, nil, service.ErrDesktopAuthStoreUnavailable.WithCause(err)
	}
	if len(value) != 4 {
		return service.DesktopRefreshUnknown, nil, service.ErrDesktopAuthStoreUnavailable.WithCause(errors.New("invalid refresh rotation result"))
	}
	status, err := toInt64(value[0])
	if err != nil {
		return service.DesktopRefreshUnknown, nil, service.ErrDesktopAuthStoreUnavailable.WithCause(err)
	}
	if status == int64(service.DesktopRefreshUnknown) {
		return service.DesktopRefreshUnknown, nil, nil
	}
	session := &service.DesktopRefreshSession{FamilyID: fmt.Sprint(value[1]), MemberPublicID: fmt.Sprint(value[2])}
	if status == int64(service.DesktopRefreshRotated) {
		expiresAt, err := toInt64(value[3])
		if err != nil {
			return service.DesktopRefreshUnknown, nil, service.ErrDesktopAuthStoreUnavailable.WithCause(err)
		}
		session.AbsoluteExpiresAt = time.Unix(expiresAt, 0)
	}
	return service.DesktopRefreshRotateResult(status), session, nil
}

func (s *desktopRefreshStore) RevokeFamily(ctx context.Context, familyID string) error {
	familyKey := desktopSessionFamilyPrefix + familyID
	fields, err := s.rdb.HGetAll(ctx, familyKey).Result()
	if err != nil {
		return service.ErrDesktopAuthStoreUnavailable.WithCause(err)
	}
	pipe := s.rdb.TxPipeline()
	for tokenHash := range fields {
		if tokenHash != "member" && tokenHash != "expires_at" {
			pipe.Del(ctx, desktopRefreshTokenPrefix+tokenHash)
		}
	}
	if member := fields["member"]; member != "" {
		pipe.SRem(ctx, desktopMemberSessionPrefix+member, familyID)
	}
	pipe.Del(ctx, familyKey)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return service.ErrDesktopAuthStoreUnavailable.WithCause(err)
	}
	return nil
}

func (s *desktopRefreshStore) RevokeMember(ctx context.Context, memberPublicID string) error {
	memberKey := desktopMemberSessionPrefix + memberPublicID
	families, err := s.rdb.SMembers(ctx, memberKey).Result()
	if err != nil {
		return service.ErrDesktopAuthStoreUnavailable.WithCause(err)
	}
	for _, familyID := range families {
		if err := s.RevokeFamily(ctx, familyID); err != nil {
			return err
		}
	}
	if err := s.rdb.Del(ctx, memberKey).Err(); err != nil {
		return service.ErrDesktopAuthStoreUnavailable.WithCause(err)
	}
	return nil
}

func toInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case string:
		return strconv.ParseInt(typed, 10, 64)
	case []byte:
		return strconv.ParseInt(string(typed), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected integer type %T", value)
	}
}
