package handler

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/desktopresponse"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type DesktopResourceHandler struct {
	resources *service.DesktopResourceService
	desktop   *service.DesktopService
}

func NewDesktopResourceHandler(resources *service.DesktopResourceService, desktop *service.DesktopService) *DesktopResourceHandler {
	return &DesktopResourceHandler{resources: resources, desktop: desktop}
}

func (h *DesktopResourceHandler) touch(c *gin.Context) bool {
	auth, ok := middleware.GetDesktopAuthorization(c)
	if !ok {
		desktopresponse.Error(c, service.ErrDesktopUnauthenticated)
		return false
	}
	if err := h.desktop.TouchSession(c.Request.Context(), auth); err != nil {
		desktopresponse.Error(c, err)
		return false
	}
	return true
}

func (h *DesktopResourceHandler) List(c *gin.Context) {
	scope := c.DefaultQuery("scope", "public")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit < 1 || limit > 100 {
		limit = 50
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	f := service.DesktopResourceFilter{Kind: c.Query("kind"), Search: c.Query("search"), Limit: limit, Offset: offset, Status: "active"}
	if (scope != "public" && scope != "enterprise") || f.Validate() != nil {
		desktopresponse.Error(c, service.ErrResourceValidation)
		return
	}
	items := make([]service.DesktopResource, 0)
	if scope == "public" {
		rows, _, err := h.resources.List(c.Request.Context(), f)
		if err != nil {
			desktopresponse.Error(c, err)
			return
		}
		for _, row := range rows {
			items = append(items, row.Public())
		}
	}
	if h.touch(c) {
		desktopresponse.Success(c, items)
	}
}

func (h *DesktopResourceHandler) Get(c *gin.Context) {
	item, err := h.resources.Get(c.Request.Context(), c.Param("resource_id"), true)
	if err != nil {
		desktopresponse.Error(c, err)
		return
	}
	if h.touch(c) {
		desktopresponse.Success(c, item.Public())
	}
}

func (h *DesktopResourceHandler) Download(c *gin.Context) {
	artifact, body, err := h.resources.Download(c.Request.Context(), c.Param("resource_id"), c.Param("version"), c.Param("platform"))
	if err != nil {
		desktopresponse.Error(c, err)
		return
	}
	defer func() { _ = body.Close() }()
	if !h.touch(c) {
		return
	}
	c.DataFromReader(http.StatusOK, artifact.SizeBytes, "application/zip", body, map[string]string{
		"Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff",
		"Content-Disposition": `attachment; filename="resource.zip"`, "X-Artifact-SHA256": artifact.SHA256,
	})
}

func (h *DesktopResourceHandler) DownloadURL(c *gin.Context) {
	link, err := h.resources.DownloadURL(c.Request.Context(), c.Param("resource_id"), c.Param("version"), c.Param("platform"))
	if err != nil {
		desktopresponse.Error(c, err)
		return
	}
	if h.touch(c) {
		desktopresponse.Success(c, link)
	}
}
