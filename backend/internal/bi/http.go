package bi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	principalKey = "bi.principal"
	webUserKey   = "bi.web_user"
)

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)

type Handler struct {
	service  *Service
	auth     *service.AuthService
	redis    *redis.Client
	worker   *Worker
	settings *service.SettingService
}

func NewHandler(db *sql.DB, rdb *redis.Client, cfg *config.Config, auth *service.AuthService, users *service.UserService, settings *service.SettingService) *Handler {
	svc := NewService(db, cfg.BI, users, newWeChatClient(cfg.BI.AppID, cfg.BI.AppSecret))
	return &Handler{service: svc, auth: auth, redis: rdb, worker: &Worker{service: svc}, settings: settings}
}

func (h *Handler) Start() { h.worker.Start() }
func (h *Handler) Stop()  { h.worker.Stop() }

func Headers(c *gin.Context) {
	requestID, _ := c.Request.Context().Value(ctxkey.RequestID).(string)
	if !requestIDPattern.MatchString(requestID) {
		requestID = uuid.NewString()
	}
	c.Set("bi.request_id", requestID)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.RequestID, requestID))
	c.Header("X-Request-ID", requestID)
	c.Header("Cache-Control", "private, no-store")
}

func Boundary(c *gin.Context) {
	Headers(c)
	for _, parameter := range c.Params {
		if !requestIDPattern.MatchString(parameter.Value) {
			WriteError(c, invalid(parameter.Key, "Invalid resource identifier"))
			return
		}
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	defer func() {
		if recover() != nil {
			WriteError(c, apiError(500, "INTERNAL_ERROR", "An internal error occurred"))
		}
	}()
	c.Next()
}

func WriteError(c *gin.Context, err error) {
	var typed *Error
	if !errors.As(err, &typed) {
		slog.ErrorContext(c.Request.Context(), "BI request failed", "request_id", c.GetString("bi.request_id"), "operation", c.FullPath(), "error_type", fmt.Sprintf("%T", err))
		typed = apiError(500, "INTERNAL_ERROR", "An internal error occurred")
		if errors.Is(err, context.DeadlineExceeded) {
			typed = apiError(503, "DATA_UNAVAILABLE", "Request timed out")
		}
	}
	response := *typed
	response.RequestID = c.GetString("bi.request_id")
	c.AbortWithStatusJSON(response.Status, response)
}

// WriteMiddlewareError adapts the existing web authentication error contract.
func WriteMiddlewareError(c *gin.Context, status int, _, _ string) {
	switch status {
	case http.StatusUnauthorized:
		WriteError(c, ErrUnauthenticated)
	case http.StatusForbidden:
		WriteError(c, ErrForbidden)
	case http.StatusTooManyRequests:
		WriteError(c, apiError(429, "RATE_LIMITED", "Too many requests"))
	default:
		WriteError(c, apiError(500, "INTERNAL_ERROR", "Authentication is unavailable"))
	}
}

func decodeRequest(c *gin.Context, target any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8<<10)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(target)
	if err == nil {
		var extra any
		if trailingErr := decoder.Decode(&extra); trailingErr != io.EOF {
			err = trailingErr
			if err == nil {
				err = errors.New("expected one JSON object")
			}
		}
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			WriteError(c, apiError(413, "PAYLOAD_TOO_LARGE", "Request body exceeds 8 KiB"))
		} else {
			WriteError(c, invalid("body", "Expected a JSON object with the documented fields"))
		}
		return false
	}
	return true
}

var rateLimitScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then redis.call('EXPIRE', KEYS[1], 60) end
return {count, redis.call('TTL', KEYS[1])}
`)

func (h *Handler) RateLimit(clientIP func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Keys contain a digest, never a credential, ticket, or raw client IP.
		key := "bi:auth-limit:" + tokenHash(clientIP(c))
		result, err := rateLimitScript.Run(c.Request.Context(), h.redis, []string{key}).Int64Slice()
		if err != nil || len(result) != 2 {
			WriteError(c, apiError(503, "DATA_UNAVAILABLE", "Authentication is temporarily unavailable"))
			return
		}
		if result[0] > 30 {
			c.Header("Retry-After", strconv.FormatInt(max(result[1], 1), 10))
			limited := apiError(429, "RATE_LIMITED", "Too many authentication requests")
			limited.Retryable = true
			WriteError(c, limited)
			return
		}
		c.Next()
	}
}

// RequireWebAudience is run after the original web JWT/session middleware.
func (h *Handler) RequireWebAudience(c *gin.Context) {
	claims, err := h.auth.ValidateToken(bearerToken(c.GetHeader("Authorization")))
	if err != nil || claims.UserID <= 0 || len(claims.Audience) != 1 || claims.Audience[0] != WebAudience || claims.Issuer != TokenIssuer {
		WriteError(c, ErrUnauthenticated)
		return
	}
	c.Set(webUserKey, claims.UserID)
	c.Next()
}

func (h *Handler) MobileAuth(c *gin.Context) {
	p, err := h.service.Authorize(c.Request.Context(), bearerToken(c.GetHeader("Authorization")))
	if err != nil {
		WriteError(c, err)
		return
	}
	c.Set(principalKey, p)
	c.Next()
}

func principal(c *gin.Context) Principal {
	value, _ := c.Get(principalKey)
	p, _ := value.(Principal)
	return p
}

func (h *Handler) Bootstrap(c *gin.Context) {
	if h.settings == nil {
		WriteError(c, apiError(503, "DATA_UNAVAILABLE", "Privacy notice is unavailable"))
		return
	}
	notice, err := h.settings.GetBIPrivacyNotice(c.Request.Context())
	if err != nil || !notice.Published() || notice.URL == "" {
		WriteError(c, apiError(503, "DATA_UNAVAILABLE", "Publish the BI privacy notice and configure its HTTPS site URL"))
		return
	}
	c.JSON(200, gin.H{"api_version": "v1", "min_client_version": h.service.config.MinClientVersion,
		"timezone": "Asia/Shanghai", "demo_available": false, "binding_enabled": true,
		"privacy_notice_version": notice.Version, "privacy_notice_url": notice.URL})
}

func (h *Handler) Login(c *gin.Context) {
	var input struct {
		Code          string  `json:"code"`
		ClientVersion *string `json:"client_version"`
	}
	if !decodeRequest(c, &input) {
		return
	}
	if input.ClientVersion == nil || utf8.RuneCountInString(*input.ClientVersion) > 32 {
		WriteError(c, invalid("client_version", "Client version is required and must not exceed 32 characters"))
		return
	}
	result, err := h.service.Login(c.Request.Context(), input.Code)
	respond(c, 200, result, err)
}

func (h *Handler) ApproveBinding(c *gin.Context) {
	var input struct {
		UserCode string `json:"user_code"`
		Confirm  bool   `json:"confirm_binding"`
	}
	if !decodeRequest(c, &input) {
		return
	}
	if !input.Confirm {
		WriteError(c, invalid("confirm_binding", "Explicit binding confirmation is required"))
		return
	}
	err := h.service.ApproveBinding(c.Request.Context(), c.GetInt64(webUserKey), input.UserCode, c.GetString("bi.request_id"))
	respond(c, 200, gin.H{"ok": true}, err)
}

func (h *Handler) ExchangeBinding(c *gin.Context) {
	var input struct {
		Ticket string `json:"binding_ticket"`
		Code   string `json:"code"`
	}
	if !decodeRequest(c, &input) {
		return
	}
	result, err := h.service.ExchangeBinding(c.Request.Context(), input.Ticket, input.Code)
	if errors.Is(err, ErrPendingBinding) {
		c.JSON(202, gin.H{"status": "pending", "retry_after_seconds": 3})
		return
	}
	respond(c, 200, result, err)
}

func readRefresh(c *gin.Context) (string, bool) {
	var input struct {
		Refresh string `json:"refresh_token"`
	}
	if !decodeRequest(c, &input) {
		return "", false
	}
	if strings.TrimSpace(input.Refresh) == "" || len(input.Refresh) > 2048 {
		WriteError(c, invalid("refresh_token", "Refresh token is required and must not exceed 2048 characters"))
		return "", false
	}
	return input.Refresh, true
}

func (h *Handler) Refresh(c *gin.Context) {
	refresh, ok := readRefresh(c)
	if !ok {
		return
	}
	result, err := h.service.Refresh(c.Request.Context(), refresh, c.GetString("bi.request_id"))
	respond(c, 200, result, err)
}

func (h *Handler) Logout(c *gin.Context) {
	refresh, ok := readRefresh(c)
	if !ok {
		return
	}
	err := h.service.Logout(c.Request.Context(), refresh, c.GetString("bi.request_id"))
	respond(c, 200, gin.H{"ok": true}, err)
}

func (h *Handler) Me(c *gin.Context) {
	result, err := h.service.Me(c.Request.Context(), principal(c))
	respond(c, 200, result, err)
}

func pageLimit(c *gin.Context) (int, bool) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit < 1 || limit > 100 || len(c.Query("cursor")) > 2048 {
		WriteError(c, invalid("pagination", "Limit must be 1–100 and cursor must not exceed 2048 characters"))
		return 0, false
	}
	return limit, true
}

func (h *Handler) Organizations(c *gin.Context) {
	limit, ok := pageLimit(c)
	if !ok {
		return
	}
	result, err := h.service.ListOrganizations(c.Request.Context(), principal(c), limit, c.Query("cursor"))
	respond(c, 200, result, err)
}

func (h *Handler) ListBindings(c *gin.Context) {
	limit, ok := pageLimit(c)
	if !ok {
		return
	}
	result, err := h.service.ListBindings(c.Request.Context(), c.GetInt64(webUserKey), limit, c.Query("cursor"))
	respond(c, 200, result, err)
}

func (h *Handler) RemoveBinding(c *gin.Context) {
	p := principal(c)
	err := h.service.RevokeBinding(c.Request.Context(), p.UserID, p.BindingID, c.GetString("bi.request_id"))
	respond(c, 204, nil, err)
}

func (h *Handler) RevokeBinding(c *gin.Context) {
	err := h.service.RevokeBinding(c.Request.Context(), c.GetInt64(webUserKey), c.Param("binding_id"), c.GetString("bi.request_id"))
	respond(c, 204, nil, err)
}

func respond(c *gin.Context, status int, value any, err error) {
	if err != nil {
		WriteError(c, err)
		return
	}
	if status == http.StatusNoContent {
		c.Status(status)
		return
	}
	c.JSON(status, value)
}
