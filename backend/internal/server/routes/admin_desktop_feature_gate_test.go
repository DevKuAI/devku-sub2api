package routes

import (
	"net/http"
	"net/http/httptest"
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
		{name: "enabled", enabled: true, want: 14},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42}) })
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
			result := httptest.NewRecorder()
			router.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/api/v1/admin/desktop/organizations/org_one/conversation-records/statistics?client=invalid", nil))
			if test.enabled {
				require.Equal(t, http.StatusUnprocessableEntity, result.Code, result.Body.String())
			} else {
				require.Equal(t, http.StatusNotFound, result.Code)
			}
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
