package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
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
	userID := int64(0)
	if member, ok := middleware.GetDesktopAuthorizedMember(c); ok && member.Organization != nil {
		userID = member.Organization.GatewayUserID
	}
	result, available, err := h.updates.Check(
		c.Request.Context(),
		c.Param("target"),
		c.Param("arch"),
		c.Param("current_version"), userID,
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
