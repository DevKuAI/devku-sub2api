package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func (h *DesktopHandler) ListConversations(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	params, filters, err := dto.ParseDesktopConversationQuery(c.Request.URL.Query())
	if response.ErrorFrom(c, err) {
		return
	}
	items, page, err := h.desktop.ListConversations(c.Request.Context(), c.Param("organization_id"), 0, params, filters)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Paginated(c, items, page.Total, page.Page, page.PageSize)
}

func (h *DesktopHandler) GetConversation(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	result, err := h.desktop.GetConversation(c.Request.Context(), c.Param("organization_id"), 0, c.Param("record_id"))
	if response.ErrorFrom(c, err) {
		return
	}
	middleware.SetAuditExtra(c, map[string]any{"record_id": result.RecordID, "organization_id": result.OrganizationID})
	response.Success(c, result)
}
