package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// GetDesktopOrganizationUsage includes historical key assignments and deleted members.
// The join is scoped by membership, not the carrier user's unrelated API keys.
func (r *usageLogRepository) GetDesktopOrganizationUsage(ctx context.Context, organizationID int64, windows service.DesktopUsageWindows) (*service.DesktopOrganizationUsageStatistics, error) {
	const query = `
  SELECT
   COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens)
    FILTER (WHERE ul.created_at >= $2), 0),
   COALESCE(SUM(ul.actual_cost) FILTER (WHERE ul.created_at >= $2), 0),
   COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens)
    FILTER (WHERE ul.created_at >= $3), 0),
   COALESCE(SUM(ul.actual_cost) FILTER (WHERE ul.created_at >= $3), 0),
   COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens)
    FILTER (WHERE ul.created_at >= $4), 0),
   COALESCE(SUM(ul.actual_cost) FILTER (WHERE ul.created_at >= $4), 0),
   COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0),
   COALESCE(SUM(ul.actual_cost), 0)
  FROM usage_logs ul WHERE ul.api_key_id IN (
 SELECT assignment.api_key_id FROM desktop_members member JOIN desktop_member_api_keys assignment ON assignment.member_id=member.id WHERE member.organization_id=$1
 UNION SELECT id FROM api_keys WHERE desktop_analysis_organization_id=$1
 ) AND ul.created_at < $5
 `
	result := &service.DesktopOrganizationUsageStatistics{}
	if err := scanSingleRow(ctx, r.sql, `SELECT COALESCE(SUM(ul.input_tokens+ul.output_tokens+ul.cache_creation_tokens+ul.cache_read_tokens),0), COALESCE(SUM(ul.actual_cost),0) FROM usage_logs ul JOIN api_keys k ON k.id=ul.api_key_id WHERE k.desktop_analysis_organization_id=$1 AND ul.created_at >= $2 AND ul.created_at < $3`, []any{organizationID, windows.Selected.Start, windows.Selected.End}, &result.Analysis.TotalTokens, &result.Analysis.ActualCost); err != nil {
		return nil, err
	}
	if err := scanSingleRow(ctx, r.sql, query, []any{organizationID, windows.Today, windows.Week, windows.Month, windows.AsOf},
		&result.Today.TotalTokens, &result.Today.ActualCost,
		&result.Week.TotalTokens, &result.Week.ActualCost,
		&result.Month.TotalTokens, &result.Month.ActualCost,
		&result.Total.TotalTokens, &result.Total.ActualCost,
	); err != nil {
		return nil, err
	}
	if err := r.loadDesktopUsageInsights(ctx, organizationID, windows.Selected, result); err != nil {
		return nil, err
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
	return result, nil
}

func (r *usageLogRepository) sumDesktopUsage(ctx context.Context, organizationID int64, start, end time.Time) (service.DesktopOrganizationUsagePeriod, error) {
	const query = `SELECT COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0),
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

	const dailyQuery = `
		SELECT to_char(ul.created_at AT TIME ZONE $4, 'YYYY-MM-DD'),
			SUM(ul.input_tokens), SUM(ul.output_tokens), SUM(ul.cache_creation_tokens), SUM(ul.cache_read_tokens), SUM(ul.actual_cost)
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
		if err = rows.Scan(&date, &input, &output, &cacheCreation, &cacheRead, &cost); err != nil {
			break
		}
		index, ok := dayIndex[date]
		if !ok {
			continue
		}
		tokens := input + output + cacheCreation + cacheRead
		result.Daily[index].TotalTokens = tokens
		result.Daily[index].ActualCost = cost
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

	const modelQuery = `
		WITH totals AS (
			SELECT COALESCE(NULLIF(ul.requested_model, ''), ul.model) AS model, COUNT(*) AS requests,
				SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens) AS total_tokens,
				SUM(ul.actual_cost) AS actual_cost
			FROM usage_logs ul WHERE ul.api_key_id IN (
 SELECT assignment.api_key_id FROM desktop_members member JOIN desktop_member_api_keys assignment ON assignment.member_id=member.id WHERE member.organization_id=$1
 UNION SELECT id FROM api_keys WHERE desktop_analysis_organization_id=$1
 ) AND ul.created_at >= $2 AND ul.created_at < $3
			GROUP BY 1
		), ranked AS (
			SELECT *, ROW_NUMBER() OVER (ORDER BY actual_cost DESC, total_tokens DESC, model) AS cost_rank,
				ROW_NUMBER() OVER (ORDER BY total_tokens DESC, actual_cost DESC, model) AS token_rank FROM totals
		)
		SELECT model, requests, total_tokens, actual_cost, cost_rank, token_rank
		FROM ranked WHERE cost_rank <= 10 OR token_rank <= 10 ORDER BY cost_rank`
	rows, err = r.sql.QueryContext(ctx, modelQuery, organizationID, start.UTC(), endTime)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item service.DesktopUsageModel
		if err = rows.Scan(&item.Model, &item.Requests, &item.TotalTokens, &item.ActualCost, &item.CostRank, &item.TokenRank); err != nil {
			break
		}
		result.Models = append(result.Models, item)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return err
	}

	const memberQuery = `
		WITH totals AS (
			SELECT member.public_id, member.name, member.deleted_at IS NOT NULL AS deleted, COUNT(*) AS requests,
				SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens) AS total_tokens,
				SUM(ul.actual_cost) AS actual_cost
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
		SELECT public_id, name, deleted, requests, total_tokens, actual_cost, cost_rank, token_rank, observed_members
		FROM ranked WHERE cost_rank <= 10 OR token_rank <= 10 ORDER BY cost_rank`
	rows, err = r.sql.QueryContext(ctx, memberQuery, organizationID, start.UTC(), endTime)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item service.DesktopUsageMember
		if err = rows.Scan(&item.MemberID, &item.Name, &item.Deleted, &item.Requests, &item.TotalTokens, &item.ActualCost, &item.CostRank, &item.TokenRank, &result.ObservedMembers); err != nil {
			break
		}
		result.Members = append(result.Members, item)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return err
	}
	result.MemberModels, err = r.loadDesktopMemberModelUsage(ctx, organizationID, selection)
	return err
}

func (r *usageLogRepository) loadDesktopMemberModelUsage(ctx context.Context, organizationID int64, selection service.DesktopAnalyticsRange) ([]service.DesktopUsageMemberModel, error) {
	const query = `
		SELECT member.public_id, member.name, member.deleted_at IS NOT NULL,
			COALESCE(NULLIF(ul.requested_model, ''), ul.model) AS model, COUNT(*),
			SUM(ul.input_tokens), SUM(ul.output_tokens), SUM(ul.cache_creation_tokens), SUM(ul.cache_read_tokens),
			SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens) AS total_tokens
		FROM desktop_members member
		JOIN desktop_member_api_keys assignment ON assignment.member_id = member.id
		JOIN usage_logs ul ON ul.api_key_id = assignment.api_key_id
		WHERE member.organization_id = $1 AND ul.created_at >= $2 AND ul.created_at < $3
		GROUP BY member.id, member.public_id, member.name, member.deleted_at, 4
		ORDER BY member.name, member.public_id, total_tokens DESC, model`
	rows, err := r.sql.QueryContext(ctx, query, organizationID, selection.Start.UTC(), selection.End)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []service.DesktopUsageMemberModel{}
	for rows.Next() {
		var item service.DesktopUsageMemberModel
		if err := rows.Scan(&item.MemberID, &item.Name, &item.Deleted, &item.Model, &item.Requests,
			&item.InputTokens, &item.OutputTokens, &item.CacheCreationTokens, &item.CacheReadTokens, &item.TotalTokens); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
