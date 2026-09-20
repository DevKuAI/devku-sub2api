package admin

import (
	"mime"

	"github.com/Wei-Shaw/sub2api/internal/pkg/desktopresponse"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type DesktopResourceHandler struct {
	resources *service.DesktopResourceService
}

func NewDesktopResourceHandler(resources *service.DesktopResourceService) *DesktopResourceHandler {
	return &DesktopResourceHandler{resources: resources}
}

func resourceZIPRequest(c *gin.Context) bool {
	contentType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || contentType != "application/zip" {
		desktopresponse.Error(c, service.ErrResourceValidation)
		return false
	}
	return true
}

func (h *DesktopResourceHandler) Validate(c *gin.Context) {
	if !resourceZIPRequest(c) {
		return
	}
	result, err := h.resources.Validate(c.Request.Context(), c.Request.Body)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *DesktopResourceHandler) Publish(c *gin.Context) {
	if !resourceZIPRequest(c) {
		return
	}
	result, err := h.resources.Publish(c.Request.Context(), c.Request.Body, adminDesktopUpdateActorID(c))
	if err != nil {
		desktopresponse.Error(c, err)
		return
	}
	middleware.SetAuditExtra(c, map[string]any{"resource_id": result.ID, "version": result.Version, "platform": result.Artifacts[0].Platform, "sha256": result.Artifacts[0].SHA256})
	desktopresponse.Success(c, result.Public())
}

func resourceAdminPage(c *gin.Context) (int, int, bool) {
	page, size := response.ParsePagination(c)
	if size > 100 || page > 10001 || (page-1)*size > 10000 {
		response.ErrorFrom(c, service.ErrResourceValidation)
		return 0, 0, false
	}
	return page, size, true
}

func (h *DesktopResourceHandler) List(c *gin.Context) {
	page, size, ok := resourceAdminPage(c)
	if !ok {
		return
	}
	items, total, err := h.resources.List(c.Request.Context(), service.DesktopResourceFilter{Kind: c.Query("kind"), Status: c.Query("status"), Key: c.Query("key"), Search: c.Query("search"), Limit: size, Offset: (page - 1) * size})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Paginated(c, items, total, page, size)
}

func (h *DesktopResourceHandler) Get(c *gin.Context) {
	item, err := h.resources.Get(c.Request.Context(), c.Param("resource_id"), false)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, item)
}

func (h *DesktopResourceHandler) Versions(c *gin.Context) {
	page, size, ok := resourceAdminPage(c)
	if !ok {
		return
	}
	items, total, err := h.resources.Versions(c.Request.Context(), c.Param("resource_id"), size, (page-1)*size)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Paginated(c, items, total, page, size)
}

func (h *DesktopResourceHandler) SetStatus(c *gin.Context) {
	var input struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		adminDesktopBindingError(c, err)
		return
	}
	item, err := h.resources.SetStatus(c.Request.Context(), c.Param("resource_id"), input.Status, input.Reason, adminDesktopUpdateActorID(c))
	if response.ErrorFrom(c, err) {
		return
	}
	middleware.SetAuditExtra(c, map[string]any{"resource_id": item.ID, "status": item.Status})
	response.Success(c, item)
}

func (h *DesktopResourceHandler) DownloadURL(c *gin.Context) {
	link, err := h.resources.DownloadURL(c.Request.Context(), c.Param("resource_id"), c.Param("version"), c.Param("platform"))
	if response.ErrorFrom(c, err) {
		return
	}
	middleware.SetAuditExtra(c, map[string]any{"resource_id": c.Param("resource_id"), "version": c.Param("version"), "platform": c.Param("platform")})
	response.Success(c, link)
}
