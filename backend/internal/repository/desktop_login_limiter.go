package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

type desktopLoginLimiter struct {
	rdb *redis.Client
	cfg config.DesktopConfig
}

func NewDesktopLoginLimiter(rdb *redis.Client, cfg *config.Config) service.DesktopLoginLimiter {
	return &desktopLoginLimiter{rdb: rdb, cfg: cfg.Desktop}
}

func (l *desktopLoginLimiter) AllowLookup(ctx context.Context, ip, installationID string) (time.Duration, error) {
	return l.allowAll(ctx, []limitDimension{
		{key: "desktop_rate:lookup:ip:" + hashLimitValue(ip), limit: l.cfg.LookupIPPerMinute, window: time.Minute},
		{key: "desktop_rate:lookup:installation:" + hashLimitValue(installationID), limit: l.cfg.LookupIPPerMinute, window: time.Minute},
	})
}

func (l *desktopLoginLimiter) AllowLogin(ctx context.Context, ip, installationID, organizationCode, phoneHash string) (time.Duration, error) {
	failureKey := desktopPhoneFailureKey(organizationCode, phoneHash)
	if ttl, err := l.rdb.TTL(ctx, failureKey).Result(); err != nil {
		return 0, service.ErrDesktopAuthStoreUnavailable.WithCause(err)
	} else if ttl > time.Minute {
		return ttl, nil
	}
	return l.allowAll(ctx, []limitDimension{
		{key: "desktop_rate:login:ip:" + hashLimitValue(ip), limit: l.cfg.LoginIPPerMinute, window: time.Minute},
		{key: "desktop_rate:login:installation:" + hashLimitValue(installationID), limit: l.cfg.LoginIPPerMinute, window: time.Minute},
		{key: "desktop_rate:login:organization:" + hashLimitValue(organizationCode), limit: l.cfg.LoginOrganizationPerMinute, window: time.Minute},
	})
}

func (l *desktopLoginLimiter) RecordLoginFailure(ctx context.Context, organizationCode, phoneHash string) (time.Duration, error) {
	key := desktopPhoneFailureKey(organizationCode, phoneHash)
	count, err := l.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, service.ErrDesktopAuthStoreUnavailable.WithCause(err)
	}
	if count == 1 {
		if err := l.rdb.Expire(ctx, key, time.Minute).Err(); err != nil {
			return 0, service.ErrDesktopAuthStoreUnavailable.WithCause(err)
		}
	}
	if count >= int64(l.cfg.LoginPhoneFailureLimit) {
		freeze := time.Duration(l.cfg.LoginPhoneFreezeMinutes) * time.Minute
		if err := l.rdb.Expire(ctx, key, freeze).Err(); err != nil {
			return 0, service.ErrDesktopAuthStoreUnavailable.WithCause(err)
		}
		return freeze, nil
	}
	return 0, nil
}

func (l *desktopLoginLimiter) ClearLoginFailures(ctx context.Context, organizationCode, phoneHash string) error {
	if err := l.rdb.Del(ctx, desktopPhoneFailureKey(organizationCode, phoneHash)).Err(); err != nil {
		return service.ErrDesktopAuthStoreUnavailable.WithCause(err)
	}
	return nil
}

type limitDimension struct {
	key    string
	limit  int
	window time.Duration
}

func (l *desktopLoginLimiter) allowAll(ctx context.Context, dimensions []limitDimension) (time.Duration, error) {
	for _, dimension := range dimensions {
		count, err := l.rdb.Incr(ctx, dimension.key).Result()
		if err != nil {
			return 0, service.ErrDesktopAuthStoreUnavailable.WithCause(err)
		}
		if count == 1 {
			if err := l.rdb.Expire(ctx, dimension.key, dimension.window).Err(); err != nil {
				return 0, service.ErrDesktopAuthStoreUnavailable.WithCause(err)
			}
		}
		if count > int64(dimension.limit) {
			ttl, err := l.rdb.TTL(ctx, dimension.key).Result()
			if err != nil {
				return 0, service.ErrDesktopAuthStoreUnavailable.WithCause(err)
			}
			if ttl <= 0 {
				ttl = dimension.window
			}
			return ttl, nil
		}
	}
	return 0, nil
}

func desktopPhoneFailureKey(organizationCode, phoneHash string) string {
	return "desktop_rate:login:phone:" + hashLimitValue(organizationCode+"\x00"+phoneHash)
}

func hashLimitValue(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}
