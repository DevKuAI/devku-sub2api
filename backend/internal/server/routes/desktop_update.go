package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func RegisterDesktopUpdateRoutes(root *gin.Engine, h *handler.Handlers, configs ...*config.Config) {
	var cfg *config.Config
	if len(configs) > 0 {
		cfg = configs[0]
	}
	var desktop *service.DesktopService
	if h.Desktop != nil && cfg != nil && cfg.Desktop.Enabled {
		desktop = h.Desktop.Service()
	}
	root.GET("/api/desktop/v1/update/:target/:arch/:current_version", middleware.DesktopOptionalSessionAuth(desktop), h.DesktopUpdate.Check)
	maxBytes := int64(200 << 20)
	if cfg != nil && cfg.DesktopUpdateStorage.MaxUploadBytes > 0 {
		maxBytes = cfg.DesktopUpdateStorage.MaxUploadBytes
	}
	root.PUT("/api/desktop/v1/releases/:version/import", middleware.DesktopUpdateUploadAuth(cfg), middleware.StrictBodyLimit(3*maxBytes+(1<<20)), h.Admin.DesktopUpdate.Import)
}
