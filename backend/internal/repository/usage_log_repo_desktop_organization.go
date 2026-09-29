package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// GetDesktopOrganizationUsage includes historical key assignments and deleted members.
// The join is scoped by membership, not the carrier user's unrelated API keys.
func (r *usageLogRepository) GetDesktopOrganizationUsage(ctx context.Context, organizationID int64, todayStart, weekStart, monthStart, endTime time.Time) (*service.DesktopOrganizationUsageStatistics, error) {
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
  FROM desktop_members member
  JOIN desktop_member_api_keys assignment ON assignment.member_id = member.id
  JOIN usage_logs ul ON ul.api_key_id = assignment.api_key_id
  WHERE member.organization_id = $1 AND ul.created_at < $5
 `
	result := &service.DesktopOrganizationUsageStatistics{}
	if err := scanSingleRow(ctx, r.sql, query, []any{organizationID, todayStart, weekStart, monthStart, endTime},
		&result.Today.TotalTokens, &result.Today.ActualCost,
		&result.Week.TotalTokens, &result.Week.ActualCost,
		&result.Month.TotalTokens, &result.Month.ActualCost,
		&result.Total.TotalTokens, &result.Total.ActualCost,
	); err != nil {
		return nil, err
	}
	if err := r.loadDesktopUsageInsights(ctx, organizationID, endTime, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *usageLogRepository) loadDesktopUsageInsights(ctx context.Context, organizationID int64, endTime time.Time, result *service.DesktopOrganizationUsageStatistics) error {
	location := timezone.Location()
	start := timezone.StartOfDay(endTime).AddDate(0, 0, -29)
	result.Daily = make([]service.DesktopUsageDay, 30)
	dayIndex := make(map[string]int, 30)
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
		FROM desktop_members member
		JOIN desktop_member_api_keys assignment ON assignment.member_id = member.id
		JOIN usage_logs ul ON ul.api_key_id = assignment.api_key_id
		WHERE member.organization_id = $1 AND ul.created_at >= $2 AND ul.created_at < $3
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
		result.Last30Days.TotalTokens += tokens
		result.Last30Days.ActualCost += cost
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
		SELECT COALESCE(NULLIF(ul.requested_model, ''), ul.model), COUNT(*),
			SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), SUM(ul.actual_cost)
		FROM desktop_members member
		JOIN desktop_member_api_keys assignment ON assignment.member_id = member.id
		JOIN usage_logs ul ON ul.api_key_id = assignment.api_key_id
		WHERE member.organization_id = $1 AND ul.created_at >= $2 AND ul.created_at < $3
		GROUP BY 1 ORDER BY 4 DESC, 3 DESC, 1 LIMIT 10`
	rows, err = r.sql.QueryContext(ctx, modelQuery, organizationID, start.UTC(), endTime)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item service.DesktopUsageModel
		if err = rows.Scan(&item.Model, &item.Requests, &item.TotalTokens, &item.ActualCost); err != nil {
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
		SELECT member.public_id, member.name, member.deleted_at IS NOT NULL, COUNT(*),
			SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens),
			SUM(ul.actual_cost), COUNT(*) OVER ()
		FROM desktop_members member
		JOIN desktop_member_api_keys assignment ON assignment.member_id = member.id
		JOIN usage_logs ul ON ul.api_key_id = assignment.api_key_id
		WHERE member.organization_id = $1 AND ul.created_at >= $2 AND ul.created_at < $3
		GROUP BY member.id, member.public_id, member.name, member.deleted_at
		ORDER BY 6 DESC, 5 DESC, member.public_id LIMIT 10`
	rows, err = r.sql.QueryContext(ctx, memberQuery, organizationID, start.UTC(), endTime)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item service.DesktopUsageMember
		if err = rows.Scan(&item.MemberID, &item.Name, &item.Deleted, &item.Requests, &item.TotalTokens, &item.ActualCost, &result.ObservedMembers); err != nil {
			break
		}
		result.Members = append(result.Members, item)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	return err
}
