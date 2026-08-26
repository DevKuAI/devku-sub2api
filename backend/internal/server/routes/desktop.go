package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func RegisterDesktopRoutes(root *gin.Engine, h *handler.Handlers, desktop *service.DesktopService, auditLog middleware.AuditLogMiddleware) {
	api := root.Group("/api/desktop/v1")
	auth := api.Group("/auth")
	auth.Use(middleware.StrictBodyLimit(8 * 1024))
	auth.Use(gin.HandlerFunc(auditLog))
	{
		auth.POST("/organization-lookup", h.Desktop.OrganizationLookup)
		auth.POST("/login", h.Desktop.Login)
		auth.POST("/refresh", h.Desktop.Refresh)
		auth.POST("/logout", middleware.DesktopAuth(desktop), h.Desktop.Logout)
	}

	protected := api.Group("")
	protected.Use(middleware.DesktopAuth(desktop))
	{
		protected.GET("/me", h.Desktop.Me)
		protected.GET("/model-configuration", h.Desktop.ModelConfiguration)
		protected.GET("/usage/summary", h.Desktop.UsageSummary)
	}
}
