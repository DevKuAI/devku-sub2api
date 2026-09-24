package routes

import (
	"encoding/json"
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

func TestDesktopV2OpenAPIRegisteredOperations(t *testing.T) {
	raw, err := os.ReadFile("../../../../docs/DESKTOP_SESSION_CONVERSATION_V2.openapi.json")
	require.NoError(t, err)
	var contract struct {
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
		} `json:"paths"`
	}
	require.NoError(t, json.Unmarshal(raw, &contract))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{Desktop: handler.NewDesktopHandler(nil), Admin: &handler.AdminHandlers{Desktop: adminhandler.NewDesktopHandler(nil)}}
	RegisterDesktopRoutes(router, handlers, nil, middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }))
	RegisterDesktopDirectWebhookRoute(router, handlers, middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }))
	registerDesktopAdminRoutes(router.Group("/api/v1/admin/desktop"), handlers)
	registerDesktopUserRoutesIfEnabled(router.Group("/api/v1"), handlers, &config.Config{Desktop: config.DesktopConfig{Enabled: true}})
	routes := map[string]bool{}
	for _, route := range router.Routes() {
		parts := strings.Split(route.Path, "/")
		for i, part := range parts {
			if strings.HasPrefix(part, ":") {
				parts[i] = "{" + part[1:] + "}"
			}
		}
		routes[route.Method+" "+strings.Join(parts, "/")] = true
	}
	ids := map[string]bool{}
	for path, methods := range contract.Paths {
		for method, operation := range methods {
			require.NotEmpty(t, operation.OperationID)
			require.False(t, ids[operation.OperationID], "duplicate operation ID")
			ids[operation.OperationID] = true
			require.True(t, routes[strings.ToUpper(method)+" "+path], "unregistered contract operation: %s %s", method, path)
		}
	}
	require.Len(t, ids, 10)
}
