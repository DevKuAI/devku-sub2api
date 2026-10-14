package bi

import (
	"slices"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *Handler) readPeriod(c *gin.Context, access Access, operation string, page, size int) (any, error) {
	ctx := c.Request.Context()
	d := h.service.desktop
	org := access.OrganizationID
	id := c.Param("report_id")
	switch operation {
	case "periods":
		items, total, e := d.ListPeriodReports(ctx, org, c.Query("kind"), c.Query("period_start"), page, size, c.Query("latest_readable") == "true")
		return pageResult(items, page, size, total), e
	case "period", "period_text":
		t, e := d.ReadPeriodReport(ctx, org, id, true)
		if e != nil {
			return nil, e
		}
		if operation == "period_text" {
			if t.Content == nil {
				return nil, apiError(409, "CONFLICT", "Summary not ready")
			}
			return gin.H{"text": *t.Content, "revision": t.Revision}, nil
		}
		v := jsonObject(t.DesktopPeriodMetadata)
		v["content"] = t.Content
		if t.Content != nil {
			dates := make([]string, 0, len(t.Sources))
			for _, source := range t.Sources {
				dates = append(dates, source.Date)
			}
			outline := service.BuildDesktopSummaryOutline(*t.Content, dates)
			v["summary_outline"] = outline.Sections
			v["validated_source_dates"] = outline.ValidatedDates
			v["unverified_source_dates"] = outline.UnverifiedDates
		}
		v["can_copy"] = slices.Contains(access.Capabilities, "reports:copy")
		return v, nil
	case "sources":
		items, e := d.PeriodReportSources(ctx, org, id)
		if e != nil {
			return nil, e
		}
		total := len(items)
		start := min((page-1)*size, total)
		end := min(start+size, total)
		return pageResult(items[start:end], page, size, int64(total)), nil
	case "source":
		return d.PeriodReportSource(ctx, org, id, c.Param("date"))
	}
	return nil, ErrNotFound
}
