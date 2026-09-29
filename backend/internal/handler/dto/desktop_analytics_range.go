package dto

import (
	"net/url"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func ParseDesktopAnalyticsRange(query url.Values) (*service.DesktopAnalyticsRangeInput, error) {
	if !query.Has("days") && !query.Has("from") && !query.Has("to") {
		return nil, nil
	}
	for _, key := range []string{"days", "from", "to"} {
		if query.Has(key) && query.Get(key) == "" {
			return nil, service.ErrDesktopValidation
		}
	}
	input := &service.DesktopAnalyticsRangeInput{FromDate: query.Get("from"), ToDate: query.Get("to")}
	if value := query.Get("days"); value != "" {
		days, err := strconv.Atoi(value)
		if err != nil {
			return nil, service.ErrDesktopValidation
		}
		input.Days = days
	}
	return input, nil
}
