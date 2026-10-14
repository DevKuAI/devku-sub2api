package bi

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// A shared office gets an outer budget; operations and identities get independent budgets.
var rateLimitScript = redis.NewScript(`
local wait = 0
for i, key in ipairs(KEYS) do
 local n = redis.call('INCR', key)
 if n == 1 then redis.call('EXPIRE', key, 60) end
 if n > tonumber(ARGV[i]) then wait = math.max(wait, redis.call('TTL', key), 1) end
end
return wait`)

func (h *Handler) RateLimit(clientIP func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.redis == nil {
			WriteError(c, apiError(503, "DATA_UNAVAILABLE", "Authentication unavailable"))
			return
		}
		ip := tokenHash(clientIP(c))
		operation := tokenHash(c.Request.Method + ":" + c.FullPath())
		keys := []string{"bi:limit:ip:" + ip, "bi:limit:operation:" + operation + ":" + ip}
		limits := []any{600, 120}
		if user := principal(c).UserID; user != "" {
			keys = append(keys, "bi:limit:user:"+operation+":"+tokenHash(user))
			limits[1] = 300
			limits = append(limits, 30)
		}
		wait, err := rateLimitScript.Run(c.Request.Context(), h.redis, keys, limits...).Int64()
		if err != nil {
			WriteError(c, apiError(503, "DATA_UNAVAILABLE", "Authentication unavailable"))
			return
		}
		if wait > 0 {
			c.Header("Retry-After", strconv.FormatInt(wait, 10))
			limited := apiError(429, "RATE_LIMITED", "Too many requests")
			limited.Retryable = true
			WriteError(c, limited)
			return
		}
		c.Next()
	}
}
