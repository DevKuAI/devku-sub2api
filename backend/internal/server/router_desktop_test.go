package server

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDesktopAPIRoutesFollowFeatureFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name    string
		enabled bool
		want    []string
	}{
		{name: "disabled"},
		{
			name:    "enabled",
			enabled: true,
			want: []string{
				"POST /api/desktop/v1/auth/organization-lookup",
				"POST /api/desktop/v1/auth/login",
				"POST /api/desktop/v1/auth/refresh",
				"POST /api/desktop/v1/auth/logout",
				"GET /api/desktop/v1/me",
				"GET /api/desktop/v1/model-configuration",
				"GET /api/desktop/v1/usage/summary",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			handlers := &handler.Handlers{Desktop: handler.NewDesktopHandler(nil)}
			audit := middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
			cfg := &config.Config{Desktop: config.DesktopConfig{Enabled: test.enabled}}

			registerDesktopRoutesIfEnabled(router, handlers, audit, cfg)

			registered := make(map[string]bool)
			for _, route := range router.Routes() {
				registered[route.Method+" "+route.Path] = true
			}
			if !test.enabled {
				require.Empty(t, registered)
				return
			}
			for _, route := range test.want {
				require.True(t, registered[route], "%s should be registered", route)
			}
			require.Len(t, registered, len(test.want))
		})
	}
}
