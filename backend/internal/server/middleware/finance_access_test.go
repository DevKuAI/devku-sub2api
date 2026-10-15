//go:build unit

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestFinanceAccessUsesCurrentRoleAndRestrictsRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, role := range []string{service.RoleFinance, service.RoleUser, service.RoleAdmin} {
		t.Run(role, func(t *testing.T) {
			user := &service.User{ID: 7, Role: role, Status: service.StatusActive}
			cfg := &config.Config{JWT: config.JWTConfig{Secret: "finance-test-secret", ExpireHour: 1}}
			repo := &stubJWTUserRepo{users: map[int64]*service.User{7: user}}
			auth := service.NewAuthService(nil, repo, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
			users := service.NewUserService(repo, nil, nil, nil)
			// The stored role must win over an older token's admin claim.
			token, err := auth.GenerateToken(context.Background(), &service.User{ID: 7, Role: service.RoleAdmin})
			require.NoError(t, err)
			for _, tc := range []struct {
				method, path   string
				admin, finance bool
			}{
				{http.MethodGet, "/api/v1/admin/accounting/summary", true, true},
				{http.MethodPost, "/api/v1/admin/accounting/transfers", true, true},
				{http.MethodPut, "/api/v1/admin/accounting/attachments/:id/content", true, true},
				{http.MethodGet, "/api/v1/admin/users", true, false},
				{http.MethodPut, "/api/v1/admin/settings", true, false},
				{http.MethodGet, "/api/v1/admin/accounting-other/summary", true, false},
				{http.MethodGet, "/api/v1/auth/me", false, true},
				{http.MethodPost, "/api/v1/auth/revoke-all-sessions", false, true},
				{http.MethodPost, "/api/v1/auth/oauth/bind-token", false, false},
				{http.MethodGet, "/api/v1/keys", false, false},
				{http.MethodPost, "/api/v1/keys", false, false},
				{http.MethodGet, "/api/v1/organization/accounting/summary", false, false},
			} {
				t.Run(tc.method+tc.path, func(t *testing.T) {
					router := gin.New()
					if tc.admin {
						router.Use(gin.HandlerFunc(NewAdminAuthMiddleware(auth, users, nil, nil)))
					} else {
						router.Use(gin.HandlerFunc(NewJWTAuthMiddleware(auth, users, nil, nil)))
					}
					router.Handle(tc.method, tc.path, func(c *gin.Context) {
						got, _ := GetUserRoleFromContext(c)
						require.Equal(t, role, got)
						c.Status(http.StatusNoContent)
					})
					w := httptest.NewRecorder()
					req := httptest.NewRequest(tc.method, tc.path, nil)
					req.Header.Set("Authorization", "Bearer "+token)
					router.ServeHTTP(w, req)
					allowed := role == service.RoleAdmin || (role == service.RoleFinance && tc.finance) || (role == service.RoleUser && !tc.admin)
					if allowed {
						require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
					} else {
						require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
					}
				})
			}
		})
	}
}

func TestFinanceCannotReuseGatewayKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := &service.User{ID: 7, Role: service.RoleFinance, Status: service.StatusActive}
	key := &service.APIKey{ID: 1, UserID: 7, Key: "finance-key", Status: service.StatusActive, User: user}
	repo := &stubApiKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) { return key, nil }}
	for _, mode := range []string{config.RunModeSimple, config.RunModeStandard} {
		cfg := &config.Config{RunMode: mode}
		svc := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)
		for _, auth := range []gin.HandlerFunc{gin.HandlerFunc(NewAPIKeyAuthMiddleware(svc, nil, cfg)), APIKeyAuthGoogle(svc, cfg)} {
			router := gin.New()
			router.Use(auth)
			router.GET("/v1beta/models", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
			req.Header.Set("x-api-key", key.Key)
			req.Header.Set("x-goog-api-key", key.Key)
			router.ServeHTTP(w, req)
			require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
			require.Contains(t, w.Body.String(), "Finance access is limited to accounting")
		}
	}
}

func TestFinanceSessionAccessInBackendMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settings := newBackendModeSettingService(t, "true")
	for _, path := range []string{"/api/v1/auth/me", "/api/v1/keys"} {
		router := gin.New()
		router.Use(func(c *gin.Context) { c.Set(string(ContextKeyUserRole), service.RoleFinance) })
		router.Use(BackendModeUserGuard(settings))
		router.GET(path, func(c *gin.Context) { c.Status(http.StatusNoContent) })
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if path == "/api/v1/auth/me" {
			require.Equal(t, http.StatusNoContent, w.Code)
		} else {
			require.Equal(t, http.StatusForbidden, w.Code)
		}
	}
}
