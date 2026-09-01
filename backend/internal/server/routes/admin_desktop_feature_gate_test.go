package routes

import (
	"os"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDesktopAdminRoutesFollowFeatureFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name    string
		enabled bool
		want    int
	}{
		{name: "disabled"},
		{name: "enabled", enabled: true, want: 10},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			handlers := &handler.Handlers{Admin: &handler.AdminHandlers{Desktop: adminhandler.NewDesktopHandler(nil)}}
			audit := middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
			cfg := &config.Config{Desktop: config.DesktopConfig{Enabled: test.enabled}}

			registerDesktopAdminRoutesIfEnabled(router.Group("/api/v1/admin"), handlers, audit, nil, cfg)

			registered := 0
			for _, route := range router.Routes() {
				if strings.HasPrefix(route.Path, "/api/v1/admin/desktop/") {
					registered++
				}
			}
			require.Equal(t, test.want, registered)
		})
	}
}

func TestDesktopUpdateAdminRoutesOnlyUseAdminAuthAndAuditGuards(t *testing.T) {
	content, err := os.ReadFile("admin.go")
	require.NoError(t, err)
	source := string(content)

	updateRoutes := strings.Index(source, "registerDesktopUpdateAdminRoutes(admin, h, cfg)")
	auditGuard := strings.Index(source, "admin.Use(gin.HandlerFunc(auditLog))")
	complianceGuard := strings.Index(source, "admin.Use(middleware.AdminComplianceGuard(settingService))")
	require.NotEqual(t, -1, updateRoutes)
	require.Less(t, auditGuard, updateRoutes)
	require.Less(t, updateRoutes, complianceGuard)
}
