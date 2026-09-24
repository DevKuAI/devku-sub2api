package handler

import (
	"io"
	"mime"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/desktopresponse"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *DesktopHandler) CreateConversation(c *gin.Context) {
	auth, ok := middleware.GetDesktopAuthorization(c)
	if !ok || auth.Session == nil {
		desktopError(c, service.ErrDesktopSessionRequired)
		return
	}
	if !auth.Member.Organization.ConversationReportingEnabled {
		desktopError(c, service.ErrDesktopConversationReportingDisabled)
		return
	}

	start := time.Now()
	input, recordID, bodySize, ok := decodeDesktopConversationRequest(c)
	defer func() {
		logger.FromContext(c.Request.Context()).Info("desktop conversation upload",
			zap.String("record_id", recordID), zap.Int("status_code", c.Writer.Status()),
			zap.Int("bytes", bodySize), zap.Int64("latency_ms", time.Since(start).Milliseconds()))
	}()
	if !ok {
		return
	}
	result, err := h.desktop.CreateConversation(c.Request.Context(), auth, input)
	if err != nil {
		desktopError(c, err)
		return
	}
	desktopresponse.Success(c, result)
}

// CreateDirectConversation accepts the same validated payload for webhook
// callers authenticated by the admin route middleware.
func (h *DesktopHandler) CreateDirectConversation(c *gin.Context) {
	start := time.Now()
	input, recordID, bodySize, ok := decodeDesktopConversationRequest(c)
	defer func() {
		logger.FromContext(c.Request.Context()).Info("desktop conversation direct upload",
			zap.String("record_id", recordID), zap.Int("status_code", c.Writer.Status()),
			zap.Int("bytes", bodySize), zap.Int64("latency_ms", time.Since(start).Milliseconds()))
	}()
	if !ok {
		return
	}
	organizationID := strings.TrimSpace(c.GetHeader("organizationId"))
	memberID := strings.TrimSpace(c.GetHeader("memberId"))
	if organizationID == "" || memberID == "" {
		desktopError(c, service.ErrDesktopConversationIdentity)
		return
	}
	result, err := h.desktop.CreateDirectConversation(c.Request.Context(), organizationID, memberID, input)
	if err != nil {
		desktopError(c, err)
		return
	}
	middleware.SetAuditExtra(c, map[string]any{"record_id": result.RecordID, "organization_id": organizationID, "resource_id": memberID})
	desktopresponse.Success(c, result)
}

func decodeDesktopConversationRequest(c *gin.Context) (*service.DesktopConversationInput, string, int, bool) {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	encoding := strings.TrimSpace(c.GetHeader("Content-Encoding"))
	if err != nil || mediaType != "application/json" || (encoding != "" && !strings.EqualFold(encoding, "identity")) {
		desktopError(c, service.ErrDesktopMediaType)
		return nil, "", 0, false
	}
	if c.GetHeader("Cache-Control") != "no-store" {
		desktopError(c, service.ErrDesktopValidation.WithMetadata(map[string]string{"field": "Cache-Control"}))
		return nil, "", 0, false
	}
	raw, err := io.ReadAll(c.Request.Body)
	bodySize := len(raw)
	if err != nil {
		desktopBindingError(c, err)
		return nil, "", bodySize, false
	}
	input, err := service.DecodeDesktopConversation(raw)
	if err != nil {
		desktopError(c, err)
		return nil, "", bodySize, false
	}
	return input, input.RecordID, bodySize, true
}

func (h *DesktopHandler) ListManagedConversations(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	userID, ok := desktopManagedUserID(c)
	if !ok {
		return
	}
	params, filters, err := dto.ParseDesktopConversationQuery(c.Request.URL.Query())
	if response.ErrorFrom(c, err) {
		return
	}
	items, page, err := h.desktop.ListConversations(c.Request.Context(), "", userID, params, filters)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Paginated(c, items, page.Total, page.Page, page.PageSize)
}

func (h *DesktopHandler) GetManagedConversation(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	userID, ok := desktopManagedUserID(c)
	if !ok {
		return
	}
	result, err := h.desktop.GetConversation(c.Request.Context(), "", userID, c.Param("record_id"))
	if response.ErrorFrom(c, err) {
		return
	}
	middleware.SetAuditExtra(c, map[string]any{"record_id": result.RecordID, "organization_id": result.OrganizationID})
	response.Success(c, result)
}

func (h *DesktopHandler) ManagedConversationStatistics(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	userID, ok := desktopManagedUserID(c)
	if !ok {
		return
	}
	_, filters, err := dto.ParseDesktopConversationQuery(c.Request.URL.Query())
	if response.ErrorFrom(c, err) {
		return
	}
	result, err := h.desktop.ConversationStatistics(c.Request.Context(), "", userID, filters)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}
