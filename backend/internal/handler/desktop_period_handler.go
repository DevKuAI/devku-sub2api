package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strings"
)

func (h *DesktopHandler) ManagedPeriodReports(c *gin.Context) {
	id, ok := desktopManagedUserID(c)
	if !ok {
		return
	}
	org, e := h.desktop.GetManagedOrganization(c.Request.Context(), id)
	if response.ErrorFrom(c, e) {
		return
	}
	page, size := response.ParsePagination(c)
	if page > 100000 || size > 100 {
		response.ErrorFrom(c, service.ErrDesktopValidation)
		return
	}
	value, e := readPeriodSection(c, h.desktop, org.PublicID, page, size)
	if response.ErrorFrom(c, e) {
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, value)
}
func readPeriodSection(c *gin.Context, d *service.DesktopService, org string, page, size int) (any, error) {
	ctx := c.Request.Context()
	id := c.Param("report_id")
	if id == "" {
		items, total, e := d.ListPeriodReports(ctx, org, c.Query("kind"), c.Query("period_start"), page, size)
		return gin.H{"items": items, "total": total, "page": page, "page_size": size, "has_more": page*size < int(total)}, e
	}
	if date := c.Param("date"); date != "" {
		return d.PeriodReportSource(ctx, org, id, date)
	}
	if strings.HasSuffix(c.FullPath(), "/sources") {
		items, e := d.PeriodReportSources(ctx, org, id)
		if e != nil {
			return nil, e
		}
		total := len(items)
		start := min((page-1)*size, total)
		end := min(start+size, total)
		return gin.H{"items": items[start:end], "total": total, "page": page, "page_size": size, "has_more": end < total}, nil
	}
	t, e := d.ReadPeriodReport(ctx, org, id, true)
	if e != nil {
		return nil, e
	}
	return gin.H{"report": t.DesktopPeriodMetadata, "content": t.Content}, nil
}
