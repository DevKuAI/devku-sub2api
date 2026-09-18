package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const conversationTestInstallation = "11111111-1111-4111-8111-111111111111"
const conversationTestToken = "dks_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

type conversationAuthRepo struct {
	service.DesktopRepository
	authorized *service.DesktopAuthorizedMember
}

func (r *conversationAuthRepo) GetAuthorizedMember(context.Context, string) (*service.DesktopAuthorizedMember, error) {
	return r.authorized, nil
}
func (r *conversationAuthRepo) FindActiveOrganizationByCode(context.Context, string) (*service.DesktopOrganization, error) {
	return r.authorized.Organization, nil
}
func (r *conversationAuthRepo) FindMemberByPhone(context.Context, int64, string) (*service.DesktopMember, error) {
	return r.authorized.Member, nil
}

type conversationSessions struct {
	session *service.DesktopSession
	touches int
	deleted bool
}

func (s *conversationSessions) Create(_ context.Context, _ string, session *service.DesktopSession) error {
	s.session = session
	session.IdleExpiresAt = time.Now().UTC().Add(service.DesktopSessionIdleTimeout)
	return nil
}
func (s *conversationSessions) Get(context.Context, string) (*service.DesktopSession, error) {
	if s.deleted {
		return nil, service.ErrDesktopUnauthenticated
	}
	return s.session, nil
}
func (s *conversationSessions) Touch(context.Context, string) error  { s.touches++; return nil }
func (s *conversationSessions) Delete(context.Context, string) error { s.deleted = true; return nil }

type conversationWriteRepo struct {
	service.DesktopConversationRepository
	writes int
	err    error
}

func (r *conversationWriteRepo) Create(_ context.Context, _, _ int64, input *service.DesktopConversationInput) (*service.DesktopConversationReceipt, error) {
	r.writes++
	return &service.DesktopConversationReceipt{RecordID: input.RecordID, ReceivedAt: time.Now().UTC()}, r.err
}

type conversationRateLimiter struct{ delay time.Duration }

func (r *conversationRateLimiter) Allow(context.Context, string) (time.Duration, error) {
	return r.delay, nil
}

type conversationLoginLimiter struct{}

func (conversationLoginLimiter) AllowLookup(context.Context, string, string) (time.Duration, error) {
	return 0, nil
}
func (conversationLoginLimiter) AllowLogin(context.Context, string, string, string, string) (time.Duration, error) {
	return 0, nil
}
func (conversationLoginLimiter) RecordLoginFailure(context.Context, string, string) (time.Duration, error) {
	return 0, nil
}
func (conversationLoginLimiter) ClearLoginFailures(context.Context, string, string) error { return nil }

type conversationRefreshStore struct{ service.DesktopRefreshStore }

func (conversationRefreshStore) Create(context.Context, string, service.DesktopRefreshSession) error {
	return nil
}

func conversationRouter(t *testing.T) (*gin.Engine, *conversationSessions, *conversationWriteRepo, *conversationRateLimiter) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{Desktop: config.DesktopConfig{JWTSecret: base64.StdEncoding.EncodeToString(make([]byte, 32)), AccessTokenTTLMinutes: 15, RefreshFamilyTTLDays: 30}}
	tokens, err := service.NewDesktopTokenManager(cfg)
	require.NoError(t, err)
	repo := &conversationAuthRepo{authorized: &service.DesktopAuthorizedMember{
		Member:       &service.DesktopMember{ID: 1, PublicID: "mem_one", Name: "Member", NameNormalized: "Member", Status: "active", AuthVersion: 1},
		Organization: &service.DesktopOrganization{ID: 2, PublicID: "org_one", Status: "active", AuthVersion: 1}, GatewayUser: &service.User{Status: "active"},
	}}
	keyID := int64(7)
	repo.authorized.Member.CurrentAPIKeyID = &keyID
	repo.authorized.Member.CurrentAPIKey = "sk-model-canary"
	repo.authorized.Member.CurrentAPIKeyStatus = service.StatusAPIKeyActive
	repo.authorized.Organization.TargetConfig = &service.DesktopTargetConfig{SchemaVersion: 1, Targets: service.DesktopTargets{ChatGPTCodex: &service.DesktopTarget{Enabled: true, ProviderID: "provider", DisplayName: "Codex", RequestedModel: "model", WireAPI: service.DesktopWireAPIResponses}}}
	sessions := &conversationSessions{session: &service.DesktopSession{MemberPublicID: "mem_one", OrganizationPublicID: "org_one", InstallationID: conversationTestInstallation, MemberVersion: 1, OrganizationVersion: 1}}
	records := &conversationWriteRepo{}
	limiter := &conversationRateLimiter{}
	svc := service.NewDesktopService(repo, nil, conversationRefreshStore{}, conversationLoginLimiter{}, tokens, nil, cfg, sessions, records, limiter)
	h := NewDesktopHandler(svc)
	router := gin.New()
	router.Use(middleware.RequestLogger())
	router.POST("/conversation-records", middleware.StrictBodyLimit(service.DesktopConversationMaxBodyBytes), middleware.DesktopSessionAuth(svc), h.CreateConversation)
	router.POST("/login", middleware.StrictBodyLimit(8*1024), h.Login)
	router.POST("/logout", middleware.DesktopAuth(svc), h.Logout)
	router.GET("/me", middleware.DesktopAuth(svc), h.Me)
	router.GET("/usage", middleware.DesktopAuth(svc), h.UsageSummary)
	router.GET("/configuration", middleware.DesktopAuth(svc), h.ModelConfiguration)
	return router, sessions, records, limiter
}

func conversationBody() string {
	return `{"schemaVersion":2,"recordId":"22222222-2222-4222-8222-222222222222","client":"workbuddy","installationId":"` + conversationTestInstallation + `","organizationId":"org_one","memberId":"mem_one","sessionId":"source","startedAt":"2026-09-18T00:00:00Z","stoppedAt":"2026-09-18T00:01:00Z","prompts":[{"text":"body-canary","truncated":false}],"response":null,"captureStatus":"response_missing"}`
}

func TestDesktopConversationHTTPContract(t *testing.T) {
	for _, name := range []string{"ok", "invalid", "identity", "installation", "v1", "missing", "media", "encoding", "rate", "storage", "chunked", "too-large", "exact-limit"} {
		t.Run(name, func(t *testing.T) {
			router, sessions, records, limiter := conversationRouter(t)
			body, status, code := conversationBody(), http.StatusOK, ""
			switch name {
			case "invalid":
				body = strings.Replace(body, `"truncated":false`, `"truncated":null`, 1)
				status, code = 422, "VALIDATION_FAILED"
			case "identity":
				body = strings.Replace(body, "mem_one", "mem_other", 1)
				status, code = 403, "CONVERSATION_IDENTITY_MISMATCH"
			case "installation":
				status, code = 403, "SESSION_INSTALLATION_MISMATCH"
			case "v1":
				status, code = 401, "SESSION_AUTH_REQUIRED"
			case "missing":
				status, code = 401, "UNAUTHENTICATED"
			case "media", "encoding":
				status, code = 415, "UNSUPPORTED_MEDIA_TYPE"
			case "rate":
				limiter.delay = time.Minute
				status, code = 429, "RATE_LIMITED"
			case "storage":
				records.err = service.ErrDesktopConversationStorage
				status, code = 503, "CONVERSATION_STORAGE_UNAVAILABLE"
			case "too-large":
				body += strings.Repeat(" ", int(service.DesktopConversationMaxBodyBytes)-len(body)+1)
				status, code = 413, "PAYLOAD_TOO_LARGE"
			case "exact-limit":
				body += strings.Repeat(" ", int(service.DesktopConversationMaxBodyBytes)-len(body))
			}
			request := httptest.NewRequest(http.MethodPost, "/conversation-records", strings.NewReader(body))
			request.Header.Set("Authorization", "Bearer "+conversationTestToken)
			request.Header.Set("X-Installation-ID", conversationTestInstallation)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Cache-Control", "no-store")
			if name == "installation" {
				request.Header.Set("X-Installation-ID", "33333333-3333-4333-8333-333333333333")
			}
			if name == "v1" {
				request.Header.Set("Authorization", "Bearer header.payload.signature")
			}
			if name == "missing" {
				request.Header.Del("Authorization")
			}
			if name == "media" {
				request.Header.Set("Content-Type", "text/plain")
			}
			if name == "encoding" {
				request.Header.Set("Content-Encoding", "gzip")
			}
			if name == "chunked" || name == "too-large" {
				request.ContentLength = -1
				request.TransferEncoding = []string{"chunked"}
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, status, response.Code, response.Body.String())
			require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
			require.NotEmpty(t, response.Header().Get("X-Request-ID"))
			require.NotContains(t, response.Body.String(), "body-canary")
			if code != "" {
				require.Contains(t, response.Body.String(), code)
			}
			if status == 200 || name == "storage" {
				require.Equal(t, 1, sessions.touches)
				require.Equal(t, 1, records.writes)
			} else {
				require.Zero(t, sessions.touches)
				require.Zero(t, records.writes)
			}
		})
	}
}

func TestDesktopLoginVersionNegotiationAndLogout(t *testing.T) {
	for _, version := range []string{"", "1", "2", "3"} {
		t.Run("version-"+version, func(t *testing.T) {
			router, sessions, _, _ := conversationRouter(t)
			request := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(`{"organization_code":"desktop","name":"Member","phone":"13800138000"}`))
			request.Header.Set("X-Desktop-Auth-Version", version)
			request.Header.Set("X-Installation-ID", conversationTestInstallation)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if version == "3" {
				require.Equal(t, 422, response.Code)
				require.Contains(t, response.Body.String(), "AUTH_VERSION_UNSUPPORTED")
				return
			}
			require.Equal(t, 200, response.Code, response.Body.String())
			var envelope struct {
				Data map[string]any `json:"data"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
			if version != "2" {
				require.Contains(t, envelope.Data, "refresh_token")
				require.EqualValues(t, 900, envelope.Data["expires_in"])
				return
			}
			require.EqualValues(t, 2, envelope.Data["auth_version"])
			require.EqualValues(t, 1296000, envelope.Data["idle_timeout_seconds"])
			require.NotContains(t, envelope.Data, "refresh_token")
			require.NotContains(t, envelope.Data, "expires_in")
			token, ok := envelope.Data["access_token"].(string)
			require.True(t, ok)
			require.Regexp(t, `^dks_[A-Za-z0-9_-]{43}$`, token)
			logout := httptest.NewRequest(http.MethodPost, "/logout", nil)
			logout.Header.Set("Authorization", "Bearer "+token)
			logout.Header.Set("X-Installation-ID", conversationTestInstallation)
			out := httptest.NewRecorder()
			router.ServeHTTP(out, logout)
			require.Equal(t, 200, out.Code)
			require.True(t, sessions.deleted)
			require.Zero(t, sessions.touches)
		})
	}
}

func TestDesktopInvalidBusinessParametersDoNotTouch(t *testing.T) {
	router, sessions, _, _ := conversationRouter(t)
	request := httptest.NewRequest(http.MethodGet, "/usage?timezone=invalid", nil)
	request.Header.Set("Authorization", "Bearer "+conversationTestToken)
	request.Header.Set("X-Installation-ID", conversationTestInstallation)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, 422, response.Code)
	require.Zero(t, sessions.touches)
}

func TestDesktopV2ConfigurationNotModifiedTouchesSession(t *testing.T) {
	router, sessions, _, _ := conversationRouter(t)
	fetch := func(path, etag string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer "+conversationTestToken)
		request.Header.Set("X-Installation-ID", conversationTestInstallation)
		request.Header.Set("If-None-Match", etag)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	first := fetch("/configuration", "")
	require.Equal(t, 200, first.Code, first.Body.String())
	second := fetch("/configuration", first.Header().Get("ETag"))
	require.Equal(t, 304, second.Code)
	require.Equal(t, "no-store", second.Header().Get("Cache-Control"))
	require.Equal(t, 2, sessions.touches)
	invalid := fetch("/configuration?targets=unknown", "")
	require.Equal(t, 422, invalid.Code)
	require.Equal(t, 2, sessions.touches)
}
