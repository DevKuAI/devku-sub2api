package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func registerDesktopStructureRoutes(group *gin.RouterGroup, desktop *service.DesktopService, managed bool) {
	h := handler.NewDesktopStructureHandler(desktop, managed)
	h.Register(group)
}
