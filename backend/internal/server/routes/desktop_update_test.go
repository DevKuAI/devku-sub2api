package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDesktopUpdateRoutesAreIndependentOfDesktopFeatureFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{
		DesktopUpdate: handler.NewDesktopUpdateHandler(nil),
		Admin: &handler.AdminHandlers{
			DesktopUpdate: adminhandler.NewDesktopUpdateHandler(nil),
		},
	}

	RegisterDesktopUpdateRoutes(router, handlers)
	registerDesktopUpdateAdminRoutes(router.Group("/api/v1/admin"), handlers, nil)

	routes := make(map[string]bool)
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	require.True(t, routes["GET /api/desktop/v1/update/:target/:arch/:current_version"])
	for _, route := range []string{
		"GET /api/v1/admin/desktop/updates",
		"GET /api/v1/admin/desktop/updates/:release_id",
		"POST /api/v1/admin/desktop/updates",
		"PATCH /api/v1/admin/desktop/updates/:release_id",
		"POST /api/v1/admin/desktop/updates/:release_id/artifacts/:platform",
		"POST /api/v1/admin/desktop/updates/:release_id/publish",
		"POST /api/v1/admin/desktop/updates/:release_id/withdraw",
	} {
		require.Truef(t, routes[route], "%s should be registered", route)
	}
}
