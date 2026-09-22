package repository

import (
	"context"
	"time"

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
	return result, nil
}
