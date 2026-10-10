package admin

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *DesktopHandler) DailyReports(c *gin.Context) {
	date := c.Param("date")
	if date == "" {
		date = c.Query("date")
	}
	day, err := h.desktop.ReportDay(c.Request.Context(), c.Param("organization_id"), 0, date)
	if response.ErrorFrom(c, err) {
		return
	}
	c.Header("Cache-Control", "no-store")
	if id := c.Param("member_id"); id != "" {
		for _, member := range day.Members {
			if member.MemberID == id {
				response.Success(c, member)
				return
			}
		}
		response.ErrorFrom(c, service.ErrDesktopMemberNotFound)
		return
	}
	if strings.HasSuffix(c.Request.URL.Path, "/summary") {
		response.Success(c, gin.H{"date": day.Date, "timezone": day.Timezone, "analysis_model": day.AnalysisModel, "status": day.Status, "reason": day.Reason, "task": day.Task, "summary": day.Summary, "expected_members": day.ExpectedMembers, "completed_members": day.CompletedMembers, "missing_members": day.MissingMembers})
		return
	}
	response.Success(c, gin.H{"date": day.Date, "timezone": day.Timezone, "analysis_model": day.AnalysisModel, "status": day.Status, "reason": day.Reason, "task": day.Task, "members": day.Members, "expected_members": day.ExpectedMembers, "completed_members": day.CompletedMembers, "missing_members": day.MissingMembers})
}
func (h *DesktopHandler) RunDailyReports(c *gin.Context) {
	var input struct {
		Mode string `json:"mode" binding:"required,oneof=generate retry regenerate"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		adminDesktopBindingError(c, err)
		return
	}
	task, err := h.desktop.RunReports(c.Request.Context(), c.Param("organization_id"), c.Param("date"), input.Mode)
	if response.ErrorFrom(c, err) {
		return
	}
	c.Header("Cache-Control", "no-store")
	if task == nil {
		response.Success(c, gin.H{"status": "no_records", "reason": "no_records"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"code": 0, "message": "success", "data": task})
}
