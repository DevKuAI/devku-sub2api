package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/bi"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterBIRoutes(r *gin.Engine, h *bi.Handler, _ middleware.JWTAuthMiddleware, cfg *config.Config) {
	if cfg == nil || !cfg.BI.Enabled || h == nil {
		return
	}
	h.RegisterMobileRoutes(r)
	h.Start()
}
func registerBIManagement(group *gin.RouterGroup, h *bi.Handler, stepUp middleware.StepUpAuthMiddleware) {
	group.Use(bi.Boundary, h.ManagerScope)
	group.GET("/invitations", h.ListInvitations)
	group.GET("/users", h.ListAccess)
	secure := group.Group("", gin.HandlerFunc(stepUp))
	secure.POST("/invitations", h.CreateInvitation)
	secure.POST("/invitations/:invitation_id/revoke", h.RevokeInvitation)
	secure.PUT("/users/:bi_user_id/access", h.UpdateAccess)
}
func registerBIManagedRoutes(group *gin.RouterGroup, h *handler.Handlers, cfg *config.Config, stepUp middleware.StepUpAuthMiddleware) {
	if cfg == nil || !cfg.BI.Enabled || !cfg.Desktop.Enabled || h.BI == nil {
		return
	}
	registerBIManagement(group.Group("/desktop/organization/bi"), h.BI, stepUp)
}
func registerBIAdminRoutes(admin *gin.RouterGroup, h *handler.Handlers, cfg *config.Config, stepUp middleware.StepUpAuthMiddleware) {
	if cfg == nil || h.BI == nil {
		return
	}
	root := admin.Group("/desktop/operations", bi.Boundary, func(c *gin.Context) { c.Set("bi.admin", true); c.Next() }, h.BI.ManagerScope)
	root.GET("/overview", h.BI.OperationsOverview)
	root.GET("/organizations", h.BI.OperationsOrganizations)
	root.GET("/report-tasks", h.BI.OperationsTasks)
	root.GET("/report-tasks/:kind/:task_id", h.BI.OperationsTask)
	root.GET("/report-tasks/:kind/:task_id/executions", h.BI.OperationsTaskExecutions)
	root.GET("/report-executions", h.BI.OperationsExecutions)
	root.GET("/report-statistics", h.BI.OperationsReportStatistics)
	if !cfg.BI.Enabled {
		return
	}
	registerBIManagement(root.Group("/organizations/:organization_id"), h.BI, stepUp)
	root.GET("/identities/users", h.BI.OperationsUsers)
	root.GET("/identities/sessions", h.BI.OperationsSessions)
	root.PUT("/identities/users/:bi_user_id/status", gin.HandlerFunc(stepUp), h.BI.SetUserStatus)
	root.GET("/audit", h.BI.OperationsAudit)
	root.GET("/retention", h.BI.GetRetention)
	root.PUT("/retention", gin.HandlerFunc(stepUp), h.BI.SaveRetention)
	root.GET("/cleanup", h.BI.CleanupStatus)
	root.POST("/cleanup/run", gin.HandlerFunc(stepUp), h.BI.RunCleanup)
}
