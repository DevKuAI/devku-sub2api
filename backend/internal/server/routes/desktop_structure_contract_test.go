package routes

import (
	"os"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDesktopStructureOpenAPIAndRegisteredRoutes(t *testing.T) {
	raw, err := os.ReadFile("../../../../docs/desktop/openapi/DESKTOP_API_V1.openapi.yaml")
	require.NoError(t, err)
	var contract struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &contract))
	router := gin.New()
	handlers := &handler.Handlers{Desktop: handler.NewDesktopHandler(nil), Admin: &handler.AdminHandlers{Desktop: adminhandler.NewDesktopHandler(nil)}}
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
	count := 0
	ids := map[string]bool{}
	for path, methods := range contract.Paths {
		for method, value := range methods {
			if method == "parameters" {
				continue
			}
			selected := strings.HasSuffix(path, "/members/usage-export") || strings.Contains(path, "/departments") || strings.Contains(path, "/positions") || strings.Contains(path, "/organization-structure/") || strings.HasSuffix(path, "/position") || strings.HasSuffix(path, "/position-history") || (method == "get" && strings.HasSuffix(path, "/members/{member_id}") && !strings.Contains(path, "/daily-reports/"))
			if !selected {
				continue
			}
			operation, ok := value.(map[string]any)
			require.True(t, ok)
			id, ok := operation["operationId"].(string)
			require.True(t, ok)
			require.NotEmpty(t, id)
			require.False(t, ids[id])
			ids[id] = true
			require.True(t, routes[strings.ToUpper(method)+" "+path], method+" "+path)
			count++
		}
	}
	require.Equal(t, 26, count)
}
