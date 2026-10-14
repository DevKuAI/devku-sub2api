package bi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func rateLimitRouter(t *testing.T) *gin.Engine {
	t.Helper()
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	h := &Handler{redis: client}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Boundary)
	ip := func(*gin.Context) string { return "192.0.2.10" }
	auth := r.Group("/api/bi/v1/auth", h.RateLimit(ip))
	auth.POST("/wechat", func(c *gin.Context) { c.Status(204) })
	auth.POST("/refresh", func(c *gin.Context) { c.Status(204) })
	invitations := r.Group("/api/bi/v1/invitations", func(c *gin.Context) {
		c.Set(principalKey, Principal{UserID: c.GetHeader("X-Test-User")})
		c.Next()
	}, h.RateLimit(ip))
	invitations.POST("/preview", func(c *gin.Context) { c.Status(204) })
	invitations.POST("/accept", func(c *gin.Context) { c.Status(204) })
	return r
}

func rateRequest(r *gin.Engine, path, user string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.Header.Set("X-Test-User", user)
	r.ServeHTTP(w, req)
	return w
}

func TestBIRateLimitOfficeOnboardingDoesNotSharePersonalQuota(t *testing.T) {
	r := rateLimitRouter(t)
	for i := 0; i < 20; i++ {
		for _, path := range []string{"/auth/wechat", "/invitations/preview", "/invitations/accept"} {
			w := rateRequest(r, "/api/bi/v1"+path, fmt.Sprintf("biu_%d", i))
			require.Equal(t, 204, w.Code, "normal office onboarding: %s user %d", path, i)
		}
	}
}

func TestBIRateLimitInvitationAbuseIsIdentityScoped(t *testing.T) {
	r := rateLimitRouter(t)
	for i := 0; i < 30; i++ {
		require.Equal(t, 204, rateRequest(r, "/api/bi/v1/invitations/preview", "biu_one").Code)
	}
	blocked := rateRequest(r, "/api/bi/v1/invitations/preview", "biu_one")
	require.Equal(t, 429, blocked.Code)
	require.NotEmpty(t, blocked.Header().Get("Retry-After"))
	require.Contains(t, blocked.Body.String(), "RATE_LIMITED")
	require.Equal(t, 204, rateRequest(r, "/api/bi/v1/invitations/preview", "biu_other").Code)
	require.Equal(t, 204, rateRequest(r, "/api/bi/v1/auth/refresh", "").Code)
}
