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

func TestDesktopReportRoutesMatchOpenAPIAndFeatureFlag(t *testing.T) {
	raw, err := os.ReadFile("../../../../docs/DESKTOP_API_V1.openapi.yaml")
	require.NoError(t, err)
	var contract struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &contract))
	gin.SetMode(gin.TestMode)
	for _, enabled := range []bool{false, true} {
		router := gin.New()
		h := &handler.Handlers{Desktop: handler.NewDesktopHandler(nil), Admin: &handler.AdminHandlers{Desktop: adminhandler.NewDesktopHandler(nil)}}
		cfg := &config.Config{Desktop: config.DesktopConfig{Enabled: enabled}}
		registerDesktopAdminRoutesIfEnabled(router.Group("/api/v1/admin"), h, func(c *gin.Context) { c.Next() }, nil, cfg)
		registerDesktopUserRoutesIfEnabled(router.Group("/api/v1"), h, cfg)
		routes := map[string]bool{}
		for _, route := range router.Routes() {
			path := route.Path
			for _, parameter := range []string{"organization_id", "date", "member_id"} {
				path = strings.ReplaceAll(path, ":"+parameter, "{"+parameter+"}")
			}
			routes[route.Method+" "+path] = true
		}
		count := 0
		for path, methods := range contract.Paths {
			if !strings.Contains(path, "/daily-reports") {
				continue
			}
			for _, method := range []string{"get", "post"} {
				if _, ok := methods[method]; ok {
					require.Equal(t, enabled, routes[strings.ToUpper(method)+" "+path])
					count++
				}
			}
		}
		require.Equal(t, 9, count)
	}
}
