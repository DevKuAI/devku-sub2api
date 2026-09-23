package bi

import (
	"context"
	"slices"

	"github.com/lib/pq"
)

func inspectContextCoverage(ctx context.Context, q queryer, state *analysisState) error {
	teams := state.Scope.TeamIDs
	allTeams := state.Scope.AllTeams
	if len(state.Context.Filters.TeamIDs) > 0 {
		teams = state.Context.Filters.TeamIDs
		allTeams = false
	}
	rows, err := q.QueryContext(ctx, `WITH days AS (`+dayRevisionSQL+`)
		SELECT f.source_id,SUM(f.requests),BOOL_OR(f.actor_type='unknown'),BOOL_OR(f.team_id IS NULL),
		BOOL_OR(f.application_id IS NULL),BOOL_OR(f.scene_id IS NULL),BOOL_OR(f.unmeasured>0),BOOL_OR(f.unknown_outcome>0)
		FROM days JOIN bi_usage_daily f ON f.organization_id=$1 AND f.day=days.day AND f.data_revision=days.data_revision
		WHERE ($5 OR f.team_id=ANY($6)) AND ($7='' OR f.application_id=$7) AND ($8='' OR f.scene_id=$8) GROUP BY f.source_id`, state.Context.OrganizationID, state.Revision, state.Context.Range.StartDate, state.Context.Range.EndDateExclusive,
		allTeams, pq.Array(teams), state.Context.Filters.ApplicationID, state.Context.Filters.SceneID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	var total int64
	for rows.Next() {
		var source string
		var count int64
		var flags [6]bool
		if err := rows.Scan(&source, &count, &flags[0], &flags[1], &flags[2], &flags[3], &flags[4], &flags[5]); err != nil {
			return err
		}
		total += count
		for i := range state.Context.Sources {
			fresh := &state.Context.Sources[i]
			if fresh.SourceID != source {
				continue
			}
			for index, dimension := range []string{"identity", "organization", "application", "scene", "tokens", "outcome"} {
				if flags[index] && !slices.Contains(fresh.MissingDimensions, dimension) {
					fresh.MissingDimensions = append(fresh.MissingDimensions, dimension)
				}
			}
			if len(fresh.MissingDimensions) > 0 && fresh.Status == "ready" {
				fresh.Status = "partial"
				fresh.Reason = ptr("Some observed calls have unavailable dimensions")
			}
			if len(fresh.MissingDimensions) > 0 && state.Context.Status != "not_observable" {
				state.Context.Status = "partial"
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if total == 0 && state.Context.Status == "ready" {
		state.Context.Status = "empty"
	}
	return nil
}
