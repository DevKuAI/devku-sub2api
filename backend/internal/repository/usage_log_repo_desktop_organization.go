package repository

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// GetDesktopOrganizationUsage includes historical key assignments and deleted members.
// The join is scoped by membership, not the carrier user's unrelated API keys.
func (r *usageLogRepository) GetDesktopOrganizationUsage(ctx context.Context, organizationID int64, windows service.DesktopUsageWindows) (*service.DesktopOrganizationUsageStatistics, error) {
	return nativeSnapshot(ctx, r, func(reader *usageLogRepository) (*service.DesktopOrganizationUsageStatistics, error) {
		return reader.desktopOrganizationUsage(ctx, organizationID, windows)
	})
}
func (r *usageLogRepository) desktopOrganizationUsage(ctx context.Context, organizationID int64, windows service.DesktopUsageWindows) (*service.DesktopOrganizationUsageStatistics, error) {
	query := `
  SELECT
   COALESCE(SUM(` + desktopMeasuredTotalSQL + `)
    FILTER (WHERE ul.created_at >= $2), 0),
   COALESCE(SUM(ul.actual_cost) FILTER (WHERE ul.created_at >= $2), 0),
   COALESCE(SUM(` + desktopMeasuredTotalSQL + `)
    FILTER (WHERE ul.created_at >= $3), 0),
   COALESCE(SUM(ul.actual_cost) FILTER (WHERE ul.created_at >= $3), 0),
   COALESCE(SUM(` + desktopMeasuredTotalSQL + `)
    FILTER (WHERE ul.created_at >= $4), 0),
   COALESCE(SUM(ul.actual_cost) FILTER (WHERE ul.created_at >= $4), 0),
   COALESCE(SUM(` + desktopMeasuredTotalSQL + `), 0),
   COALESCE(SUM(ul.actual_cost), 0)
  FROM usage_logs ul WHERE ul.api_key_id IN (
 SELECT assignment.api_key_id FROM desktop_members member JOIN desktop_member_api_keys assignment ON assignment.member_id=member.id WHERE member.organization_id=$1
 UNION SELECT id FROM api_keys WHERE desktop_analysis_organization_id=$1
 ) AND ul.created_at < $5
 `
	result := &service.DesktopOrganizationUsageStatistics{Daily: []service.DesktopUsageDay{}, Models: []service.DesktopUsageModel{}, Members: []service.DesktopUsageMember{}, MemberModels: []service.DesktopUsageMemberModel{}}
	if windows.MemberModelsOnly {
		rows, page, err := r.desktopMemberModelPage(ctx, organizationID, windows.Selected, windows.Query)
		result.MemberModels = rows
		result.MemberModelsPage = page
		return result, err
	}
	if err := scanSingleRow(ctx, r.sql, `SELECT COALESCE(SUM(`+desktopMeasuredTotalSQL+`),0), COALESCE(SUM(ul.actual_cost),0) FROM usage_logs ul JOIN api_keys k ON k.id=ul.api_key_id WHERE k.desktop_analysis_organization_id=$1 AND ul.created_at >= $2 AND ul.created_at < $3`, []any{organizationID, windows.Selected.Start, windows.Selected.End}, &result.Analysis.TotalTokens, &result.Analysis.ActualCost); err != nil {
		return nil, err
	}
	measurementSQL, measurementArgs, measurementDest, applyMeasurements := desktopUsageMeasurements(windows, result)
	query = strings.Replace(query, "  FROM usage_logs ul", ","+measurementSQL+" FROM usage_logs ul", 1)
	args := append([]any{organizationID, windows.Today, windows.Week, windows.Month, windows.AsOf}, measurementArgs...)
	dest := []any{&result.Today.TotalTokens, &result.Today.ActualCost, &result.Week.TotalTokens, &result.Week.ActualCost, &result.Month.TotalTokens, &result.Month.ActualCost, &result.Total.TotalTokens, &result.Total.ActualCost}
	dest = append(dest, measurementDest...)
	if err := scanSingleRow(ctx, r.sql, query, args, dest...); err != nil {
		return nil, err
	}

	if windows.SummaryOnly {
		selected, err := r.sumDesktopUsage(ctx, organizationID, windows.Selected.Start, windows.Selected.End)
		if err != nil {
			return nil, err
		}
		result.Selected = selected
	} else if err := r.loadDesktopUsageInsights(ctx, organizationID, windows.Selected, result); err != nil {
		return nil, err
	}
	if !windows.SummaryOnly {
		rows, page, err := r.desktopMemberModelPage(ctx, organizationID, windows.Selected, windows.Query)
		if err != nil {
			return nil, err
		}
		result.MemberModels = rows
		result.MemberModelsPage = page
	}
	last30Start := timezone.StartOfDay(windows.AsOf).AddDate(0, 0, -29)
	if windows.Selected.Start.Equal(last30Start) && windows.Selected.End.Equal(windows.AsOf) {
		result.Last30Days = result.Selected
	} else {
		last30, err := r.sumDesktopUsage(ctx, organizationID, last30Start, windows.AsOf)
		if err != nil {
			return nil, err
		}
		result.Last30Days = last30
	}
	previous, err := r.sumDesktopUsage(ctx, organizationID, windows.Selected.PreviousStart, windows.Selected.PreviousEnd)
	if err != nil {
		return nil, err
	}
	result.Previous = previous
	applyMeasurements()
	return result, nil
}

func (r *usageLogRepository) sumDesktopUsage(ctx context.Context, organizationID int64, start, end time.Time) (service.DesktopOrganizationUsagePeriod, error) {
	query := `SELECT COALESCE(SUM(` + desktopMeasuredTotalSQL + `), 0),
		COALESCE(SUM(ul.actual_cost), 0)
		FROM usage_logs ul WHERE ul.api_key_id IN (
 SELECT assignment.api_key_id FROM desktop_members member JOIN desktop_member_api_keys assignment ON assignment.member_id=member.id WHERE member.organization_id=$1
 UNION SELECT id FROM api_keys WHERE desktop_analysis_organization_id=$1
 ) AND ul.created_at >= $2 AND ul.created_at < $3`
	result := service.DesktopOrganizationUsagePeriod{}
	err := scanSingleRow(ctx, r.sql, query, []any{organizationID, start, end}, &result.TotalTokens, &result.ActualCost)
	return result, err
}

func (r *usageLogRepository) loadDesktopUsageInsights(ctx context.Context, organizationID int64, selection service.DesktopAnalyticsRange, result *service.DesktopOrganizationUsageStatistics) error {
	location := timezone.Location()
	start, endTime := selection.Start, selection.End
	result.Daily = make([]service.DesktopUsageDay, selection.Days)
	dayIndex := make(map[string]int, selection.Days)
	for i := range result.Daily {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		result.Daily[i].Date = date
		dayIndex[date] = i
	}
	result.Models = []service.DesktopUsageModel{}
	result.Members = []service.DesktopUsageMember{}

	dailyQuery := `
		SELECT to_char(ul.created_at AT TIME ZONE $4, 'YYYY-MM-DD'),
			SUM(` + desktopMeasuredComponentSQL("input_tokens") + `), SUM(` + desktopMeasuredComponentSQL("output_tokens") + `), SUM(` + desktopMeasuredComponentSQL("cache_creation_tokens") + `), SUM(` + desktopMeasuredComponentSQL("cache_read_tokens") + `), SUM(ul.actual_cost),COUNT(*) FILTER(WHERE ` + nativeMeasured + `),COUNT(*) FILTER(WHERE NOT ` + nativeMeasured + `)
		FROM usage_logs ul WHERE ul.api_key_id IN (
 SELECT assignment.api_key_id FROM desktop_members member JOIN desktop_member_api_keys assignment ON assignment.member_id=member.id WHERE member.organization_id=$1
 UNION SELECT id FROM api_keys WHERE desktop_analysis_organization_id=$1
 ) AND ul.created_at >= $2 AND ul.created_at < $3
		GROUP BY 1 ORDER BY 1`
	rows, err := r.sql.QueryContext(ctx, dailyQuery, organizationID, start.UTC(), endTime, location.String())
	if err != nil {
		return err
	}
	for rows.Next() {
		var date string
		var input, output, cacheCreation, cacheRead int64
		var cost float64
		var measured, unmeasured int64
		if err = rows.Scan(&date, &input, &output, &cacheCreation, &cacheRead, &cost, &measured, &unmeasured); err != nil {
			break
		}
		index, ok := dayIndex[date]
		if !ok {
			continue
		}
		tokens := input + output + cacheCreation + cacheRead
		result.Daily[index].TotalTokens = tokens
		result.Daily[index].ActualCost = cost
		result.Daily[index].TokenStatus = desktopUsageStatus(measured, unmeasured)
		result.Daily[index].MeasuredRequests = measured
		result.Daily[index].UnmeasuredRequests = unmeasured
		result.Selected.TotalTokens += tokens
		result.Selected.ActualCost += cost
		result.Breakdown.InputTokens += input
		result.Breakdown.OutputTokens += output
		result.Breakdown.CacheCreationTokens += cacheCreation
		result.Breakdown.CacheReadTokens += cacheRead
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return err
	}

	modelQuery := `
		WITH totals AS (
			SELECT COALESCE(NULLIF(ul.requested_model, ''), ul.model) AS model, COUNT(*) AS requests,
				SUM(` + desktopMeasuredTotalSQL + `) AS total_tokens,
				SUM(ul.actual_cost) AS actual_cost,COUNT(*) FILTER(WHERE ` + nativeMeasured + `) AS measured,COUNT(*) FILTER(WHERE NOT ` + nativeMeasured + `) AS unmeasured
			FROM usage_logs ul WHERE ul.api_key_id IN (
 SELECT assignment.api_key_id FROM desktop_members member JOIN desktop_member_api_keys assignment ON assignment.member_id=member.id WHERE member.organization_id=$1
 UNION SELECT id FROM api_keys WHERE desktop_analysis_organization_id=$1
 ) AND ul.created_at >= $2 AND ul.created_at < $3
			GROUP BY 1
		), ranked AS (
			SELECT *, ROW_NUMBER() OVER (ORDER BY actual_cost DESC, total_tokens DESC, model) AS cost_rank,
				ROW_NUMBER() OVER (ORDER BY total_tokens DESC, actual_cost DESC, model) AS token_rank FROM totals
		)
		SELECT model, requests, total_tokens, actual_cost, cost_rank, token_rank,measured,unmeasured
		FROM ranked WHERE cost_rank <= 10 OR token_rank <= 10 ORDER BY cost_rank`
	rows, err = r.sql.QueryContext(ctx, modelQuery, organizationID, start.UTC(), endTime)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item service.DesktopUsageModel
		if err = rows.Scan(&item.Model, &item.Requests, &item.TotalTokens, &item.ActualCost, &item.CostRank, &item.TokenRank, &item.MeasuredRequests, &item.UnmeasuredRequests); err != nil {
			break
		}
		item.TokenStatus = desktopUsageStatus(item.MeasuredRequests, item.UnmeasuredRequests)
		result.Models = append(result.Models, item)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return err
	}

	memberQuery := `
		WITH totals AS (
			SELECT member.public_id, member.name, member.deleted_at IS NOT NULL AS deleted, COUNT(*) AS requests,
				SUM(` + desktopMeasuredTotalSQL + `) AS total_tokens,
				SUM(ul.actual_cost) AS actual_cost,COUNT(*) FILTER(WHERE ` + nativeMeasured + `) AS measured,COUNT(*) FILTER(WHERE NOT ` + nativeMeasured + `) AS unmeasured
			FROM desktop_members member
			JOIN desktop_member_api_keys assignment ON assignment.member_id = member.id
			JOIN usage_logs ul ON ul.api_key_id = assignment.api_key_id
			WHERE member.organization_id = $1 AND ul.created_at >= $2 AND ul.created_at < $3
			GROUP BY member.id, member.public_id, member.name, member.deleted_at
		), ranked AS (
			SELECT *, ROW_NUMBER() OVER (ORDER BY actual_cost DESC, total_tokens DESC, public_id) AS cost_rank,
				ROW_NUMBER() OVER (ORDER BY total_tokens DESC, actual_cost DESC, public_id) AS token_rank,
				COUNT(*) OVER () AS observed_members FROM totals
		)
		SELECT public_id, name, deleted, requests, total_tokens, actual_cost, cost_rank, token_rank, observed_members,measured,unmeasured
		FROM ranked WHERE cost_rank <= 10 OR token_rank <= 10 ORDER BY cost_rank`
	rows, err = r.sql.QueryContext(ctx, memberQuery, organizationID, start.UTC(), endTime)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item service.DesktopUsageMember
		if err = rows.Scan(&item.MemberID, &item.Name, &item.Deleted, &item.Requests, &item.TotalTokens, &item.ActualCost, &item.CostRank, &item.TokenRank, &result.ObservedMembers, &item.MeasuredRequests, &item.UnmeasuredRequests); err != nil {
			break
		}
		item.TokenStatus = desktopUsageStatus(item.MeasuredRequests, item.UnmeasuredRequests)
		result.Members = append(result.Members, item)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return err
	}
	return nil
}
