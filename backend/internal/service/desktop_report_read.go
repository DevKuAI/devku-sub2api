package service

import (
	"context"
	"strings"
)

func (s *DesktopService) ReadReportDay(ctx context.Context, publicID string, managerID int64, input string, query DesktopReportQuery) (*DesktopReportDay, error) {
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > 100 || query.ExecutionPage < 1 || query.Page > 100000 || query.ExecutionPage > 100000 || len([]rune(query.Search)) > 100 {
		return nil, ErrDesktopValidation
	}
	query.Search = strings.TrimSpace(query.Search)
	reader, ok := s.reportsReader()
	if !ok {
		return s.ReportDay(ctx, publicID, managerID, input)
	}
	var o *DesktopOrganization
	var err error
	if managerID > 0 {
		o, err = s.GetManagedOrganization(ctx, managerID)
	} else {
		o, err = s.GetOrganization(ctx, publicID)
	}
	if err != nil {
		return nil, err
	}
	if managerID > 0 && !o.ConversationSummaryEnabled {
		return nil, ErrDesktopOrgReadOnly
	}
	date, err := s.reports.date(input)
	if err != nil {
		return nil, err
	}
	day, err := reader.ReadDay(ctx, o.ID, date, query)
	if err != nil {
		return nil, err
	}
	day.AnalysisModel = o.AnalysisModel
	if day.ExpectedMembers == 0 {
		day.Status, day.Reason = "no_records", "no_records"
		day.Summary.Status = "no_records"
		return day, nil
	}
	if reason := reportConfigReason(o); reason != "" {
		day.Reason = reason
	} else if err = s.reports.check(ctx, o, nil, o.AnalysisModel); err != nil {
		day.Reason = reportUnavailableReason(err)
	} else if day.Task != nil && reportConfigurationChanged(o, day.Task) {
		day.Reason = "configuration_changed"
	} else {
		day.Reason = ""
		if day.Status == "waiting" {
			day.Status = "pending"
		}
	}
	return day, nil
}

func (s *DesktopService) reportsReader() (DesktopReportReader, bool) {
	if s.reports == nil {
		return nil, false
	}
	reader, ok := s.reports.repo.(DesktopReportReader)
	return reader, ok
}
