package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type DesktopUpdateHandler struct {
	updates *service.DesktopUpdateService
}

func NewDesktopUpdateHandler(updates *service.DesktopUpdateService) *DesktopUpdateHandler {
	return &DesktopUpdateHandler{updates: updates}
}

func (h *DesktopUpdateHandler) Check(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	result, available, err := h.updates.Check(
		c.Request.Context(),
		c.Param("target"),
		c.Param("arch"),
		c.Param("current_version"),
	)
	if err != nil {
		desktopError(c, err)
		return
	}
	if !available {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, result)
}
