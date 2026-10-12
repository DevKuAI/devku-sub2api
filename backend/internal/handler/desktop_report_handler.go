package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *DesktopHandler) ManagedDailyReports(c *gin.Context) {
	userID, ok := desktopManagedUserID(c)
	if !ok {
		return
	}
	query, err := dto.ParseDesktopReportQuery(c)
	if response.ErrorFrom(c, err) {
		return
	}
	day, err := h.desktop.ReadReportDay(c.Request.Context(), "", userID, desktopReportDate(c), query)
	if response.ErrorFrom(c, err) {
		return
	}
	writeDesktopReport(c, day)
}
func desktopReportDate(c *gin.Context) string {
	if date := c.Param("date"); date != "" {
		return date
	}
	return c.Query("date")
}
func writeDesktopReport(c *gin.Context, day *service.DesktopReportDay) {
	c.Header("Cache-Control", "no-store")
	if c.Param("member_id") != "" {
		for _, member := range day.Members {
			if member.MemberID == c.Param("member_id") {
				response.Success(c, member)
				return
			}
		}
		response.ErrorFrom(c, service.ErrDesktopMemberNotFound)
		return
	}
	if strings.HasSuffix(c.Request.URL.Path, "/summary") {
		response.Success(c, gin.H{"date": day.Date, "timezone": day.Timezone, "analysis_model": day.AnalysisModel, "status": day.Status, "reason": day.Reason, "task": day.Task, "total_members": day.TotalMembers, "no_record_members": day.NoRecordMembers, "page": day.Page, "page_size": day.PageSize, "queue": day.Queue, "missing_dates": day.MissingDates, "summary": day.Summary, "expected_members": day.ExpectedMembers, "completed_members": day.CompletedMembers, "missing_members": day.MissingMembers})
		return
	}
	// Metadata endpoint supports separate member and summary reads.
	response.Success(c, gin.H{"date": day.Date, "timezone": day.Timezone, "analysis_model": day.AnalysisModel, "status": day.Status, "reason": day.Reason, "task": day.Task, "total_members": day.TotalMembers, "no_record_members": day.NoRecordMembers, "page": day.Page, "page_size": day.PageSize, "queue": day.Queue, "missing_dates": day.MissingDates, "members": day.Members, "expected_members": day.ExpectedMembers, "completed_members": day.CompletedMembers, "missing_members": day.MissingMembers})
}
