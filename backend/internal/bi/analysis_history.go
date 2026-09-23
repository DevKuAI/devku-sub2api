package bi

import (
	"encoding/json"
	"time"
)

type membershipChangeDay struct {
	MemberID string `json:"member_id"`
	Day      string `json:"day"`
}

func (f *analysisFrame) historicalMembershipChanges(before time.Time) []membershipChangeDay {
	seen := map[membershipChangeDay]bool{}
	for _, membership := range f.records["membership"] {
		if !membership.boolean("primary") {
			continue
		}
		for _, field := range []string{"valid_from", "valid_to"} {
			at := membership.instant(field)
			if at != nil && at.Before(before) {
				seen[membershipChangeDay{MemberID: membership.str("member_id"), Day: at.In(shanghai).Format("2006-01-02")}] = true
			}
		}
	}
	result := make([]membershipChangeDay, 0, len(seen))
	for change := range seen {
		result = append(result, change)
	}
	return result
}

// Older facts are needed only for human activity and first-use history. Keep
// one earliest event per member/day/scope/outcome, while retaining all events
// on membership-change days so intra-day role attribution stays exact.
// Current and comparison periods always retain full call and Token detail.
func (f *analysisFrame) readAnalysisFacts() ([]usageFact, error) {
	start, end := f.state.Context.Range.bounds()
	previousStart, previousEnd := f.state.Context.ComparisonRange.bounds()
	if previousStart.Before(start) {
		start = previousStart
	}
	if previousEnd.After(end) {
		end = previousEnd
	}
	changes, err := json.Marshal(f.historicalMembershipChanges(start))
	if err != nil {
		return nil, err
	}
	// Resolve latest versions before reducing history. Filtering versions first
	// could resurrect a corrected or retracted event in an old activity bucket.
	rows, err := f.service.db.QueryContext(f.ctx, `WITH latest AS MATERIALIZED (`+latestUsageFactsSQL+`), changes AS (
		SELECT * FROM jsonb_to_recordset($5::jsonb) AS change(member_id text,day date)
	), history AS (
		SELECT DISTINCT ON (u.member_id,u.team_id,u.application_id,u.scene_id,u.outcome,
		(u.occurred_at AT TIME ZONE 'Asia/Shanghai')::date,CASE WHEN change.member_id IS NOT NULL THEN u.entity_id ELSE '' END) u.*
		FROM latest u LEFT JOIN changes change ON change.member_id=u.member_id AND change.day=(u.occurred_at AT TIME ZONE 'Asia/Shanghai')::date
		WHERE NOT u.tombstone AND u.occurred_at<$3 AND u.actor_type='human'
		ORDER BY u.member_id,u.team_id,u.application_id,u.scene_id,u.outcome,
		(u.occurred_at AT TIME ZONE 'Asia/Shanghai')::date,CASE WHEN change.member_id IS NOT NULL THEN u.entity_id ELSE '' END,u.occurred_at,u.entity_id COLLATE "C"
	)
		SELECT `+usageFactColumns+`
		FROM (SELECT * FROM latest WHERE NOT tombstone AND occurred_at>=$3 AND occurred_at<$4
		UNION ALL SELECT * FROM history) f
		JOIN bi_entities e ON e.organization_id=f.organization_id AND e.kind='usage' AND e.id=f.entity_id`, f.state.Context.OrganizationID, f.state.Revision, start, end, string(changes))
	if err != nil {
		return nil, err
	}
	return scanUsageFacts(rows)
}
