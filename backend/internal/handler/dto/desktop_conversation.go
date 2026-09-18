package dto

import (
	"net/url"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func ParseDesktopConversationQuery(query url.Values) (pagination.PaginationParams, service.DesktopConversationFilters, error) {
	params := pagination.DefaultPagination()
	filters := service.DesktopConversationFilters{
		MemberID: query.Get("member_id"), MemberSearch: query.Get("member_search"), Client: query.Get("client"),
		CaptureStatus: query.Get("capture_status"), RecordID: query.Get("record_id"),
		SourceSessionID: query.Get("source_session_id"), InstallationID: query.Get("installation_id"),
	}
	for key, target := range map[string]*int{"page": &params.Page, "page_size": &params.PageSize} {
		if value := query.Get(key); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 1 || parsed > 1000000 {
				return params, filters, service.ErrDesktopValidation
			}
			*target = parsed
		}
	}
	if value := query.Get("sort_order"); value != "" {
		params.SortOrder = value
	}
	if params.PageSize > 100 || (params.SortOrder != "asc" && params.SortOrder != "desc") {
		return params, filters, service.ErrDesktopValidation
	}
	for key, target := range map[string]**time.Time{"received_from": &filters.ReceivedFrom, "received_to": &filters.ReceivedTo} {
		if value := query.Get(key); value != "" {
			parsed, err := time.Parse(time.RFC3339Nano, value)
			if err != nil {
				return params, filters, service.ErrDesktopValidation
			}
			*target = &parsed
		}
	}
	return params, filters, filters.Validate()
}
