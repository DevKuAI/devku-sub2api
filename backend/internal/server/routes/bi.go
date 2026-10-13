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
	api := r.Group("/api/bi/v1", bi.Boundary)
	api.GET("/bootstrap", h.Bootstrap)
	auth := api.Group("/auth", h.RateLimit(middleware.SecurityClientIP))
	auth.POST("/wechat", h.Login)
	auth.POST("/refresh", h.Refresh)
	auth.POST("/logout", h.Logout)
	mobile := api.Group("", h.MobileAuth)
	mobile.GET("/me", h.Me)
	mobile.GET("/organizations", h.Organizations)
	invites := mobile.Group("/invitations", h.RateLimit(middleware.SecurityClientIP))
	invites.POST("/preview", h.PreviewInvitation)
	invites.POST("/accept", h.AcceptInvitation)
	desktop := mobile.Group("/organizations/:organization_id/desktop")
	for _, route := range []struct{ path, operation string }{{"", "organization"}, {"/overview", "overview"}, {"/usage", "usage"}, {"/models", "models"}, {"/members", "members"}, {"/members/:member_id", "member"}, {"/members/:member_id/models", "member_models"}, {"/clients", "clients"}, {"/clients/:client", "client"}, {"/conversations/statistics", "conversation_statistics"}, {"/conversations", "conversations"}, {"/conversations/:record_id", "conversation"}, {"/daily-reports", "daily"}, {"/daily-reports/:date/summary", "daily_summary"}, {"/daily-reports/:date/members", "daily_members"}, {"/daily-reports/:date/members/:member_id", "daily_member"}, {"/daily-reports/:date/text", "daily_text"}, {"/period-reports", "periods"}, {"/period-reports/:report_id", "period"}, {"/period-reports/:report_id/sources", "sources"}, {"/period-reports/:report_id/sources/:date", "source"}, {"/period-reports/:report_id/text", "period_text"}} {
		desktop.GET(route.path, h.NativeRead(route.operation))
	}
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
	root := admin.Group("/bi", bi.Boundary, func(c *gin.Context) { c.Set("bi.admin", true); c.Next() }, h.BI.ManagerScope)
	root.GET("/overview", h.BI.OperationsOverview)
	if !cfg.BI.Enabled {
		return
	}
	registerBIManagement(root.Group("/organizations/:organization_id"), h.BI, stepUp)
	root.GET("/organizations", h.BI.OperationsOrganizations)
	root.GET("/identities/users", h.BI.OperationsUsers)
	root.GET("/identities/sessions", h.BI.OperationsSessions)
	root.PUT("/identities/users/:bi_user_id/status", gin.HandlerFunc(stepUp), h.BI.SetUserStatus)
	root.GET("/report-tasks", h.BI.OperationsTasks)
	root.GET("/report-tasks/:kind/:task_id", h.BI.OperationsTask)
	root.GET("/report-tasks/:kind/:task_id/executions", h.BI.OperationsTaskExecutions)
	root.GET("/report-executions", h.BI.OperationsExecutions)
	root.GET("/audit", h.BI.OperationsAudit)
	root.GET("/retention", h.BI.GetRetention)
	root.PUT("/retention", gin.HandlerFunc(stepUp), h.BI.SaveRetention)
	root.GET("/cleanup", h.BI.CleanupStatus)
	root.POST("/cleanup/run", gin.HandlerFunc(stepUp), h.BI.RunCleanup)
}
