package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/desktopresponse"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type DesktopHandler struct {
	desktop *service.DesktopService
}

func NewDesktopHandler(desktop *service.DesktopService) *DesktopHandler {
	return &DesktopHandler{desktop: desktop}
}

func (h *DesktopHandler) Service() *service.DesktopService {
	return h.desktop
}

type desktopLookupRequest struct {
	OrganizationCode string `json:"organization_code" binding:"required"`
}

type desktopLoginRequest struct {
	OrganizationCode string `json:"organization_code" binding:"required"`
	Name             string `json:"name" binding:"required"`
	Phone            string `json:"phone" binding:"required"`
}

type desktopRefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *DesktopHandler) OrganizationLookup(c *gin.Context) {
	installationID, ok := desktopInstallationID(c)
	if !ok {
		return
	}
	var req desktopLookupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		desktopBindingError(c, err)
		return
	}
	result, err := h.desktop.LookupOrganization(c.Request.Context(), req.OrganizationCode, middleware2.SecurityClientIP(c), installationID)
	if err != nil {
		desktopError(c, err)
		return
	}
	desktopresponse.Success(c, result)
}

func (h *DesktopHandler) Login(c *gin.Context) {
	version := c.GetHeader("X-Desktop-Auth-Version")
	if version != "" && version != "1" && version != "2" {
		desktopError(c, service.ErrDesktopAuthVersionUnsupported)
		return
	}
	if version == "2" {
		h.loginV2(c)
		return
	}

	installationID, ok := desktopInstallationID(c)
	if !ok {
		return
	}
	var req desktopLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		desktopBindingError(c, err)
		return
	}
	result, err := h.desktop.Login(c.Request.Context(), req.OrganizationCode, req.Name, req.Phone, middleware2.SecurityClientIP(c), installationID)
	if err != nil {
		desktopError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	desktopresponse.Success(c, result)
}

func (h *DesktopHandler) Refresh(c *gin.Context) {
	var req desktopRefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		desktopBindingError(c, err)
		return
	}
	result, err := h.desktop.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		desktopError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	desktopresponse.Success(c, result)
}

func (h *DesktopHandler) Logout(c *gin.Context) {
	auth, _ := middleware2.GetDesktopAuthorization(c)
	if err := h.desktop.LogoutAuthorized(c.Request.Context(), auth, desktopBearerToken(c)); err != nil {
		desktopError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	desktopresponse.Success(c, gin.H{"logged_out": true})
}

func (h *DesktopHandler) Me(c *gin.Context) {
	authorized, ok := middleware2.GetDesktopAuthorizedMember(c)
	if !ok {
		desktopError(c, service.ErrDesktopUnauthenticated)
		return
	}
	c.Header("Cache-Control", "no-store")
	if !h.touchDesktopSession(c) {
		return
	}
	desktopresponse.Success(c, h.desktop.Me(authorized))
}

func (h *DesktopHandler) ModelConfiguration(c *gin.Context) {
	authorized, ok := middleware2.GetDesktopAuthorizedMember(c)
	if !ok {
		desktopError(c, service.ErrDesktopUnauthenticated)
		return
	}
	requested := splitDesktopTargets(c.Query("targets"))
	if auth, ok := middleware2.GetDesktopAuthorization(c); ok && auth.Session != nil {
		for _, target := range requested {
			if target != "workbuddy" && target != "chatgpt_codex" {
				desktopError(c, service.ErrDesktopValidation)
				return
			}
		}
	}
	if !h.touchDesktopSession(c) {
		return
	}
	configuration, version, err := h.desktop.ModelConfiguration(authorized, requested)
	if err != nil {
		desktopError(c, err)
		return
	}
	etag := `"` + version + `"`
	c.Header("Cache-Control", "no-store")
	c.Header("ETag", etag)
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	desktopresponse.Success(c, configuration)
}

func (h *DesktopHandler) UsageSummary(c *gin.Context) {
	authorized, ok := middleware2.GetDesktopAuthorizedMember(c)
	if !ok {
		desktopError(c, service.ErrDesktopUnauthenticated)
		return
	}
	if _, err := time.LoadLocation(c.Query("timezone")); err != nil || c.Query("timezone") == "Local" {
		desktopError(c, service.ErrDesktopValidation.WithMetadata(map[string]string{"field": "timezone"}))
		return
	}
	if !h.touchDesktopSession(c) {
		return
	}
	result, err := h.desktop.UsageSummary(c.Request.Context(), authorized, c.Query("timezone"))
	if err != nil {
		desktopError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	desktopresponse.Success(c, result)
}

func desktopInstallationID(c *gin.Context) (string, bool) {
	value := strings.TrimSpace(c.GetHeader("X-Installation-ID"))
	parsed, err := uuid.Parse(value)
	if err != nil || len(value) != 36 || parsed.String() != strings.ToLower(value) {
		desktopError(c, service.ErrDesktopValidation.WithMetadata(map[string]string{"field": "X-Installation-ID"}))
		return "", false
	}
	return value, true
}

func desktopBindingError(c *gin.Context, err error) {
	if middleware2.IsBodyTooLarge(err) {
		desktopresponse.PayloadTooLarge(c)
		return
	}
	desktopError(c, service.ErrDesktopValidation)
}

func desktopError(c *gin.Context, err error) {
	desktopresponse.SetRetryAfter(c, err)
	desktopresponse.Error(c, err)
}

func desktopBearerToken(c *gin.Context) string {
	parts := strings.Fields(c.GetHeader("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func splitDesktopTargets(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func (h *DesktopHandler) loginV2(c *gin.Context) {
	installationID := c.GetHeader("X-Installation-ID")
	if err := service.ValidateDesktopInstallationID(installationID); err != nil {
		desktopError(c, err)
		return
	}
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		desktopBindingError(c, err)
		return
	}
	if _, err := service.DecodeDesktopJSONObject(raw, []string{"organization_code", "name", "phone"}, nil, nil); err != nil {
		desktopError(c, err)
		return
	}
	var req desktopLoginRequest
	if json.Unmarshal(raw, &req) != nil || req.OrganizationCode == "" || req.Name == "" || req.Phone == "" {
		desktopError(c, service.ErrDesktopValidation)
		return
	}
	result, err := h.desktop.LoginV2(c.Request.Context(), req.OrganizationCode, req.Name, req.Phone, middleware2.SecurityClientIP(c), installationID)
	if err != nil {
		desktopError(c, err)
		return
	}
	desktopresponse.Success(c, result)
}

func (h *DesktopHandler) touchDesktopSession(c *gin.Context) bool {
	auth, ok := middleware2.GetDesktopAuthorization(c)
	if !ok {
		desktopError(c, service.ErrDesktopUnauthenticated)
		return false
	}
	if err := h.desktop.TouchSession(c.Request.Context(), auth); err != nil {
		desktopError(c, err)
		return false
	}
	return true
}
