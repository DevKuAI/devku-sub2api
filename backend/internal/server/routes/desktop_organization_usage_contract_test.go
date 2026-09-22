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

func TestDesktopOrganizationUsageOpenAPIAndFeatureFlag(t *testing.T) {
	raw, err := os.ReadFile("../../../../docs/DESKTOP_API_V1.openapi.yaml")
	require.NoError(t, err)
	var contract struct {
		Paths map[string]struct {
			Get struct {
				OperationID string `yaml:"operationId"`
			} `yaml:"get"`
		} `yaml:"paths"`
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
			routes[route.Method+" "+strings.ReplaceAll(route.Path, ":organization_id", "{organization_id}")] = true
		}
		for path, id := range map[string]string{
			"/api/v1/admin/desktop/organizations/{organization_id}/usage/statistics": "getAdminDesktopOrganizationUsageStatistics",
			"/api/v1/desktop/organization/usage/statistics":                          "getManagedDesktopOrganizationUsageStatistics",
		} {
			require.Equal(t, id, contract.Paths[path].Get.OperationID)
			require.Equal(t, enabled, routes["GET "+path])
		}
	}
}
