//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDesktopStructureHTTPActualTypeScriptClient(t *testing.T) {
	frontend, err := filepath.Abs("../../../frontend")
	require.NoError(t, err)
	vitest := filepath.Join(frontend, "node_modules/vitest/vitest.mjs")
	if _, err = os.Stat(vitest); err != nil {
		if os.Getenv("DESKTOP_REQUIRE_WEB_HTTP") == "1" {
			t.Fatal("frontend dependencies are required for Web HTTP integration")
		}
		t.Skip("install frontend dependencies to run the actual Web TypeScript client")
	}
	// Reading the client inputs also makes Go's test cache observe client changes.
	for _, name := range []string{"src/api/desktopStructure.ts", "src/api/desktopOrganization.ts", "src/api/admin/desktop.ts", "src/api/client.ts", "src/api/url.ts", "src/utils/authStorage.ts", "tests/desktop-structure/client.http.ts", "tests/desktop-structure/vitest.config.ts"} {
		_, err = os.ReadFile(filepath.Join(frontend, name))
		require.NoError(t, err)
	}
	f := newDesktopRepositoryFixture(t, "webhttp", 10)
	member := f.createMember(t, "webhttp", 1)
	cfg := &config.Config{Desktop: config.DesktopConfig{Enabled: true}}
	keys := service.NewAPIKeyService(f.apiKeys, nil, nil, nil, nil, nil, cfg)
	t.Cleanup(keys.StopAuthCacheInvalidationSubscriber)
	svc := service.NewDesktopService(f.repo, nil, nil, nil, nil, keys, cfg, nil, nil, nil)
	previous := service.DefaultIdempotencyCoordinator()
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(NewIdempotencyRepository(integrationEntClient, integrationDB), service.DefaultIdempotencyConfig()))
	t.Cleanup(func() { service.SetDefaultIdempotencyCoordinator(previous) })
	router := gin.New()
	router.Use(middleware.RequestLogger(), func(c *gin.Context) {
		if c.GetHeader("Authorization") != "Bearer desktop-structure-http-test" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if (c.Request.Method == "POST" || c.Request.Method == "PUT") && !strings.HasSuffix(c.Request.URL.Path, "/members/usage-export") && c.GetHeader("Idempotency-Key") == "" {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: f.user.ID})
		c.Next()
	})
	group := router.Group("/api/v1/desktop/organization")
	group.GET("", handler.NewDesktopHandler(svc).GetManagedOrganization)
	group.POST("/members", handler.NewDesktopHandler(svc).CreateManagedMember)
	handler.NewDesktopStructureHandler(svc, true).Register(group)
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	node := exec.CommandContext(ctx, "node", vitest, "run", "--config", "tests/desktop-structure/vitest.config.ts")
	node.Dir = frontend
	node.Env = append(os.Environ(), "DESKTOP_STRUCTURE_HTTP_URL="+server.URL+"/api/v1", "DESKTOP_STRUCTURE_HTTP_MEMBER="+member.PublicID, "DESKTOP_STRUCTURE_HTTP_ORGANIZATION="+f.organization.PublicID, "DESKTOP_STRUCTURE_HTTP_PHONE="+desktopTestPhone(f.suffix+2))
	out, err := node.CombinedOutput()
	require.NoError(t, err, string(out))
	require.Contains(t, string(out), "Desktop structure TypeScript HTTP flow verified")
}

func TestDesktopStructureHTTPAuthorizationIdempotencyAndSafeDTO(t *testing.T) {
	f := newDesktopRepositoryFixture(t, "structhttp", 10)
	other := newDesktopRepositoryFixture(t, "structhttpother", 10)
	member := f.createMember(t, "http", 1)
	structure := f.repo.(service.DesktopStructureRepository)
	target := createDesktopTestUnit(t, structure, f.organization.PublicID, service.DesktopPositionKind, f.position.Department.PublicID, "HTTP Target")
	desktop := service.NewDesktopService(f.repo, nil, nil, nil, nil, nil, &config.Config{Desktop: config.DesktopConfig{Enabled: true}}, nil, nil, nil)
	h := handler.NewDesktopStructureHandler(desktop, true)
	previous := service.DefaultIdempotencyCoordinator()
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(NewIdempotencyRepository(integrationEntClient, integrationDB), service.DefaultIdempotencyConfig()))
	t.Cleanup(func() { service.SetDefaultIdempotencyCoordinator(previous) })
	userID := f.user.ID
	router := gin.New()
	router.Use(middleware.RequestLogger(), func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID})
		c.Next()
	})
	router.GET("/members/:member_id", h.GetMember)
	router.PUT("/members/:member_id/position", h.Assign)
	router.POST("/positions", h.Unit(service.DesktopPositionKind))
	router.GET("/departments", h.Unit(service.DesktopDepartmentKind))
	router.GET("/positions", h.Unit(service.DesktopPositionKind))
	perform := func(method, path, body, key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Request-ID", "structure-http-request")
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	for _, kind := range []string{"departments", "positions"} {
		counted := perform("GET", "/"+kind+"?include_member_count=true", "", "")
		require.Equal(t, http.StatusOK, counted.Code, counted.Body.String())
		require.Contains(t, counted.Body.String(), `"member_count":1`)
		options := perform("GET", "/"+kind+"?include_member_count=false", "", "")
		require.Equal(t, http.StatusOK, options.Code, options.Body.String())
		require.NotContains(t, options.Body.String(), `"member_count":1`)
		invalid := perform("GET", "/"+kind+"?include_member_count=invalid", "", "")
		require.Equal(t, http.StatusUnprocessableEntity, invalid.Code, invalid.Body.String())
	}
	payload, _ := json.Marshal(map[string]any{"position_id": target.PublicID, "expected_revision": 1, "reason": "Confirmed"})
	key := "structure-http-" + f.organization.PublicID
	response := perform("PUT", "/members/"+member.PublicID+"/position", string(payload), key)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	response = perform("PUT", "/members/"+member.PublicID+"/position", string(payload), key)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, "true", response.Header().Get("X-Idempotency-Replayed"))
	require.NotContains(t, response.Body.String(), member.CurrentAPIKey)
	require.NotContains(t, response.Body.String(), "gateway_user")
	response = perform("PUT", "/members/"+member.PublicID+"/position", strings.Replace(string(payload), target.PublicID, f.position.PublicID, 1), key)
	require.Equal(t, http.StatusConflict, response.Code, response.Body.String())
	response = perform("POST", "/positions", `{"name":"Bad","department_id":"`+f.position.Department.PublicID+`","organization_id":"other"}`, "unknown-field")
	require.Equal(t, http.StatusUnprocessableEntity, response.Code, response.Body.String())
	userID = other.user.ID
	response = perform("GET", "/members/"+member.PublicID, "", "")
	require.Equal(t, http.StatusNotFound, response.Code, response.Body.String())
	response = perform("PUT", "/members/"+member.PublicID+"/position", string(payload), key)
	require.Equal(t, http.StatusNotFound, response.Code, response.Body.String())
	userID = f.user.ID
	_, err := integrationDB.Exec("UPDATE desktop_organizations SET status='disabled' WHERE id=$1", f.organization.ID)
	require.NoError(t, err)
	response = perform("PUT", "/members/"+member.PublicID+"/position", string(payload), key)
	require.Equal(t, http.StatusConflict, response.Code, response.Body.String())
	require.NotEqual(t, "true", response.Header().Get("X-Idempotency-Replayed"))
	var requestID string
	require.NoError(t, integrationDB.QueryRow("SELECT request_id FROM desktop_member_position_history WHERE member_id=$1 AND position_revision=2", member.ID).Scan(&requestID))
	require.Equal(t, "structure-http-request", requestID)
}
