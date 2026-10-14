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
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const principalKey = "bi.principal"

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)

type Handler struct {
	service        *Service
	redis          *redis.Client
	settings       *service.SettingService
	workerMu       sync.Mutex
	desktopEnabled bool
	cancel         context.CancelFunc
	done           chan struct{}
}

func NewHandler(db *sql.DB, rdb *redis.Client, cfg *config.Config, desktop *service.DesktopService, settings *service.SettingService) *Handler {
	return &Handler{service: NewService(db, cfg.BI, desktop, newWeChatClient(cfg.BI.AppID, cfg.BI.AppSecret)), redis: rdb, settings: settings, desktopEnabled: cfg.Desktop.Enabled}
}
func Headers(c *gin.Context) {
	id, _ := c.Request.Context().Value(ctxkey.RequestID).(string)
	if !requestIDPattern.MatchString(id) {
		id = uuid.NewString()
	}
	c.Set("bi.request_id", id)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.RequestID, id))
	c.Header("X-Request-ID", id)
	c.Header("Cache-Control", "private, no-store")
}
func Boundary(c *gin.Context) {
	Headers(c)
	for _, p := range c.Params {
		if !requestIDPattern.MatchString(p.Value) {
			WriteError(c, invalid(p.Key, "Invalid resource identifier"))
			return
		}
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	defer func() {
		if recover() != nil {
			WriteError(c, apiError(500, "INTERNAL_ERROR", "Request failed"))
		}
	}()
	c.Next()
}
func WriteError(c *gin.Context, err error) {
	var typed *Error
	if !errors.As(err, &typed) {
		var application *infraerrors.Error
		if errors.As(err, &application) && application.Code >= 400 && application.Code < 500 {
			if application.Code == 404 {
				typed = ErrNotFound
			} else {
				typed = apiError(int(application.Code), application.Reason, "Request cannot be completed")
			}
		} else {
			slog.ErrorContext(c.Request.Context(), "BI request failed", "request_id", c.GetString("bi.request_id"), "operation", c.FullPath(), "error_type", fmt.Sprintf("%T", err))
			typed = apiError(503, "DATA_UNAVAILABLE", "Data is temporarily unavailable")
		}
	}
	response := *typed
	response.RequestID = c.GetString("bi.request_id")
	c.AbortWithStatusJSON(response.Status, response)
}
func decodeRequest(c *gin.Context, target any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8<<10)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var oversized *http.MaxBytesError
		if errors.As(err, &oversized) {
			WriteError(c, apiError(413, "PAYLOAD_TOO_LARGE", "Request body exceeds limit"))
		} else {
			WriteError(c, invalid("body", "Expected documented JSON fields"))
		}
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		WriteError(c, invalid("body", "Expected one JSON object"))
		return false
	}
	return true
}
func respond(c *gin.Context, status int, value any, err error) {
	if err != nil {
		WriteError(c, err)
		return
	}
	c.JSON(status, value)
}
func adminRespond(c *gin.Context, value any, err error) {
	if err != nil {
		WriteError(c, err)
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "success", "data": value})
}
func principal(c *gin.Context) Principal {
	p, _ := c.Get(principalKey)
	value, _ := p.(Principal)
	return value
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

func (h *Handler) Bootstrap(c *gin.Context) {
	if h.settings == nil {
		WriteError(c, apiError(503, "DATA_UNAVAILABLE", "Privacy notice unavailable"))
		return
	}
	notice, err := h.settings.GetBIPrivacyNotice(c.Request.Context())
	if err != nil || !notice.Published() || notice.URL == "" {
		WriteError(c, apiError(503, "PRIVACY_NOTICE_UNAVAILABLE", "Publish the BI privacy notice"))
		return
	}
	c.JSON(200, gin.H{"api_version": "v1", "min_client_version": h.service.config.MinClientVersion, "identity_mode": "independent_wechat", "timezone": "Asia/Shanghai", "demo_available": false, "privacy_notice_version": notice.Version, "privacy_notice_url": notice.URL})
}
func (h *Handler) Login(c *gin.Context) {
	var in struct {
		Code          string `json:"code"`
		ClientVersion string `json:"client_version"`
		DisplayName   string `json:"display_name"`
	}
	if !decodeRequest(c, &in) {
		return
	}
	if in.ClientVersion == "" {
		WriteError(c, invalid("client_version", "Client version required"))
		return
	}
	v, e := h.service.Login(c.Request.Context(), in.Code, LoginOptions{ClientVersion: in.ClientVersion, DisplayName: in.DisplayName})
	respond(c, 200, v, e)
}
func (h *Handler) Refresh(c *gin.Context) {
	var in struct {
		Refresh string `json:"refresh_token"`
	}
	if !decodeRequest(c, &in) {
		return
	}
	v, e := h.service.Refresh(c.Request.Context(), in.Refresh, c.GetString("bi.request_id"))
	respond(c, 200, v, e)
}
func (h *Handler) Logout(c *gin.Context) {
	var in struct {
		Refresh string `json:"refresh_token"`
	}
	if !decodeRequest(c, &in) {
		return
	}
	e := h.service.Logout(c.Request.Context(), in.Refresh, c.GetString("bi.request_id"))
	respond(c, 200, gin.H{"ok": true}, e)
}
func (h *Handler) Me(c *gin.Context) {
	v, e := h.service.Me(c.Request.Context(), principal(c))
	respond(c, 200, v, e)
}
func (h *Handler) Organizations(c *gin.Context) {
	page, size, ok := queryPage(c)
	if !ok {
		return
	}
	v, e := h.service.Organizations(c.Request.Context(), principal(c), page, size)
	respond(c, 200, v, e)
}
func (h *Handler) PreviewInvitation(c *gin.Context) {
	var in struct {
		Token string `json:"token"`
	}
	if !decodeRequest(c, &in) {
		return
	}
	v, e := h.service.PreviewInvitation(c.Request.Context(), principal(c), in.Token)
	respond(c, 200, v, e)
}
func (h *Handler) AcceptInvitation(c *gin.Context) {
	var in struct {
		Token   string `json:"token"`
		Confirm bool   `json:"confirm_join"`
	}
	if !decodeRequest(c, &in) {
		return
	}
	if !in.Confirm {
		WriteError(c, invalid("confirm_join", "Explicit confirmation required"))
		return
	}
	v, e := h.service.AcceptInvitation(c.Request.Context(), principal(c), in.Token, c.GetString("bi.request_id"))
	respond(c, 200, v, e)
}
func queryPage(c *gin.Context) (int, int, bool) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, e := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil || e != nil || page < 1 || page > 100000 || size < 1 || size > 100 {
		WriteError(c, invalid("page", "Invalid pagination"))
		return 0, 0, false
	}
	return page, size, true
}

// ManagerScope is called only inside the already authenticated web route group.
func (h *Handler) ManagerScope(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		WriteError(c, ErrUnauthenticated)
		return
	}
	c.Set("bi.actor", subject.UserID)
	if c.GetBool("bi.admin") {
		c.Next()
		return
	}
	if h.service.desktop == nil {
		WriteError(c, ErrForbidden)
		return
	}
	org, err := h.service.desktop.GetManagedOrganization(c.Request.Context(), subject.UserID)
	if err != nil {
		WriteError(c, ErrForbidden)
		return
	}
	c.Set("bi.managed_org", org.PublicID)
	c.Next()
}
func managedOrganization(c *gin.Context) string {
	if c.GetBool("bi.admin") {
		return c.Param("organization_id")
	}
	return c.GetString("bi.managed_org")
}
func actor(c *gin.Context) int64 { return c.GetInt64("bi.actor") }
