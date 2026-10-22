package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// The membership set, mutable ordering, assignments and usage all share one snapshot.
func (r *desktopRepository) StreamMembersWithUsage(ctx context.Context, org string, filters service.DesktopMemberListFilters, at time.Time, write func(*service.DesktopOrganization, []service.DesktopMember) error) error {
	tx, err := r.client.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	reader := *r
	reader.client = tx.Client()
	organization, err := reader.GetOrganization(ctx, org)
	if err != nil {
		return err
	}
	usageReader := &usageLogRepository{sql: tx.Client()}
	selected, err := (service.DesktopAnalyticsRangeInput{Days: 30}).Resolve(at)
	if err != nil {
		return err
	}
	for page := 1; ; page++ {
		members, metadata, err := reader.ListMembers(ctx, org, pagination.PaginationParams{Page: page, PageSize: 100}, filters)
		if err != nil {
			return err
		}
		ids := make([]int64, len(members))
		for i := range members {
			ids[i] = members[i].ID
		}
		usage, err := usageReader.GetDesktopMembersUsage(ctx, ids, timezone.StartOfDay(at).UTC(), selected.Start.UTC(), at)
		if err != nil {
			return err
		}
		for i := range members {
			members[i].Usage = usage[members[i].ID]
		}
		if err := write(organization, members); err != nil {
			return err
		}
		if len(members) == 0 || int64(page*100) >= metadata.Total {
			break
		}
	}
	return tx.Commit()
}

var _ service.DesktopMemberUsageExporter = (*desktopRepository)(nil)
