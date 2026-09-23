package bi

import "time"

type Filters struct {
	Period        string   `json:"period"`
	TeamIDs       []string `json:"team_ids"`
	ApplicationID string   `json:"application_id,omitempty"`
	SceneID       string   `json:"scene_id,omitempty"`
}

type DateRange struct {
	StartDate        string `json:"start_date"`
	EndDateExclusive string `json:"end_date_exclusive"`
	Timezone         string `json:"timezone"`
	Label            string `json:"label"`
	CompleteDays     int    `json:"complete_days"`
}

func dateRange(start, end time.Time, label string) DateRange {
	return DateRange{StartDate: start.In(shanghai).Format("2006-01-02"), EndDateExclusive: end.In(shanghai).Format("2006-01-02"), Timezone: "Asia/Shanghai", Label: label, CompleteDays: int(end.Sub(start) / (24 * time.Hour))}
}
func (r DateRange) bounds() (time.Time, time.Time) {
	start, _ := time.ParseInLocation("2006-01-02", r.StartDate, shanghai)
	end, _ := time.ParseInLocation("2006-01-02", r.EndDateExclusive, shanghai)
	return start, end
}
func dayStart(now time.Time) time.Time {
	local := now.In(shanghai)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, shanghai)
}
func weekStart(now time.Time) time.Time {
	day := dayStart(now)
	return day.AddDate(0, 0, -(int(day.Weekday())+6)%7)
}
func resolvePeriod(period string, now time.Time) (DateRange, DateRange, error) {
	today := dayStart(now)
	week := weekStart(today)
	switch period {
	case "week":
		return dateRange(week.AddDate(0, 0, -7), week, "上周"), dateRange(week.AddDate(0, 0, -14), week.AddDate(0, 0, -7), "前周"), nil
	case "current":
		return dateRange(week, today, "本周截至昨日"), dateRange(week.AddDate(0, 0, -7), today.AddDate(0, 0, -7), "上周同期"), nil
	case "month":
		month := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, shanghai)
		return dateRange(month.AddDate(0, -1, 0), month, "上月"), dateRange(month.AddDate(0, -2, 0), month.AddDate(0, -1, 0), "前月"), nil
	default:
		return DateRange{}, DateRange{}, invalid("period", "Expected week, current or month")
	}
}

type Freshness struct {
	SourceID          string     `json:"source_id"`
	Status            string     `json:"status"`
	CompleteThrough   *time.Time `json:"complete_through"`
	MissingDimensions []string   `json:"missing_dimensions"`
	Reason            *string    `json:"reason"`
}
type AnalysisContext struct {
	ID                   string      `json:"id"`
	OrganizationID       string      `json:"organization_id"`
	Filters              Filters     `json:"filters"`
	Range                DateRange   `json:"range"`
	ComparisonRange      DateRange   `json:"comparison_range"`
	AsOf                 time.Time   `json:"as_of"`
	ContentAsOf          time.Time   `json:"content_as_of"`
	DataRevision         string      `json:"data_revision"`
	MetricVersion        string      `json:"metric_version"`
	AuthorizationVersion string      `json:"authorization_version"`
	ExpiresAt            time.Time   `json:"expires_at"`
	Status               string      `json:"status"`
	Sources              []Freshness `json:"sources"`
}
type sourceState struct {
	ID               string
	Kinds            []string
	CompleteThrough  *time.Time
	HistoryStartDate *string
	BackfillComplete bool
	Applied          bool
}
type analysisState struct {
	Context   AnalysisContext
	Principal Principal
	Scope     OrganizationScope
	Revision  int64
	Sources   []sourceState
}
type Metric struct {
	Value       *float64 `json:"value"`
	Numerator   *int64   `json:"numerator"`
	Denominator *int64   `json:"denominator"`
	Status      string   `json:"status"`
	Reason      *string  `json:"reason"`
}

func ptr[T any](value T) *T { return &value }
func unavailableMetric(reason string) Metric {
	return Metric{Status: "unavailable", Reason: ptr(reason)}
}
func countMetric(count int64) Metric {
	return Metric{Value: ptr(float64(count)), Numerator: ptr(count), Status: "ready"}
}
func ratioMetric(numerator, denominator int64) Metric {
	m := Metric{Numerator: ptr(numerator), Denominator: ptr(denominator), Status: "ready"}
	if denominator == 0 {
		m.Status = "not_observable"
		m.Reason = ptr("zero_denominator")
	} else {
		m.Value = ptr(float64(numerator) / float64(denominator))
	}
	return m
}
