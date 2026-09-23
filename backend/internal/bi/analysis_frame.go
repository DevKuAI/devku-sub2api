package bi

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"time"
)

type usageFact struct {
	UsageRecord
	Normalized NormalizedTokens
	SourceID   string
}

type analysisFrame struct {
	state             analysisState
	service           *Service
	ctx               context.Context
	records           map[string]map[string]*record
	facts             []usageFact
	factsLoaded       bool
	periodCache       []usageFact
	periodLoaded      bool
	organizationStart time.Time
	visibility        map[recordKey]bool
	statisticsCache   map[analysisSelection]metricResult
	memberships       map[string][]*record
	eligibilities     map[string][]*record
	ratingValues      map[string]string
	knowledgeHistory  map[string][]knowledgeStatus
}

func (s *Service) loadMetadata(ctx context.Context, state analysisState) (*analysisFrame, error) {
	f := &analysisFrame{state: state, service: s, ctx: ctx, records: map[string]map[string]*record{}, visibility: map[recordKey]bool{}}
	kinds := []string{"team", "role", "member", "membership", "eligibility", "application", "application_version", "scene", "rating", "knowledge", "knowledge_version", "reference", "evaluation", "case"}
	all, err := recordsAt(ctx, s.db, state.Context.OrganizationID, kinds, state.Revision)
	if err != nil {
		return nil, err
	}
	for _, r := range all {
		if f.records[r.Kind] == nil {
			f.records[r.Kind] = map[string]*record{}
		}
		f.records[r.Kind][r.ID] = r
	}
	err = s.db.QueryRowContext(ctx, `SELECT d.created_at FROM bi_organizations o JOIN desktop_organizations d ON d.id=o.desktop_organization_id WHERE o.id=$1`, state.Context.OrganizationID).Scan(&f.organizationStart)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (s *Service) loadAnalysis(ctx context.Context, state analysisState) (*analysisFrame, error) {
	f, err := s.loadMetadata(ctx, state)
	if err != nil {
		return nil, err
	}
	f.facts, err = f.readAnalysisFacts()
	if err != nil {
		return nil, err
	}
	f.factsLoaded = true
	return f, nil
}

const usageFactColumns = `f.entity_id,f.occurred_at,f.actor_type,f.member_id,f.team_id,f.application_id,f.application_version_id,f.scene_id,f.requested_model,
		f.outcome,f.duration_ms,f.input_tokens::text,f.output_tokens::text,f.cache_read_tokens::text,f.cache_write_tokens::text,e.source_id`

const latestUsageFactsSQL = `SELECT DISTINCT ON (u.entity_id) u.* FROM bi_usage_facts u JOIN bi_data_revisions d ON d.id=u.data_revision
	WHERE u.organization_id=$1 AND u.data_revision<=$2 AND d.status='published' ORDER BY u.entity_id,u.data_revision DESC,u.entity_revision DESC`

// A materialized candidates CTE plus indexed per-ID lookup avoids quadratic
// semi-join plans when import volume has outgrown PostgreSQL's table statistics.
// $1 is the organization and $2 is the frozen data revision.
const latestCandidateUsageSQL = `SELECT version.* FROM candidates candidate CROSS JOIN LATERAL (
	SELECT u.* FROM bi_usage_facts u JOIN bi_data_revisions d ON d.id=u.data_revision
	WHERE u.organization_id=$1 AND u.entity_id=candidate.entity_id AND u.data_revision<=$2 AND d.status='published'
	ORDER BY u.data_revision DESC,u.entity_revision DESC LIMIT 1
) version`

func usageFactsQuery(bounded bool) string {
	prefix := ""
	latest := latestUsageFactsSQL
	if bounded {
		// Select IDs by time, then resolve all versions before checking time again.
		prefix = `WITH candidates AS MATERIALIZED (SELECT DISTINCT entity_id FROM bi_usage_facts
			WHERE organization_id=$1 AND data_revision<=$2 AND occurred_at>=$3 AND occurred_at<$4) `
		latest = latestCandidateUsageSQL
	}
	return prefix + `SELECT ` + usageFactColumns + ` FROM (` + latest + `) f
		JOIN bi_entities e ON e.organization_id=f.organization_id AND e.kind='usage' AND e.id=f.entity_id WHERE NOT f.tombstone
		AND ($3::timestamptz IS NULL OR f.occurred_at>=$3) AND ($4::timestamptz IS NULL OR f.occurred_at<$4)`
}

func (s *Service) readUsageFacts(ctx context.Context, state analysisState, start, end *time.Time) ([]usageFact, error) {
	rows, err := s.db.QueryContext(ctx, usageFactsQuery(start != nil && end != nil), state.Context.OrganizationID, state.Revision, start, end)
	if err != nil {
		return nil, err
	}
	return scanUsageFacts(rows)
}

func scanUsageFacts(rows *sql.Rows) ([]usageFact, error) {
	defer func() { _ = rows.Close() }()
	facts := []usageFact{}
	for rows.Next() {
		var u usageFact
		if err := rows.Scan(&u.ID, &u.OccurredAt, &u.ActorType, &u.MemberID, &u.TeamID, &u.ApplicationID, &u.ApplicationVersionID, &u.SceneID, &u.RequestedModel, &u.Outcome, &u.DurationMS,
			&u.Normalized.Input, &u.Normalized.Output, &u.Normalized.CacheRead, &u.Normalized.CacheWrite, &u.SourceID); err != nil {
			return nil, err
		}
		facts = append(facts, u)
	}
	return facts, rows.Err()
}

func (f *analysisFrame) periodFacts(selection analysisSelection) ([]usageFact, error) {
	if !f.periodLoaded {
		if f.factsLoaded {
			f.periodCache = f.facts
		} else {
			start, end := f.state.Context.Range.bounds()
			var err error
			f.periodCache, err = f.service.readUsageFacts(f.ctx, f.state, &start, &end)
			if err != nil {
				return nil, err
			}
		}
		f.periodLoaded = true
	}
	result := []usageFact{}
	for _, usage := range f.periodCache {
		if eventWithin(usage, f.state.Context.Range) && f.matchesUsage(usage, selection, true) {
			result = append(result, usage)
		}
	}
	return result, nil
}

func (f *analysisFrame) visible(kind, id string) (bool, error) {
	if id == "" {
		return false, nil
	}
	if value, ok := f.visibility[recordKey{kind, id}]; ok {
		return value, nil
	}
	if kind == "role" && (!f.state.Scope.AllTeams || len(f.state.Context.Filters.TeamIDs) > 0) {
		linked := false
		for _, membership := range f.records["membership"] {
			if membership.str("role_id") == id && f.state.teamAllowed(membership.str("team_id")) {
				linked = true
				break
			}
		}
		if !linked {
			f.visibility[recordKey{kind, id}] = false
			return false, nil
		}
	}
	_, err := visibleRecord(f.ctx, f.service.db, f.state, kind, id)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return false, err
	}
	allowed := err == nil
	f.visibility[recordKey{kind, id}] = allowed
	return allowed, nil
}

func (f *analysisFrame) summary(kind, id string) *EntitySummary {
	r := f.records[kind][id]
	if r == nil {
		return nil
	}
	status := r.str("status")
	if status != "active" && status != "disabled" {
		status = "unknown"
	}
	return &EntitySummary{ID: id, Name: r.str("name"), Status: status}
}

func (f *analysisFrame) safeSummary(kind, id string) (*EntitySummary, error) {
	if id == "" {
		return nil, nil
	}
	allowed, err := f.visible(kind, id)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, nil
	}
	return f.summary(kind, id), nil
}

func inInterval(r *record, at time.Time) bool {
	start, end := r.instant("valid_from"), r.instant("valid_to")
	return start != nil && !at.Before(*start) && (end == nil || at.Before(*end))
}

func (f *analysisFrame) membership(memberID string, at time.Time) *record {
	f.indexDirectory()
	for _, r := range f.memberships[memberID] {
		if r.str("member_id") == memberID && r.boolean("primary") && inInterval(r, at) {
			return r
		}
	}
	return nil
}

func (f *analysisFrame) indexDirectory() {
	if f.memberships != nil {
		return
	}
	f.memberships = map[string][]*record{}
	f.eligibilities = map[string][]*record{}
	for _, r := range f.records["membership"] {
		id := r.str("member_id")
		f.memberships[id] = append(f.memberships[id], r)
	}
	for _, r := range f.records["eligibility"] {
		id := r.str("member_id")
		f.eligibilities[id] = append(f.eligibilities[id], r)
	}
}

type analysisSelection struct{ TeamID, RoleID, MemberID, ApplicationID, SceneID string }

func (f *analysisFrame) filters(selection analysisSelection) Filters {
	filters := f.state.Context.Filters
	filters.TeamIDs = append([]string{}, filters.TeamIDs...)
	if selection.TeamID != "" {
		filters.TeamIDs = []string{selection.TeamID}
	}
	if selection.ApplicationID != "" {
		filters.ApplicationID = selection.ApplicationID
	}
	if selection.SceneID != "" {
		filters.SceneID = selection.SceneID
	}
	return filters
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (f *analysisFrame) matchesUsage(u usageFact, sel analysisSelection, teams bool) bool {
	filters := f.filters(sel)
	if filters.ApplicationID != "" && stringValue(u.ApplicationID) != filters.ApplicationID {
		return false
	}
	if filters.SceneID != "" && stringValue(u.SceneID) != filters.SceneID {
		return false
	}
	if sel.MemberID != "" && stringValue(u.MemberID) != sel.MemberID {
		return false
	}
	if teams && (!f.state.teamAllowed(stringValue(u.TeamID)) || (sel.TeamID != "" && stringValue(u.TeamID) != sel.TeamID)) {
		return false
	}
	if sel.RoleID != "" {
		member := f.membership(stringValue(u.MemberID), u.OccurredAt)
		if member == nil || member.str("role_id") != sel.RoleID {
			return false
		}
	}
	return true
}

func (f *analysisFrame) cohort(period DateRange, sel analysisSelection) map[string]*record {
	f.indexDirectory()
	_, end := period.bounds()
	at := end.Add(-time.Nanosecond)
	filters := f.filters(sel)
	result := map[string]*record{}
	for id, member := range f.records["member"] {
		if sel.MemberID != "" && id != sel.MemberID {
			continue
		}
		start, stop := member.instant("enabled_at"), member.instant("disabled_at")
		if start == nil || at.Before(*start) || (stop != nil && !at.Before(*stop)) || (member.str("status") == "disabled" && stop == nil) {
			continue
		}
		membership := f.membership(id, at)
		if membership == nil || !f.state.teamAllowed(membership.str("team_id")) {
			continue
		}
		if sel.TeamID != "" && membership.str("team_id") != sel.TeamID {
			continue
		}
		if sel.RoleID != "" && membership.str("role_id") != sel.RoleID {
			continue
		}
		for _, eligible := range f.eligibilities[id] {
			if eligible.str("member_id") != id || !inInterval(eligible, at) {
				continue
			}
			if app := eligible.str("application_id"); filters.ApplicationID != "" && app != "" && app != filters.ApplicationID {
				continue
			}
			if scene := eligible.str("scene_id"); filters.SceneID != "" && scene != "" && scene != filters.SceneID {
				continue
			}
			result[id] = member
			break
		}
	}
	return result
}

func (f *analysisFrame) ratings() map[string]string {
	if f.ratingValues != nil {
		return f.ratingValues
	}
	result := map[string]string{}
	for _, r := range f.records["rating"] {
		result[r.str("usage_event_id")] = r.str("value")
	}
	f.ratingValues = result
	return result
}

func (f *analysisFrame) historyComplete(end time.Time, kinds ...string) bool {
	rangeAll := dateRange(dayStart(f.organizationStart), end, "")
	return coverage(f.state.Sources, rangeAll, kinds...) == "ready"
}

func (f *analysisFrame) checkSelection(sel analysisSelection) error {
	base := f.state.Context.Filters
	if sel.ApplicationID != "" && base.ApplicationID != "" && sel.ApplicationID != base.ApplicationID {
		return ErrNotFound
	}
	if sel.SceneID != "" && base.SceneID != "" && sel.SceneID != base.SceneID {
		return ErrNotFound
	}
	for kind, id := range map[string]string{"team": sel.TeamID, "role": sel.RoleID, "application": sel.ApplicationID, "scene": sel.SceneID} {
		if id != "" {
			allowed, err := f.visible(kind, id)
			if err != nil {
				return err
			}
			if !allowed {
				return ErrNotFound
			}
		}
	}
	return nil
}

func recordListContains(r *record, field, id string) bool {
	return slices.Contains(r.strings(field), id)
}
