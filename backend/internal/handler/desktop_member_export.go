package handler

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *DesktopStructureHandler) ExportMemberUsage(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	managerID := int64(0)
	if h.managed {
		var ok bool
		managerID, ok = desktopManagedUserID(c)
		if !ok {
			return
		}
	}
	var format service.DesktopMemberExportFormat
	if err := dto.BindDesktopWrite(c, &format); response.ErrorFrom(c, err) {
		return
	}
	filters := service.DesktopMemberListFilters{Search: c.Query("search"), Status: c.Query("status"), DepartmentID: c.Query("department_id"), PositionID: c.Query("position_id"), Unassigned: c.Query("unassigned") == "true"}
	if raw := c.Query("unassigned"); raw != "" && raw != "true" && raw != "false" {
		response.ErrorFrom(c, service.ErrDesktopValidation)
		return
	}
	file, err := os.CreateTemp("", "desktop-member-usage-*.csv")
	if err != nil {
		response.ErrorFrom(c, service.ErrDesktopUsageUnavailable.WithCause(err))
		return
	}
	defer func() { _ = file.Close(); _ = os.Remove(file.Name()) }()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()
	err = h.desktop.ExportMemberUsage(ctx, c.Param("organization_id"), managerID, filters, format, file)
	if errors.Is(err, service.ErrDesktopExportEmpty) {
		c.Status(204)
		return
	}
	if response.ErrorFrom(c, err) {
		return
	}
	info, err := file.Stat()
	if err != nil {
		response.ErrorFrom(c, service.ErrDesktopUsageUnavailable.WithCause(err))
		return
	}
	if _, err = file.Seek(0, 0); err != nil {
		response.ErrorFrom(c, service.ErrDesktopUsageUnavailable.WithCause(err))
		return
	}
	c.DataFromReader(200, info.Size(), "text/csv; charset=utf-8", file, map[string]string{"Content-Disposition": `attachment; filename="desktop_member_usage.csv"`, "X-Content-Type-Options": "nosniff"})
}
