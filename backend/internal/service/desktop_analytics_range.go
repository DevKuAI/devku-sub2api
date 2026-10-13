package service

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

type DesktopAnalyticsRangeInput struct {
	Days     int
	Timezone string
	FromDate string
	ToDate   string
}

type DesktopAnalyticsRange struct {
	Start         time.Time
	End           time.Time
	PreviousStart time.Time
	PreviousEnd   time.Time
	StartDate     string
	EndDate       string
	Days          int
}

func (input DesktopAnalyticsRangeInput) Resolve(now time.Time) (DesktopAnalyticsRange, error) {
	location := timezone.Location()
	if input.Timezone != "" {
		var err error
		location, err = time.LoadLocation(input.Timezone)
		if err != nil {
			return DesktopAnalyticsRange{}, ErrDesktopValidation
		}
	}
	localNow := now.In(location)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	var start, last time.Time
	var days int
	if input.FromDate != "" || input.ToDate != "" {
		if input.Days != 0 || input.FromDate == "" || input.ToDate == "" {
			return DesktopAnalyticsRange{}, ErrDesktopValidation
		}
		var err error
		start, err = time.ParseInLocation("2006-01-02", input.FromDate, location)
		if err != nil {
			return DesktopAnalyticsRange{}, ErrDesktopValidation
		}
		last, err = time.ParseInLocation("2006-01-02", input.ToDate, location)
		if err != nil || start.After(last) || last.After(today) {
			return DesktopAnalyticsRange{}, ErrDesktopValidation
		}
		for day := start; !day.After(last) && days <= 90; day = day.AddDate(0, 0, 1) {
			days++
		}
		if days > 90 {
			return DesktopAnalyticsRange{}, ErrDesktopValidation
		}
	} else {
		days = input.Days
		if days == 0 {
			days = 30
		}
		if days != 7 && days != 30 && days != 90 {
			return DesktopAnalyticsRange{}, ErrDesktopValidation
		}
		start, last = today.AddDate(0, 0, 1-days), today
	}
	end := last.AddDate(0, 0, 1)
	if last.Equal(today) {
		end = now.In(location)
	}
	return DesktopAnalyticsRange{
		Start: start, End: end, PreviousStart: start.AddDate(0, 0, -days), PreviousEnd: end.AddDate(0, 0, -days),
		StartDate: start.Format("2006-01-02"), EndDate: last.Format("2006-01-02"), Days: days,
	}, nil
}
