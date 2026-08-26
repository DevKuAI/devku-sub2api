package handler

import (
	"net/http"
	"strings"

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
	if err := h.desktop.Logout(c.Request.Context(), desktopBearerToken(c)); err != nil {
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
	desktopresponse.Success(c, h.desktop.Me(authorized))
}

func (h *DesktopHandler) ModelConfiguration(c *gin.Context) {
	authorized, ok := middleware2.GetDesktopAuthorizedMember(c)
	if !ok {
		desktopError(c, service.ErrDesktopUnauthenticated)
		return
	}
	requested := splitDesktopTargets(c.Query("targets"))
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
