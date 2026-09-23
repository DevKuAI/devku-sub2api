package routes

import (
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/bi"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
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

func TestBIAdminRoutesMatchSupplementalOpenAPI(t *testing.T) {
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	raw, err := os.ReadFile("../../../../docs/mobile-bi/admin.openapi.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &spec))
	parameter := regexp.MustCompile(`:([a-z_]+)`)
	for _, enabled := range []bool{false, true} {
		r := gin.New()
		admin := r.Group("/api/v1/admin", func(c *gin.Context) { middleware.AbortWithError(c, 401, "UNAUTHORIZED", "Authentication required") })
		h := &handler.Handlers{BI: bi.NewHandler(nil, nil, &config.Config{}, nil, nil)}
		registerBIAdminRoutes(admin, h, &config.Config{BI: config.BIConfig{Enabled: enabled}})
		if !enabled {
			require.Empty(t, r.Routes())
			continue
		}
		seen := map[string]bool{}
		for _, route := range r.Routes() {
			path := parameter.ReplaceAllString(route.Path, "{$1}")
			rawOperation, ok := spec.Paths[path][strings.ToLower(route.Method)]
			require.True(t, ok, "%s %s", route.Method, path)
			var op struct{ OperationID string }
			require.NoError(t, json.Unmarshal(rawOperation, &op))
			require.NotEmpty(t, op.OperationID)
			require.False(t, seen[op.OperationID])
			seen[op.OperationID] = true
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(route.Method, parameter.ReplaceAllString(route.Path, "test-id"), strings.NewReader(`{}`)))
			require.Equal(t, 401, w.Code)
		}
		for _, methods := range spec.Paths {
			for method, rawOperation := range methods {
				if method == "parameters" {
					continue
				}
				var op struct{ OperationID string }
				require.NoError(t, json.Unmarshal(rawOperation, &op))
				if op.OperationID != "" {
					require.True(t, seen[op.OperationID])
				}
			}
		}
		require.Len(t, seen, 3)
	}
}
