//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type impersonationUserRepo struct {
	service.UserRepository
	users map[int64]*service.User
}

func (r *impersonationUserRepo) GetByID(_ context.Context, id int64) (*service.User, error) {
	user := r.users[id]
	if user == nil {
		return nil, service.ErrUserNotFound
	}
	copy := *user
	return &copy, nil
}

func (*impersonationUserRepo) GetUserAvatar(context.Context, int64) (*service.UserAvatar, error) {
	return nil, nil
}

type impersonationTokenCache struct {
	userHandlerRefreshTokenCacheStub
	tokens   map[string]*service.RefreshTokenData
	storeErr error
}

func (r *impersonationTokenCache) StoreRefreshToken(_ context.Context, hash string, data *service.RefreshTokenData, _ time.Duration) error {
	if r.storeErr != nil {
		return r.storeErr
	}
	r.tokens[hash] = data
	return nil
}

func (r *impersonationTokenCache) GetRefreshToken(_ context.Context, hash string) (*service.RefreshTokenData, error) {
	if data := r.tokens[hash]; data != nil {
		return data, nil
	}
	return nil, service.ErrRefreshTokenNotFound
}

func (r *impersonationTokenCache) DeleteRefreshToken(_ context.Context, hash string) error {
	delete(r.tokens, hash)
	return nil
}

type impersonationAuditRepo struct {
	service.AuditLogRepository
	logs []*service.AuditLog
}

func (r *impersonationAuditRepo) BatchInsert(_ context.Context, logs []*service.AuditLog) (int64, error) {
	r.logs = append(r.logs, logs...)
	return int64(len(logs)), nil
}

func (*impersonationAuditRepo) DeleteBefore(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

func newImpersonationTestHandler() (*AuthHandler, *impersonationUserRepo, *impersonationTokenCache) {
	repo := &impersonationUserRepo{users: map[int64]*service.User{
		1: {ID: 1, Email: "admin@example.com", Role: service.RoleAdmin, Status: service.StatusActive},
		2: {ID: 2, Email: "user@example.com", Role: service.RoleUser, Status: service.StatusActive},
		3: {ID: 3, Email: "other-admin@example.com", Role: service.RoleAdmin, Status: service.StatusActive},
	}}
	cache := &impersonationTokenCache{tokens: make(map[string]*service.RefreshTokenData)}
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "impersonation-test-secret", ExpireHour: 1, RefreshTokenExpireDays: 7}}
	auth := service.NewAuthService(nil, repo, nil, cache, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	return &AuthHandler{authService: auth, userService: service.NewUserService(repo, nil, nil, nil)}, repo, cache
}

func impersonationTestRouter(h *AuthHandler, audit *service.AuditLogService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.HandlerFunc(middleware.NewAdminAuthMiddleware(h.authService, h.userService, nil, nil)))
	if audit != nil {
		router.Use(gin.HandlerFunc(middleware.NewAuditLogMiddleware(audit)))
	}
	router.POST("/api/v1/admin/users/:id/impersonate", h.ImpersonateUser)
	router.GET("/api/v1/admin/ping", func(c *gin.Context) { c.Status(http.StatusOK) })
	return router
}

func TestImpersonateUserSessionAndAudit(t *testing.T) {
	h, repo, cache := newImpersonationTestHandler()
	auditRepo := &impersonationAuditRepo{}
	audit := service.NewAuditLogService(auditRepo, nil)
	audit.Start()
	router := impersonationTestRouter(h, audit)
	adminToken, err := h.authService.GenerateToken(context.Background(), repo.users[1])
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/2/impersonate", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	result := httptest.NewRecorder()
	router.ServeHTTP(result, req)
	audit.Stop()
	require.Equal(t, http.StatusOK, result.Code, result.Body.String())
	require.Equal(t, "no-store", result.Header().Get("Cache-Control"))

	var body struct {
		Data AuthResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(result.Body.Bytes(), &body))
	require.Equal(t, int64(2), body.Data.User.ID)
	claims, err := h.authService.ValidateToken(body.Data.AccessToken)
	require.NoError(t, err)
	require.Equal(t, int64(2), claims.UserID)
	require.Equal(t, service.RoleUser, claims.Role)
	require.NotEmpty(t, claims.SessionID)
	require.Len(t, cache.tokens, 1)
	for _, token := range cache.tokens {
		require.Equal(t, claims.SessionID, token.FamilyID)
		require.Equal(t, int64(2), token.UserID)
	}

	refreshed, err := h.authService.RefreshTokenPair(context.Background(), body.Data.RefreshToken)
	require.NoError(t, err)
	refreshedClaims, err := h.authService.ValidateToken(refreshed.AccessToken)
	require.NoError(t, err)
	require.Equal(t, int64(2), refreshedClaims.UserID)
	require.Equal(t, service.RoleUser, refreshedClaims.Role)

	for _, tc := range []struct {
		token string
		want  int
	}{{body.Data.AccessToken, http.StatusForbidden}, {adminToken, http.StatusOK}} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ping", nil)
		req.Header.Set("Authorization", "Bearer "+tc.token)
		result := httptest.NewRecorder()
		router.ServeHTTP(result, req)
		require.Equal(t, tc.want, result.Code)
	}

	require.Len(t, auditRepo.logs, 1)
	entry := auditRepo.logs[0]
	require.Equal(t, int64(1), *entry.ActorUserID)
	require.Equal(t, "admin@example.com", entry.ActorEmail)
	require.Contains(t, entry.Action, "impersonate")
	require.Equal(t, "2", entry.Extra["params"].(map[string]string)["id"])
	require.NotContains(t, entry.RequestBody, body.Data.AccessToken)
}

func TestImpersonateUserRejectsInvalidRequests(t *testing.T) {
	for _, tc := range []struct {
		name     string
		actor    int64
		target   string
		disabled int64
		cacheErr bool
		want     int
	}{
		{"unauthenticated", 0, "2", 0, false, http.StatusUnauthorized},
		{"regular user", 2, "3", 0, false, http.StatusForbidden},
		{"self", 1, "1", 0, false, http.StatusForbidden},
		{"another admin", 1, "3", 0, false, http.StatusForbidden},
		{"disabled target", 1, "2", 2, false, http.StatusForbidden},
		{"disabled actor", 1, "2", 1, false, http.StatusUnauthorized},
		{"missing target", 1, "99", 0, false, http.StatusNotFound},
		{"invalid ID", 1, "oops", 0, false, http.StatusBadRequest},
		{"negative ID", 1, "-2", 0, false, http.StatusBadRequest},
		{"cache unavailable", 1, "2", 0, true, http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, repo, cache := newImpersonationTestHandler()
			if tc.disabled != 0 {
				repo.users[tc.disabled].Status = service.StatusDisabled
			}
			if tc.cacheErr {
				cache.storeErr = errors.New("cache unavailable")
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+tc.target+"/impersonate", nil)
			if tc.actor != 0 {
				token, err := h.authService.GenerateToken(context.Background(), repo.users[tc.actor])
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+token)
			}
			result := httptest.NewRecorder()
			impersonationTestRouter(h, nil).ServeHTTP(result, req)
			require.Equal(t, tc.want, result.Code, result.Body.String())
			require.NotContains(t, result.Body.String(), "access_token")
			require.Empty(t, cache.tokens)
		})
	}
}

func TestImpersonateUserRequiresBrowserSession(t *testing.T) {
	h, _, cache := newImpersonationTestHandler()
	result := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(result)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/2/impersonate", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})
	c.Set("auth_method", service.AuditAuthMethodAdminAPIKey)
	h.ImpersonateUser(c)
	require.Equal(t, http.StatusForbidden, result.Code)
	require.Empty(t, cache.tokens)
	_, _, err := h.authService.ImpersonateUser(context.Background(), 2, 3)
	require.ErrorIs(t, err, service.ErrImpersonationNotAllowed)
}
