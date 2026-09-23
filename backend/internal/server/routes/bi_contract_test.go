package routes

import (
	"encoding/base64"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/bi"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestBIRoutesMatchEveryVersion110OperationAndProtectBearerRoutes(t *testing.T) {
	var spec struct {
		Info struct {
			Version string `yaml:"version"`
		} `yaml:"info"`
		Paths map[string]map[string]struct {
			OperationID string                `yaml:"operationId"`
			Security    []map[string][]string `yaml:"security"`
		} `yaml:"paths"`
	}
	raw, err := os.ReadFile("../../../../docs/mobile-bi/openapi.yaml")
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(raw, &spec))
	require.Equal(t, "1.1.0", spec.Info.Version)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	store := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: store.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cfg := &config.Config{BI: config.BIConfig{Enabled: true, JWTSecret: base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", 32)))}}
	h := bi.NewHandler(nil, client, cfg, nil, nil)
	RegisterBIRoutes(r, h, middleware.JWTAuthMiddleware(func(c *gin.Context) { middleware.AbortWithError(c, 401, "UNAUTHORIZED", "Authentication required") }), cfg)
	parameter := regexp.MustCompile(`:([a-z_]+)`)
	registered := map[string]bool{}
	for _, route := range r.Routes() {
		path := parameter.ReplaceAllString(route.Path, "{$1}")
		operation, ok := spec.Paths[path][strings.ToLower(route.Method)]
		require.True(t, ok, "uncontracted route %s %s", route.Method, path)
		registered[operation.OperationID] = true
		if len(operation.Security) > 0 {
			w := httptest.NewRecorder()
			target := parameter.ReplaceAllString(route.Path, "test-id")
			r.ServeHTTP(w, httptest.NewRequest(route.Method, target, strings.NewReader(`{}`)))
			require.Equal(t, 401, w.Code, "unprotected %s: %s", operation.OperationID, w.Body.String())
			require.Contains(t, w.Body.String(), `"code":"UNAUTHENTICATED"`)
		}
	}
	count := 0
	for path, methods := range spec.Paths {
		for method, operation := range methods {
			if operation.OperationID == "" {
				continue
			}
			count++
			require.True(t, registered[operation.OperationID], "missing %s %s (%s)", method, path, operation.OperationID)
		}
	}
	require.Equal(t, 59, count)
}
