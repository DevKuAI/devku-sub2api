package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/desktopconversationrecord"
	"github.com/Wei-Shaw/sub2api/ent/desktopmember"
	"github.com/Wei-Shaw/sub2api/ent/desktoporganization"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

type desktopConversationRepository struct{ client *dbent.Client }

func NewDesktopConversationRepository(client *dbent.Client) service.DesktopConversationRepository {
	return &desktopConversationRepository{client: client}
}

func (r *desktopConversationRepository) Create(ctx context.Context, organizationID, memberID int64, input *service.DesktopConversationInput) (*service.DesktopConversationReceipt, error) {
	prompts, err := json.Marshal(input.Prompts)
	if err != nil {
		return nil, service.ErrDesktopConversationStorage
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, service.ErrDesktopConversationStorage
	}
	defer func() { _ = tx.Rollback() }()
	// Serialize the opt-in check with organization updates so disabling reporting
	// cannot commit before a later conversation insert using a stale auth snapshot.
	organization, err := tx.DesktopOrganization.Query().Where(desktoporganization.IDEQ(organizationID)).
		Select(desktoporganization.FieldID, desktoporganization.FieldConversationReportingEnabled).ForShare().Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, service.ErrDesktopMembershipRevoked
	}
	if err != nil {
		return nil, service.ErrDesktopConversationStorage
	}
	if !organization.ConversationReportingEnabled {
		return nil, service.ErrDesktopConversationReportingDisabled
	}

	builder := tx.DesktopConversationRecord.Create().
		SetRecordID(uuid.MustParse(input.RecordID)).SetOrganizationID(organizationID).SetMemberID(memberID).
		SetInstallationID(uuid.MustParse(input.InstallationID)).SetClient(input.Client).
		SetSourceSessionID(input.SessionID).SetNillableSourceTurnID(input.SourceTurnID).
		SetStartedAt(input.StartedAt).SetStoppedAt(input.StoppedAt).
		SetDurationMs(desktopConversationDurationMS(input.StartedAt, input.StoppedAt)).SetNillableCwd(input.CWD).
		SetPrompts(prompts).SetPromptCount(len(input.Prompts)).SetCaptureStatus(input.CaptureStatus)
	if input.Response != nil {
		body, marshalErr := json.Marshal(input.Response)
		if marshalErr != nil {
			return nil, service.ErrDesktopConversationStorage
		}
		builder.SetResponse(body)
	}
	row, err := builder.Save(ctx)
	if err != nil {
		if isUniqueConstraintViolation(err) {
			return nil, service.ErrDesktopConversationExists
		}
		return nil, service.ErrDesktopConversationStorage
	}
	if err := tx.Commit(); err != nil {
		return nil, service.ErrDesktopConversationStorage
	}
	return &service.DesktopConversationReceipt{RecordID: row.RecordID.String(), ReceivedAt: row.ReceivedAt.UTC()}, nil
}

func (r *desktopConversationRepository) query(organizationID int64, filters service.DesktopConversationFilters) *dbent.DesktopConversationRecordQuery {
	q := r.client.DesktopConversationRecord.Query().Where(desktopconversationrecord.OrganizationIDEQ(organizationID))
	if filters.MemberID != "" {
		q.Where(desktopconversationrecord.HasMemberWith(desktopmember.PublicIDEQ(filters.MemberID)))
	}
	if filters.MemberSearch != "" {
		q.Where(desktopconversationrecord.HasMemberWith(desktopmember.Or(desktopmember.NameContainsFold(filters.MemberSearch), desktopmember.PublicIDContainsFold(filters.MemberSearch))))
	}
	if filters.Client != "" {
		q.Where(desktopconversationrecord.ClientEQ(filters.Client))
	}
	if filters.CaptureStatus != "" {
		q.Where(desktopconversationrecord.CaptureStatusEQ(filters.CaptureStatus))
	}
	if filters.RecordID != "" {
		q.Where(desktopconversationrecord.RecordIDEQ(uuid.MustParse(filters.RecordID)))
	}
	if filters.SourceSessionID != "" {
		q.Where(desktopconversationrecord.SourceSessionIDEQ(filters.SourceSessionID))
	}
	if filters.InstallationID != "" {
		q.Where(desktopconversationrecord.InstallationIDEQ(uuid.MustParse(filters.InstallationID)))
	}
	if filters.ReceivedFrom != nil {
		q.Where(desktopconversationrecord.ReceivedAtGTE(*filters.ReceivedFrom))
	}
	if filters.ReceivedTo != nil {
		q.Where(desktopconversationrecord.ReceivedAtLT(*filters.ReceivedTo))
	}
	return q
}

func (r *desktopConversationRepository) List(ctx context.Context, organizationID int64, params pagination.PaginationParams, filters service.DesktopConversationFilters) ([]service.DesktopConversationMetadata, *pagination.PaginationResult, error) {
	// Organization authorization is completed before opting into historical members.
	ctx = mixins.SkipSoftDelete(ctx)
	q := r.query(organizationID, filters)
	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, service.ErrDesktopConversationStorage
	}
	columns := make([]string, 0, len(desktopconversationrecord.Columns)-2)
	for _, column := range desktopconversationrecord.Columns {
		if column != desktopconversationrecord.FieldPrompts && column != desktopconversationrecord.FieldResponse {
			columns = append(columns, column)
		}
	}
	order := dbent.Desc
	if params.SortOrder == pagination.SortOrderAsc {
		order = dbent.Asc
	}
	rows, err := q.Select(columns...).WithMember().
		Order(order(desktopconversationrecord.FieldReceivedAt), order(desktopconversationrecord.FieldID)).
		Offset(params.Offset()).Limit(params.PageSize).All(ctx)
	if err != nil {
		return nil, nil, service.ErrDesktopConversationStorage
	}
	items := make([]service.DesktopConversationMetadata, 0, len(rows))
	for _, row := range rows {
		items = append(items, desktopConversationMetadata(row))
	}
	return items, paginationResultFromTotal(int64(total), params), nil
}

func (r *desktopConversationRepository) Get(ctx context.Context, organizationID int64, recordID string) (*service.DesktopConversationDetail, error) {
	row, err := r.client.DesktopConversationRecord.Query().Where(
		desktopconversationrecord.OrganizationIDEQ(organizationID), desktopconversationrecord.RecordIDEQ(uuid.MustParse(recordID)),
	).WithMember().Only(mixins.SkipSoftDelete(ctx))
	if dbent.IsNotFound(err) {
		return nil, service.ErrDesktopConversationNotFound
	}
	if err != nil {
		return nil, service.ErrDesktopConversationStorage
	}
	detail := &service.DesktopConversationDetail{DesktopConversationMetadata: desktopConversationMetadata(row), SchemaVersion: 2}
	if json.Unmarshal(row.Prompts, &detail.Prompts) != nil {
		return nil, service.ErrDesktopConversationStorage
	}
	if len(row.Response) > 0 && json.Unmarshal(row.Response, &detail.Response) != nil {
		return nil, service.ErrDesktopConversationStorage
	}
	return detail, nil
}

func desktopConversationMetadata(row *dbent.DesktopConversationRecord) service.DesktopConversationMetadata {
	result := service.DesktopConversationMetadata{
		RecordID: row.RecordID.String(), InstallationID: row.InstallationID.String(), Client: row.Client,
		SourceSessionID: row.SourceSessionID, SourceTurnID: row.SourceTurnID, StartedAt: row.StartedAt.UTC(),
		StoppedAt: row.StoppedAt.UTC(), DurationMS: row.DurationMs, ReceivedAt: row.ReceivedAt.UTC(), CWD: row.Cwd,
		CaptureStatus: row.CaptureStatus, PromptCount: row.PromptCount,
	}
	if member := row.Edges.Member; member != nil {
		result.MemberID, result.MemberName, result.MemberDeleted = member.PublicID, member.Name, member.DeletedAt != nil
	}
	return result
}

func desktopConversationDurationMS(start, stop time.Time) int64 {
	seconds := stop.Unix() - start.Unix()
	nanoseconds := int64(stop.Nanosecond() - start.Nanosecond())
	if nanoseconds < 0 {
		seconds--
		nanoseconds += int64(time.Second)
	}
	return seconds*1000 + nanoseconds/int64(time.Millisecond)
}

// Statistics aggregates metadata only in one database snapshot, independent of list pagination.
func (r *desktopConversationRepository) Statistics(ctx context.Context, organizationID int64, filters service.DesktopConversationFilters, periods service.DesktopConversationPeriods) (*service.DesktopConversationStatistics, error) {
	ctx = mixins.SkipSoftDelete(ctx)
	var rows []struct {
		TodayRecords              int64 `json:"today_records"`
		TodayPrompts              int64 `json:"today_prompts"`
		TodayDurationCount        int64 `json:"today_duration_count"`
		TodayDurationMS           int64 `json:"today_duration_ms"`
		WeekRecords               int64 `json:"week_records"`
		WeekPrompts               int64 `json:"week_prompts"`
		WeekDurationCount         int64 `json:"week_duration_count"`
		WeekDurationMS            int64 `json:"week_duration_ms"`
		MonthRecords              int64 `json:"month_records"`
		MonthPrompts              int64 `json:"month_prompts"`
		MonthDurationCount        int64 `json:"month_duration_count"`
		MonthDurationMS           int64 `json:"month_duration_ms"`
		Last30DaysRecords         int64 `json:"last_30_days_records"`
		Last30DaysPrompts         int64 `json:"last_30_days_prompts"`
		Last30DaysDurationCount   int64 `json:"last_30_days_duration_count"`
		Last30DaysDurationMS      int64 `json:"last_30_days_duration_ms"`
		CapturedLast30Days        int64 `json:"captured_last_30_days"`
		ResponseMissingLast30Days int64 `json:"response_missing_last_30_days"`
		WorkbuddyLast30Days       int64 `json:"workbuddy_last_30_days"`
		ChatGPTCodexLast30Days    int64 `json:"chatgpt_codex_last_30_days"`
		DistinctMembers           int64 `json:"distinct_members"`
		DistinctSessions          int64 `json:"distinct_sessions"`
		Captured                  int64 `json:"captured"`
		ResponseMissing           int64 `json:"response_missing"`
		Workbuddy                 int64 `json:"workbuddy"`
		ChatGPTCodex              int64 `json:"chatgpt_codex"`
		TotalRecords              int64 `json:"total_records"`
		TotalPrompts              int64 `json:"total_prompts"`
		TotalDurationCount        int64 `json:"total_duration_count"`
		TotalDurationMS           int64 `json:"total_duration_ms"`
	}
	countsSince := func(start time.Time, prefix string) []dbent.AggregateFunc {
		// Only server-generated timestamps are formatted here; request filters use Ent predicates.
		condition := func(s *sql.Selector) string {
			return fmt.Sprintf("%s >= '%s'::timestamptz", s.C(desktopconversationrecord.FieldReceivedAt), start.UTC().Format(time.RFC3339Nano))
		}
		return []dbent.AggregateFunc{
			func(s *sql.Selector) string {
				return sql.As("COUNT(*) FILTER (WHERE "+condition(s)+")", prefix+"_records")
			},
			func(s *sql.Selector) string {
				return sql.As("COALESCE(SUM("+s.C(desktopconversationrecord.FieldPromptCount)+") FILTER (WHERE "+condition(s)+"), 0)", prefix+"_prompts")
			},
			func(s *sql.Selector) string {
				return sql.As("COUNT("+s.C(desktopconversationrecord.FieldDurationMs)+") FILTER (WHERE "+condition(s)+")", prefix+"_duration_count")
			},
			func(s *sql.Selector) string {
				return sql.As("COALESCE(SUM("+s.C(desktopconversationrecord.FieldDurationMs)+") FILTER (WHERE "+condition(s)+"), 0)::bigint", prefix+"_duration_ms")
			},
		}
	}
	aggregates := append(countsSince(periods.Today, "today"), countsSince(periods.Week, "week")...)
	aggregates = append(aggregates, countsSince(periods.Month, "month")...)
	aggregates = append(aggregates, countsSince(periods.Last30Days, "last_30_days")...)
	for _, dimension := range []struct{ column, value, alias string }{
		{desktopconversationrecord.FieldCaptureStatus, "captured", "captured_last_30_days"},
		{desktopconversationrecord.FieldCaptureStatus, "response_missing", "response_missing_last_30_days"},
		{desktopconversationrecord.FieldClient, "workbuddy", "workbuddy_last_30_days"},
		{desktopconversationrecord.FieldClient, "chatgpt_codex", "chatgpt_codex_last_30_days"},
	} {
		dimension := dimension
		aggregates = append(aggregates, func(s *sql.Selector) string {
			return sql.As(fmt.Sprintf("COUNT(*) FILTER (WHERE %s >= '%s'::timestamptz AND %s = '%s')",
				s.C(desktopconversationrecord.FieldReceivedAt), periods.Last30Days.UTC().Format(time.RFC3339Nano), s.C(dimension.column), dimension.value), dimension.alias)
		})
	}
	aggregates = append(aggregates, func(s *sql.Selector) string {
		return sql.As("COUNT(DISTINCT "+s.C(desktopconversationrecord.FieldMemberID)+")", "distinct_members")
	}, func(s *sql.Selector) string {
		return sql.As(fmt.Sprintf("COUNT(DISTINCT (%s, %s, %s, %s))",
			s.C(desktopconversationrecord.FieldMemberID), s.C(desktopconversationrecord.FieldClient),
			s.C(desktopconversationrecord.FieldInstallationID), s.C(desktopconversationrecord.FieldSourceSessionID)), "distinct_sessions")
	})
	for _, dimension := range []struct{ column, value, alias string }{
		{desktopconversationrecord.FieldCaptureStatus, "captured", "captured"},
		{desktopconversationrecord.FieldCaptureStatus, "response_missing", "response_missing"},
		{desktopconversationrecord.FieldClient, "workbuddy", "workbuddy"},
		{desktopconversationrecord.FieldClient, "chatgpt_codex", "chatgpt_codex"},
	} {
		dimension := dimension
		aggregates = append(aggregates, func(s *sql.Selector) string {
			return sql.As(fmt.Sprintf("COUNT(*) FILTER (WHERE %s = '%s')", s.C(dimension.column), dimension.value), dimension.alias)
		})
	}
	aggregates = append(aggregates, dbent.As(dbent.Count(), "total_records"), func(s *sql.Selector) string {
		return sql.As("COALESCE(SUM("+s.C(desktopconversationrecord.FieldPromptCount)+"), 0)", "total_prompts")
	}, func(s *sql.Selector) string {
		return sql.As("COUNT("+s.C(desktopconversationrecord.FieldDurationMs)+")", "total_duration_count")
	}, func(s *sql.Selector) string {
		return sql.As("COALESCE(SUM("+s.C(desktopconversationrecord.FieldDurationMs)+"), 0)::bigint", "total_duration_ms")
	})
	err := r.query(organizationID, filters).Where(desktopconversationrecord.ReceivedAtLTE(periods.AsOf)).Aggregate(aggregates...).Scan(ctx, &rows)
	if err != nil || len(rows) != 1 {
		return nil, service.ErrDesktopConversationStorage
	}
	row := rows[0]
	result := &service.DesktopConversationStatistics{
		Today:                     desktopConversationCounts(row.TodayRecords, row.TodayPrompts, row.TodayDurationCount, row.TodayDurationMS),
		Week:                      desktopConversationCounts(row.WeekRecords, row.WeekPrompts, row.WeekDurationCount, row.WeekDurationMS),
		Month:                     desktopConversationCounts(row.MonthRecords, row.MonthPrompts, row.MonthDurationCount, row.MonthDurationMS),
		Last30Days:                desktopConversationCounts(row.Last30DaysRecords, row.Last30DaysPrompts, row.Last30DaysDurationCount, row.Last30DaysDurationMS),
		CapturedLast30Days:        row.CapturedLast30Days,
		ResponseMissingLast30Days: row.ResponseMissingLast30Days,
		WorkbuddyLast30Days:       row.WorkbuddyLast30Days,
		ChatGPTCodexLast30Days:    row.ChatGPTCodexLast30Days,
		DistinctMembers:           row.DistinctMembers,
		DistinctSessions:          row.DistinctSessions,
		Captured:                  row.Captured,
		ResponseMissing:           row.ResponseMissing,
		Workbuddy:                 row.Workbuddy,
		ChatGPTCodex:              row.ChatGPTCodex,
		Total:                     desktopConversationCounts(row.TotalRecords, row.TotalPrompts, row.TotalDurationCount, row.TotalDurationMS),
		Daily:                     make([]service.DesktopConversationDay, periods.TrendDays),
	}
	dayIndex := make(map[string]int, periods.TrendDays)
	for i := range result.Daily {
		date := periods.TrendStart.In(timezone.Location()).AddDate(0, 0, i).Format("2006-01-02")
		result.Daily[i].Date = date
		dayIndex[date] = i
	}
	if periods.TrendDays == 0 {
		return result, nil
	}
	zone := strings.ReplaceAll(timezone.Location().String(), "'", "''")
	var days []struct {
		Date          string `json:"date"`
		Records       int64  `json:"records"`
		Prompts       int64  `json:"prompts"`
		DurationCount int64  `json:"duration_count"`
		DurationMS    int64  `json:"duration_ms"`
	}
	err = r.query(organizationID, filters).Where(
		desktopconversationrecord.ReceivedAtGTE(periods.TrendStart), desktopconversationrecord.ReceivedAtLTE(periods.TrendEnd),
	).Aggregate(func(s *sql.Selector) string {
		expr := fmt.Sprintf("to_char(%s AT TIME ZONE '%s', 'YYYY-MM-DD')", s.C(desktopconversationrecord.FieldReceivedAt), zone)
		s.GroupBy(expr)
		return sql.As(expr, "date")
	}, dbent.As(dbent.Count(), "records"), func(s *sql.Selector) string {
		return sql.As("COALESCE(SUM("+s.C(desktopconversationrecord.FieldPromptCount)+"), 0)", "prompts")
	}, func(s *sql.Selector) string {
		return sql.As("COUNT("+s.C(desktopconversationrecord.FieldDurationMs)+")", "duration_count")
	}, func(s *sql.Selector) string {
		return sql.As("COALESCE(SUM("+s.C(desktopconversationrecord.FieldDurationMs)+"), 0)::bigint", "duration_ms")
	}).Scan(ctx, &days)
	if err != nil {
		return nil, service.ErrDesktopConversationStorage
	}
	for _, day := range days {
		if index, ok := dayIndex[day.Date]; ok {
			result.Daily[index].DesktopConversationCounts = desktopConversationCounts(day.Records, day.Prompts, day.DurationCount, day.DurationMS)
		}
	}
	return result, nil
}

func desktopConversationCounts(records, prompts, durationCount, durationMS int64) service.DesktopConversationCounts {
	result := service.DesktopConversationCounts{RecordCount: records, PromptCount: prompts, DurationRecordCount: durationCount, TotalDurationMS: durationMS}
	if durationCount > 0 {
		average := float64(durationMS) / float64(durationCount)
		result.AverageDurationMS = &average
	}
	return result
}
