package handler

import (
	"io"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/desktopresponse"
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
func (h *DesktopResourceHandler) List(c *gin.Context) {
	member, ok := middleware.GetDesktopAuthorizedMember(c)
	if !ok {
		desktopError(c, service.ErrDesktopUnauthenticated)
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	result, err := h.resources.List(c.Request.Context(), member, service.DesktopResourceQuery{Scope: c.DefaultQuery("scope", "public"), Kind: c.Query("kind"), Search: c.Query("search"), Limit: limit, Offset: offset})
	if err != nil {
		desktopError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	desktopresponse.Success(c, result)
}
func (h *DesktopResourceHandler) Get(c *gin.Context) {
	member, ok := middleware.GetDesktopAuthorizedMember(c)
	if !ok {
		desktopError(c, service.ErrDesktopUnauthenticated)
		return
	}
	resource, err := h.resources.Get(c.Request.Context(), member, c.Param("resource_id"))
	if err != nil {
		desktopError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	desktopresponse.Success(c, resource)
}
func (h *DesktopResourceHandler) Download(c *gin.Context) {
	member, ok := middleware.GetDesktopAuthorizedMember(c)
	if !ok {
		desktopError(c, service.ErrDesktopUnauthenticated)
		return
	}
	artifact, data, err := h.resources.Download(c.Request.Context(), member, c.Param("resource_id"), c.Param("version"), c.Param("platform"))
	if err != nil {
		desktopError(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Artifact-SHA256", artifact.SHA256)
	c.Header("Content-Disposition", `attachment; filename="resource.zip"`)
	c.Data(http.StatusOK, "application/zip", data)
}
func (h *DesktopResourceHandler) PublishPublic(c *gin.Context) {
	// This handler is only mounted beneath the existing admin authentication/compliance guards.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.DesktopResourceMaxBytes)
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		desktopBindingError(c, err)
		return
	}
	result, err := h.resources.PublishPublic(c.Request.Context(), data)
	if err != nil {
		desktopError(c, err)
		return
	}
	desktopresponse.Success(c, result)
}
