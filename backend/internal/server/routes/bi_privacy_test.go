package routes

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/bi"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestBIPrivacyAdminPublishPublicReadAndBootstrap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &biPrivacyRepo{values: map[string]string{service.SettingKeyFrontendURL: "https://web.example.com"}}
	cfg := &config.Config{Server: config.ServerConfig{FrontendURL: "https://fallback.example.com"}, BI: config.BIConfig{MinClientVersion: "0.1.0"}}
	settings := service.NewSettingService(repo, cfg)
	h := &handler.Handlers{Admin: &handler.AdminHandlers{Setting: adminhandler.NewSettingHandler(settings, nil, nil, nil, nil, nil, nil)}}
	r := gin.New()
	group := r.Group("/api/v1/admin", func(c *gin.Context) {
		if c.GetHeader("Authorization") != "test-admin" {
			response.Unauthorized(c, "Unauthorized")
			c.Abort()
		}
	})
	registerSettingsRoutes(group, h)
	r.GET("/api/v1/settings/bi-privacy", handler.NewSettingHandler(settings, "test").GetBIPrivacyNotice)
	r.GET("/api/bi/v1/bootstrap", bi.Headers, bi.NewHandler(nil, nil, cfg, nil, nil, settings).Bootstrap)
	var spec struct {
		Paths map[string]map[string]struct {
			OperationID string `yaml:"operationId"`
		} `yaml:"paths"`
	}
	raw, err := os.ReadFile("../../../../docs/mobile-bi/privacy-notice.openapi.yaml")
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(raw, &spec))
	operations := 0
	for _, route := range r.Routes() {
		if strings.HasSuffix(route.Path, "/settings/bi-privacy") {
			require.NotEmpty(t, spec.Paths[route.Path][strings.ToLower(route.Method)].OperationID)
			operations++
		}
	}
	require.Equal(t, 3, operations)
	request := func(method, path, body string, admin bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-Host", "attacker.example.com")
		if admin {
			req.Header.Set("Authorization", "test-admin")
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	adminPath := "/api/v1/admin/settings/bi-privacy"
	publicPath := "/api/v1/settings/bi-privacy"
	bootstrapPath := "/api/bi/v1/bootstrap"
	for _, method := range []string{"GET", "PUT"} {
		require.Equal(t, 401, request(method, adminPath, `{}`, false).Code)
	}
	require.Equal(t, 200, request("GET", adminPath, "", true).Code)
	require.Equal(t, 404, request("GET", publicPath, "", false).Code)
	require.Equal(t, 503, request("GET", bootstrapPath, "", false).Code)
	require.Equal(t, 400, request("PUT", adminPath, `{"title":"Privacy"}`, true).Code)
	for _, version := range []string{"v1", "v2"} {
		payload := `{"site_url":"https://example.com","title":"Privacy","version":"` + version + `","content_md":"# Content ` + version + `"}`
		rec := request("PUT", adminPath, payload, true)
		require.Equal(t, 200, rec.Code, rec.Body.String())
		public := request("GET", publicPath, "", false)
		require.Equal(t, 200, public.Code)
		require.JSONEq(t, rec.Body.String(), public.Body.String())
		require.Equal(t, "no-store", public.Header().Get("Cache-Control"))
		bootstrap := request("GET", bootstrapPath, "", false)
		require.Equal(t, 200, bootstrap.Code)
		var data map[string]any
		require.NoError(t, json.Unmarshal(bootstrap.Body.Bytes(), &data))
		require.Equal(t, version, data["privacy_notice_version"])
		require.Equal(t, "https://example.com/legal/bi-privacy", data["privacy_notice_url"])
	}
	repo.values[service.SettingKeyFrontendURL] = "http://example.com"
	require.Equal(t, 200, request("GET", bootstrapPath, "", false).Code)
	moved := request("PUT", adminPath, `{"site_url":"https://new.example.com/","title":"Privacy","version":"v2","content_md":"# Content v2"}`, true)
	require.Equal(t, 200, moved.Code, moved.Body.String())
	require.Contains(t, request("GET", bootstrapPath, "", false).Body.String(), `"privacy_notice_url":"https://new.example.com/legal/bi-privacy"`)
	invalid := request("PUT", adminPath, `{"site_url":"http://insecure.example.com","title":"Privacy","version":"v2","content_md":"# Content v2"}`, true)
	require.Equal(t, 400, invalid.Code)
	require.Contains(t, invalid.Body.String(), "INVALID_BI_PRIVACY_SITE_URL")
	require.Contains(t, request("GET", bootstrapPath, "", false).Body.String(), `"privacy_notice_url":"https://new.example.com/legal/bi-privacy"`)
	// Existing records without a BI origin must not inherit the web origin.
	repo.values[service.SettingKeyBIPrivacyNotice] = `{"title":"Privacy","version":"v2","content_md":"# Content v2"}`
	repo.values[service.SettingKeyFrontendURL] = "https://web.example.com"
	require.Equal(t, 503, request("GET", bootstrapPath, "", false).Code)
	require.Equal(t, 200, request("GET", publicPath, "", false).Code)
}
