package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/bi"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterBIRoutes(r *gin.Engine, h *bi.Handler, jwtAuth middleware.JWTAuthMiddleware, cfg *config.Config) {
	if cfg == nil || !cfg.BI.Enabled || h == nil {
		return
	}
	api := r.Group("/api/bi/v1")
	api.Use(bi.Boundary)
	api.Use(func(c *gin.Context) {
		middleware.SetErrorResponder(c, bi.WriteMiddlewareError)
		c.Next()
	})
	api.GET("/bootstrap", h.Bootstrap)
	auth := api.Group("/auth")
	auth.Use(h.RateLimit(middleware.SecurityClientIP))
	auth.POST("/wechat", h.Login)
	auth.POST("/bindings/exchange", h.ExchangeBinding)
	auth.POST("/refresh", h.Refresh)
	auth.POST("/logout", h.Logout)
	web := auth.Group("")
	web.Use(gin.HandlerFunc(jwtAuth), h.RequireWebAudience)
	web.POST("/bindings/approve", h.ApproveBinding)
	web.GET("/bindings", h.ListBindings)
	web.DELETE("/bindings/:binding_id", h.RevokeBinding)
	mobile := api.Group("")
	mobile.Use(h.MobileAuth)
	mobile.GET("/me", h.Me)
	mobile.GET("/organizations", h.Organizations)
	mobile.DELETE("/auth/binding", h.RemoveBinding)
	analytics := mobile.Group("/organizations/:organization_id")
	analytics.GET("/data-status", h.DataStatus)
	analytics.GET("/metric-definitions", h.MetricDefinitions)
	analytics.GET("/filter-options", h.FilterOptions)
	analytics.POST("/analysis-contexts", h.CreateAnalysisContext)
	analytics.GET("/analysis-contexts/:context_id", h.GetAnalysisContext)
	for _, operation := range []string{"overview", "adoption", "tokens", "feedback", "assets", "trend", "comparisons"} {
		analytics.GET("/analytics/"+operation, h.QueryAnalysis(operation))
	}
	analytics.GET("/teams/:team_id", h.QueryAnalysis("team"))
	analytics.GET("/members", h.QueryAnalysis("members"))
	analytics.GET("/members/:member_id", h.QueryAnalysis("member"))
	analytics.GET("/applications", h.QueryAnalysis("applications"))
	analytics.GET("/applications/:application_id", h.QueryAnalysis("application"))
	analytics.GET("/scenes", h.QueryAnalysis("scenes"))
	analytics.GET("/scenes/:scene_id", h.QueryAnalysis("scene"))
	analytics.GET("/knowledge", h.QueryContent("knowledge"))
	analytics.GET("/knowledge/:knowledge_id", h.QueryContent("knowledge_detail"))
	analytics.GET("/knowledge/:knowledge_id/versions", h.QueryContent("versions"))
	analytics.GET("/knowledge/:knowledge_id/versions/:version_id", h.QueryContent("version"))
	analytics.GET("/knowledge/:knowledge_id/references", h.QueryContent("references"))
	analytics.GET("/cases", h.QueryContent("cases"))
	analytics.GET("/cases/:case_id", h.QueryContent("case"))
	analytics.GET("/sources/:source_id", h.QueryContent("source"))
	analytics.GET("/evaluations", h.QueryContent("evaluations"))
	analytics.GET("/evaluations/:evaluation_id", h.QueryContent("evaluation"))
	analytics.GET("/evaluations/:evaluation_id/samples", h.QueryContent("samples"))
	analytics.GET("/search", h.QueryContent("search"))
	analytics.GET("/me/favorites", h.QueryContent("favorites"))
	analytics.PUT("/me/favorites/:knowledge_id", h.PutFavorite)
	analytics.DELETE("/me/favorites/:knowledge_id", h.DeleteFavorite)
	analytics.GET("/me/knowledge-notes/:knowledge_id", h.GetNote)
	analytics.PUT("/me/knowledge-notes/:knowledge_id", h.PutNote)
	analytics.DELETE("/me/knowledge-notes/:knowledge_id", h.DeleteNote)
	analytics.POST("/reports", h.CreateReport)
	analytics.GET("/reports", h.ListReports)
	analytics.GET("/reports/:report_id", h.GetReport)
	analytics.GET("/reports/:report_id/text", h.GetReportText)
	analytics.POST("/reports/:report_id/shares", h.CreateShare)
	analytics.GET("/reports/:report_id/shares", h.ListShares)
	analytics.DELETE("/shares/:share_id", h.DeleteShare)
	mobile.GET("/shares/:share_id", h.ResolveShare)
	ingestion := api.Group("/internal/ingestion", h.ConnectorAuth)
	ingestion.POST("/batches", h.SubmitImport)
	ingestion.GET("/batches/:batch_id", h.GetImport)
	ingestion.GET("/sources/:source_id/checkpoint", h.GetCheckpoint)
	h.Start()
}

func registerBIAdminRoutes(admin *gin.RouterGroup, h *handler.Handlers, cfg *config.Config) {
	if cfg == nil || !cfg.BI.Enabled || h.BI == nil {
		return
	}
	group := admin.Group("/bi/organizations/:organization_id/grants")
	group.Use(bi.Headers)
	group.GET("", h.BI.AdminListGrants)
	group.PUT("", func(c *gin.Context) {
		actor, ok := middleware.GetAuthSubjectFromContext(c)
		if !ok {
			bi.WriteError(c, bi.ErrUnauthenticated)
			return
		}
		h.BI.AdminSaveGrant(c, actor.UserID)
	})
	group.POST("/:manager_id/revoke", func(c *gin.Context) {
		actor, ok := middleware.GetAuthSubjectFromContext(c)
		if !ok {
			bi.WriteError(c, bi.ErrUnauthenticated)
			return
		}
		h.BI.AdminRevokeGrant(c, actor.UserID)
	})
}
