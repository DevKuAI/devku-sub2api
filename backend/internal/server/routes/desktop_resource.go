package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/desktopresource"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func registerDesktopResourceAdminRoutes(admin *gin.RouterGroup, h *handler.Handlers, audit middleware.AuditLogMiddleware, settings *service.SettingService, cfg *config.Config) {
	if cfg == nil || !cfg.Desktop.Enabled {
		return
	}
	group := admin.Group("/desktop/resources")
	group.Use(gin.HandlerFunc(audit), middleware.AdminComplianceGuard(settings))
	group.Use(func(c *gin.Context) { c.Header("Cache-Control", "no-store"); c.Next() })
	group.POST("", middleware.StrictBodyLimit(desktopresource.MaxZIPBytes), h.Admin.DesktopResource.Publish)
	group.POST("/validate", middleware.StrictBodyLimit(desktopresource.MaxZIPBytes), h.Admin.DesktopResource.Validate)
	group.GET("", h.Admin.DesktopResource.List)
	group.GET("/:resource_id", h.Admin.DesktopResource.Get)
	group.GET("/:resource_id/versions", h.Admin.DesktopResource.Versions)
	group.PATCH("/:resource_id/status", middleware.StrictBodyLimit(8<<10), h.Admin.DesktopResource.SetStatus)
	group.POST("/:resource_id/versions/:version/artifacts/:platform/download-url", middleware.StrictBodyLimit(8<<10), h.Admin.DesktopResource.DownloadURL)
}
