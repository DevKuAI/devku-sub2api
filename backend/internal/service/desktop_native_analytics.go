package service

import (
	"context"
	"time"
)

// DesktopNativeAnalyticsReader reuses billing facts without loading legacy dashboard arrays.
type DesktopNativeAnalyticsReader interface {
	NativeUsage(context.Context, int64, DesktopAnalyticsRange) (*DesktopNativeUsage, error)
	NativeModels(context.Context, int64, DesktopAnalyticsRange, string, int, int) ([]DesktopNativeModel, int64, error)
	NativeMembers(context.Context, int64, DesktopAnalyticsRange, string, string, int, int, bool) ([]DesktopNativeMember, int64, error)
	NativeMember(context.Context, int64, DesktopAnalyticsRange, string, bool) (*DesktopNativeMember, error)
}
type DesktopNativeTokens struct {
	Input         *string `json:"input_tokens"`
	Output        *string `json:"output_tokens"`
	CacheRead     *string `json:"cache_read_tokens"`
	CacheCreation *string `json:"cache_creation_tokens"`
	Total         *string `json:"total_tokens"`
	Measured      int64   `json:"measured_requests"`
	Unmeasured    int64   `json:"unmeasured_requests"`
	Status        string  `json:"status"`
}
type DesktopNativeRange struct {
	Start         string `json:"start_date"`
	End           string `json:"end_date"`
	Timezone      string `json:"timezone"`
	IncludesToday bool   `json:"includes_today"`
}
type DesktopNativeUsageDay struct {
	Date   string  `json:"date"`
	Tokens *string `json:"total_tokens"`
}
type DesktopNativeUsage struct {
	Range           DesktopNativeRange      `json:"range"`
	ComparisonRange DesktopNativeRange      `json:"comparison_range"`
	AsOf            time.Time               `json:"as_of"`
	Totals          DesktopNativeTokens     `json:"totals"`
	PreviousTokens  *string                 `json:"previous_total_tokens"`
	AnalysisTokens  *string                 `json:"analysis_tokens"`
	ObservedMembers int64                   `json:"observed_members"`
	Requests        int64                   `json:"requests"`
	Daily           []DesktopNativeUsageDay `json:"daily"`
}
type DesktopNativeModel struct {
	Model    string  `json:"model"`
	Tokens   *string `json:"total_tokens"`
	Requests int64   `json:"requests"`
}
type DesktopNativeMember struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	Deleted  bool    `json:"deleted"`
	Tokens   *string `json:"total_tokens"`
	Requests *int64  `json:"requests"`
}

func (s *DesktopService) NativeAnalytics(ctx context.Context, org string, days int) (DesktopNativeAnalyticsReader, *DesktopOrganization, DesktopAnalyticsRange, error) {
	o, err := s.GetOrganization(ctx, org)
	if err != nil {
		return nil, nil, DesktopAnalyticsRange{}, err
	}
	if o.Status != DesktopStatusActive {
		return nil, nil, DesktopAnalyticsRange{}, ErrDesktopOrganizationDisabled
	}
	reader, ok := s.usage.(DesktopNativeAnalyticsReader)
	if !ok {
		return nil, nil, DesktopAnalyticsRange{}, ErrDesktopUsageUnavailable
	}
	rangeValue, err := (DesktopAnalyticsRangeInput{Days: days, Timezone: "Asia/Shanghai"}).Resolve(s.now())
	return reader, o, rangeValue, err
}
func (s *DesktopService) NativeUsage(ctx context.Context, org string, days int) (*DesktopNativeUsage, error) {
	reader, o, selected, err := s.NativeAnalytics(ctx, org, days)
	if err != nil {
		return nil, err
	}
	return reader.NativeUsage(ctx, o.ID, selected)
}

func (s *DesktopService) ReadLatestReportDay(ctx context.Context, org string) (*DesktopReportDay, error) {
	o, err := s.GetOrganization(ctx, org)
	if err != nil {
		return nil, err
	}
	if s.reports == nil {
		return nil, ErrDesktopUsageUnavailable
	}
	reader, ok := s.reports.repo.(interface {
		LatestSummaryDate(context.Context, int64, string) (string, error)
	})
	if !ok {
		return nil, ErrDesktopUsageUnavailable
	}
	limit := s.now().In(s.reports.loc).AddDate(0, 0, -1).Format("2006-01-02")
	date, err := reader.LatestSummaryDate(ctx, o.ID, limit)
	if err != nil {
		return nil, err
	}
	if date == "" {
		date = limit
	}
	return s.ReadReportDay(ctx, org, 0, date, DesktopReportQuery{Section: "metadata", Page: 1, PageSize: 1, ExecutionPage: 1})
}
