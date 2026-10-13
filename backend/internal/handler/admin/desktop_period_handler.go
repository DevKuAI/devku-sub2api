package admin

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *DesktopHandler) PeriodReports(c *gin.Context) {
	ctx := c.Request.Context()
	org := c.Param("organization_id")
	page, size := response.ParsePagination(c)
	if page > 100000 || size > 100 {
		response.ErrorFrom(c, service.ErrDesktopValidation)
		return
	}
	var value any
	var e error
	id := c.Param("report_id")
	if id == "" {
		items, total, err := h.desktop.ListPeriodReports(ctx, org, c.Query("kind"), c.Query("period_start"), page, size)
		value = gin.H{"items": items, "total": total, "page": page, "page_size": size, "has_more": page*size < int(total)}
		e = err
	} else if c.Param("date") != "" {
		value, e = h.desktop.PeriodReportSource(ctx, org, id, c.Param("date"))
	} else if strings.HasSuffix(c.FullPath(), "/sources") {
		items, err := h.desktop.PeriodReportSources(ctx, org, id)
		e = err
		total := len(items)
		start := min((page-1)*size, total)
		end := min(start+size, total)
		value = gin.H{"items": items[start:end], "total": total, "page": page, "page_size": size, "has_more": end < total}
	} else {
		t, err := h.desktop.ReadPeriodReport(ctx, org, id, true)
		e = err
		if err == nil {
			value = gin.H{"report": t.DesktopPeriodMetadata, "content": t.Content}
		}
	}
	if response.ErrorFrom(c, e) {
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, value)
}
func (h *DesktopHandler) RunPeriodReport(c *gin.Context) {
	var input struct {
		Kind  string `json:"kind" binding:"required,oneof=week month"`
		Start string `json:"period_start"`
		Mode  string `json:"mode" binding:"required,oneof=generate retry regenerate"`
	}
	if e := c.ShouldBindJSON(&input); e != nil {
		adminDesktopBindingError(c, e)
		return
	}
	t, e := h.desktop.RunPeriodReport(c.Request.Context(), c.Param("organization_id"), input.Kind, input.Start, input.Mode)
	if response.ErrorFrom(c, e) {
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.JSON(http.StatusAccepted, gin.H{"code": 0, "message": "success", "data": t.DesktopPeriodMetadata})
}
