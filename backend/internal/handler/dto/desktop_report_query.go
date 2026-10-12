package dto

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
	"strings"
	"time"
)

func ParseDesktopReportQuery(c *gin.Context) (service.DesktopReportQuery, error) {
	query := service.DesktopReportQuery{Section: "members", MemberID: c.Param("member_id"), Search: c.Query("search"), Page: 1, PageSize: 20, ExecutionPage: 1}
	if strings.HasSuffix(c.Request.URL.Path, "/summary") {
		query.Section = "summary"
	}
	for _, value := range []struct {
		name   string
		target *int
	}{{"page", &query.Page}, {"page_size", &query.PageSize}, {"execution_page", &query.ExecutionPage}} {
		if raw, ok := c.GetQuery(value.name); ok {
			n, err := strconv.Atoi(raw)
			if err != nil {
				return query, service.ErrDesktopValidation
			}
			*value.target = n
		}
	}
	return query, nil
}

func ParseDesktopUsageAsOf(input string) (time.Time, error) {
	if input == "" {
		return time.Time{}, nil
	}
	at, err := time.Parse(time.RFC3339Nano, input)
	if err != nil {
		return time.Time{}, service.ErrDesktopValidation
	}
	return at, nil
}
