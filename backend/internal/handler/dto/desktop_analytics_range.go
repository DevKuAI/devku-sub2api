package dto

import (
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func ParseDesktopAnalyticsRange(query url.Values) (*service.DesktopAnalyticsRangeInput, error) {
	if !query.Has("days") && !query.Has("from") && !query.Has("to") && !query.Has("as_of") {
		return nil, nil
	}
	for _, key := range []string{"days", "from", "to"} {
		if query.Has(key) && query.Get(key) == "" {
			return nil, service.ErrDesktopValidation
		}
	}
	input := &service.DesktopAnalyticsRangeInput{FromDate: query.Get("from"), ToDate: query.Get("to")}
	if query.Has("as_of") && query.Get("as_of") == "" {
		return nil, service.ErrDesktopValidation
	}
	if raw := query.Get("as_of"); raw != "" {
		at, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return nil, service.ErrDesktopValidation
		}
		input.AsOf = at
	}
	if value := query.Get("days"); value != "" {
		days, err := strconv.Atoi(value)
		if err != nil {
			return nil, service.ErrDesktopValidation
		}
		input.Days = days
	}
	return input, nil
}

func ParseDesktopUsageQuery(query url.Values) (service.DesktopUsageQuery, error) {
	result := service.DesktopUsageQuery{Section: query.Get("section"), Page: 1, PageSize: 20, Search: strings.TrimSpace(query.Get("search")), Model: strings.TrimSpace(query.Get("model")), Sort: query.Get("sort")}
	if result.Section == "" {
		result.Section = "insights"
	}
	if result.Section != "summary" && result.Section != "insights" && result.Section != "member_models" {
		return result, service.ErrDesktopValidation
	}
	if result.Sort != "" && result.Sort != "name" && result.Sort != "tokens" {
		return result, service.ErrDesktopValidation
	}
	if utf8.RuneCountInString(result.Search) > 100 || utf8.RuneCountInString(result.Model) > 200 {
		return result, service.ErrDesktopValidation
	}
	for _, item := range []struct {
		name   string
		target *int
	}{{"page", &result.Page}, {"page_size", &result.PageSize}} {
		if raw := query.Get(item.name); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || value > 1000000 {
				return result, service.ErrDesktopValidation
			}
			*item.target = value
		}
	}
	if result.PageSize > 100 {
		return result, service.ErrDesktopValidation
	}
	return result, nil
}
