package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
)

func RegisterDesktopUpdateRoutes(root *gin.Engine, h *handler.Handlers) {
	root.GET("/api/desktop/v1/update/:target/:arch/:current_version", h.DesktopUpdate.Check)
}
