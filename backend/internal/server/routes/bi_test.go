package routes

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/bi"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestBIRoutesAreFeatureGatedAndPreserveWebErrorContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redisServer := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	for _, enabled := range []bool{false, true} {
		cfg := &config.Config{BI: config.BIConfig{Enabled: enabled, MinClientVersion: "0.1.0"}}
		settings := service.NewSettingService(&biPrivacyRepo{values: map[string]string{
			service.SettingKeyFrontendURL:     "https://unrelated.example.com",
			service.SettingKeyBIPrivacyNotice: `{"site_url":"https://example.com","title":"Privacy","version":"1","content_md":"Privacy content"}`,
		}}, cfg)
		h := bi.NewHandler(nil, rdb, cfg, nil, nil, settings)
		r := gin.New()
		deny := func(c *gin.Context) { middleware.AbortWithError(c, 401, "TOKEN_EXPIRED", "Expired") }
		RegisterBIRoutes(r, h, middleware.JWTAuthMiddleware(deny), cfg)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/bi/v1/bootstrap", nil))
		if !enabled {
			require.Equal(t, 404, w.Code)
			require.Empty(t, r.Routes())
			continue
		}
		require.Equal(t, 200, w.Code)
		require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
		require.JSONEq(t, `{"api_version":"v1","min_client_version":"0.1.0","timezone":"Asia/Shanghai","demo_available":false,"binding_enabled":true,"privacy_notice_version":"1","privacy_notice_url":"https://example.com/legal/bi-privacy"}`, w.Body.String())
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/api/bi/v1/auth/bindings/approve", strings.NewReader(`{}`)))
		require.Equal(t, 401, w.Code)
		var body bi.Error
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.Equal(t, "UNAUTHENTICATED", body.Code)
		require.Equal(t, w.Header().Get("X-Request-ID"), body.RequestID)
		r.GET("/legacy", deny)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/legacy", nil))
		require.JSONEq(t, `{"code":"TOKEN_EXPIRED","message":"Expired"}`, w.Body.String())
	}
}

// Only the setting operations exercised by the HTTP flow are implemented.
type biPrivacyRepo struct {
	service.SettingRepository
	values map[string]string
}

func (r *biPrivacyRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		out[key] = r.values[key]
	}
	return out, nil
}

func (r *biPrivacyRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}
