package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDesktopUserOrganizationSettingsAreReadOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, role := range []string{"user", "admin"} {
		t.Run(role, func(t *testing.T) {
			router := gin.New()
			router.Use(gin.Recovery())
			authenticated := router.Group("/api/v1", func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
				c.Set(string(middleware.ContextKeyUserRole), role)
			})
			handlers := &handler.Handlers{Desktop: handler.NewDesktopHandler(nil)}
			cfg := &config.Config{Desktop: config.DesktopConfig{Enabled: true}}
			registerDesktopUserRoutesIfEnabled(authenticated, handlers, cfg)

			for _, test := range []struct {
				name, method, path, body, reason string
			}{
				{
					name: "organization", method: http.MethodPatch, path: "/api/v1/desktop/organization",
					body:   `{"name":"Changed","status":"disabled","gateway_user_id":99,"group_id":99,"member_limit":100}`,
					reason: "ORGANIZATION_READ_ONLY",
				},
				{
					name: "configuration", method: http.MethodPut, path: "/api/v1/desktop/organization/model-configuration",
					body:   `{"target_config":{"schema_version":1,"targets":{"chatgpt_codex":{"enabled":true,"provider_id":"openai","display_name":"Codex","requested_model":"model-one","wire_api":"responses"}}}}`,
					reason: "MODEL_CONFIGURATION_READ_ONLY",
				},
			} {
				t.Run(test.name, func(t *testing.T) {
					request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
					request.Header.Set("Content-Type", "application/json")
					recorder := httptest.NewRecorder()
					router.ServeHTTP(recorder, request)

					require.Equal(t, http.StatusForbidden, recorder.Code)
					require.Contains(t, recorder.Body.String(), `"reason":"`+test.reason+`"`)
				})
			}
		})
	}
}

func TestDesktopUserRoutesFollowFeatureFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, test := range []struct {
		name    string
		enabled bool
		want    int
	}{
		{name: "disabled"},
		{name: "enabled", enabled: true, want: 8},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			handlers := &handler.Handlers{Desktop: handler.NewDesktopHandler(nil)}
			cfg := &config.Config{Desktop: config.DesktopConfig{Enabled: test.enabled}}

			registerDesktopUserRoutesIfEnabled(router.Group("/api/v1"), handlers, cfg)

			registered := 0
			for _, route := range router.Routes() {
				if strings.HasPrefix(route.Path, "/api/v1/desktop/organization") {
					registered++
				}
			}
			require.Equal(t, test.want, registered)
		})
	}
}
