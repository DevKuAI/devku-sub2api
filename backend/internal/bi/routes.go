package bi

import (
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// RegisterMobileRoutes is shared by production and the real HTTP integration fixture.
func (h *Handler) RegisterMobileRoutes(r *gin.Engine) {
	api := r.Group("/api/bi/v1", Boundary)
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
}
