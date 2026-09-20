package routes

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func resourceRouteHandlers() *handler.Handlers {
	svc := service.NewDesktopResourceService(nil, nil, nil)
	return &handler.Handlers{Desktop: handler.NewDesktopHandler(nil), DesktopResource: handler.NewDesktopResourceHandler(svc, nil), Admin: &handler.AdminHandlers{DesktopResource: adminhandler.NewDesktopResourceHandler(svc)}}
}

func TestDesktopResourceAdminFeatureGateAndBodyLimit(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		r := gin.New()
		h := resourceRouteHandlers()
		registerDesktopResourceAdminRoutes(r.Group("/api/v1/admin"), h, func(c *gin.Context) { c.Next() }, nil, &config.Config{Desktop: config.DesktopConfig{Enabled: enabled}})
		if !enabled {
			require.Empty(t, r.Routes())
			continue
		}
		require.Len(t, r.Routes(), 7)
		// This exceeds the old Desktop JSON limit but must reach ZIP validation, not return 413.
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/desktop/resources", strings.NewReader(strings.Repeat("x", 20<<10)))
		req.Header.Set("Content-Type", "application/zip")
		r.ServeHTTP(w, req)
		require.Equal(t, 400, w.Code)
		require.Contains(t, w.Body.String(), "RESOURCE_PACKAGE_INVALID")
	}
}

func TestDesktopResourceRequiresV2InstallationHeader(t *testing.T) {
	r := gin.New()
	h := resourceRouteHandlers()
	RegisterDesktopRoutes(r, h, &service.DesktopService{}, func(c *gin.Context) { c.Next() })
	for _, p := range []string{"/resources", "/resources/id", "/resources/id/versions/1.0.0/artifacts/darwin-arm64", "/resources/id/versions/1.0.0/artifacts/darwin-arm64/download-url"} {
		w := httptest.NewRecorder()
		method := http.MethodGet
		if strings.HasSuffix(p, "/download-url") {
			method = http.MethodPost
		}
		req := httptest.NewRequest(method, "/api/desktop/v1"+p, nil)
		req.Header.Set("Authorization", "Bearer dks_"+base64.RawURLEncoding.EncodeToString(make([]byte, 32)))
		r.ServeHTTP(w, req)
		require.Equal(t, 422, w.Code)
		require.Contains(t, w.Body.String(), "X-Installation-ID")
	}
}

func TestDesktopResourceOpenAPIRouteParity(t *testing.T) {
	raw, err := os.ReadFile("../../../../docs/openapi/desktop-resources-v1.json")
	require.NoError(t, err)
	var spec struct {
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
		} `json:"paths"`
	}
	require.NoError(t, json.Unmarshal(raw, &spec))
	r := gin.New()
	h := resourceRouteHandlers()
	audit := middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	RegisterDesktopRoutes(r, h, nil, audit)
	registerDesktopResourceAdminRoutes(r.Group("/api/v1/admin"), h, audit, nil, &config.Config{Desktop: config.DesktopConfig{Enabled: true}})
	routes := map[string]bool{}
	for _, route := range r.Routes() {
		if !strings.Contains(route.Path, "/resources") {
			continue
		}
		parts := strings.Split(route.Path, "/")
		for i, p := range parts {
			if strings.HasPrefix(p, ":") {
				parts[i] = "{" + p[1:] + "}"
			}
		}
		routes[route.Method+" "+strings.Join(parts, "/")] = true
	}
	ids := map[string]bool{}
	for p, methods := range spec.Paths {
		for method, op := range methods {
			require.NotEmpty(t, op.OperationID)
			require.False(t, ids[op.OperationID])
			ids[op.OperationID] = true
			key := strings.ToUpper(method) + " " + p
			require.True(t, routes[key], key)
			delete(routes, key)
		}
	}
	require.Len(t, ids, 11)
	require.Empty(t, routes)
}
