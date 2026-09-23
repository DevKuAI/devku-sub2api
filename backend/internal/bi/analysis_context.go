package bi

import (
	"context"
	"crypto/hmac"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

func readSourceStates(ctx context.Context, q queryer, org string) ([]sourceState, error) {
	rows, err := q.QueryContext(ctx, `SELECT source_id,allowed_kinds,complete_through,to_char(history_start_date,'YYYY-MM-DD'),initial_backfill_complete,last_applied_at IS NOT NULL
		FROM bi_connector_sources WHERE organization_id=$1 ORDER BY source_id`, org)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	sources := []sourceState{}
	for rows.Next() {
		var source sourceState
		var kinds []byte
		if err := rows.Scan(&source.ID, &kinds, &source.CompleteThrough, &source.HistoryStartDate, &source.BackfillComplete, &source.Applied); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(kinds, &source.Kinds); err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

func sourceFreshness(sources []sourceState, period DateRange) []Freshness {
	_, end := period.bounds()
	result := []Freshness{}
	for _, source := range sources {
		fresh := Freshness{SourceID: source.ID, Status: "ready", CompleteThrough: source.CompleteThrough, MissingDimensions: []string{}}
		switch {
		case !source.Applied:
			fresh.Status = "unavailable"
			fresh.Reason = ptr("Source has not published data")
		case !source.BackfillComplete || source.HistoryStartDate == nil || *source.HistoryStartDate > period.StartDate:
			fresh.Status = "partial"
			fresh.MissingDimensions = append(fresh.MissingDimensions, "history")
			fresh.Reason = ptr("Historical coverage is incomplete")
		case source.CompleteThrough == nil || source.CompleteThrough.Before(end):
			fresh.Status = "stale"
			fresh.Reason = ptr("Source completeness does not cover this period")
		}
		result = append(result, fresh)
	}
	return result
}

func coverage(sources []sourceState, period DateRange, kinds ...string) string {
	_, end := period.bounds()
	complete, available := true, true
	for _, kind := range kinds {
		found, applied := false, false
		for _, source := range sources {
			if !slices.Contains(source.Kinds, kind) {
				continue
			}
			found = true
			applied = applied || source.Applied
			if !source.Applied || !source.BackfillComplete || source.HistoryStartDate == nil || *source.HistoryStartDate > period.StartDate || source.CompleteThrough == nil || source.CompleteThrough.Before(end) {
				complete = false
			}
		}
		if !found {
			complete = false
		}
		if !applied {
			available = false
		}
	}
	if !available {
		return "unavailable"
	}
	if !complete {
		return "partial"
	}
	return "ready"
}

func contentCoverage(sources []sourceState, at time.Time, kinds ...string) string {
	complete, available := true, true
	for _, kind := range kinds {
		found, applied := false, false
		for _, source := range sources {
			if !slices.Contains(source.Kinds, kind) {
				continue
			}
			found = true
			applied = applied || source.Applied
			if !source.Applied || !source.BackfillComplete || source.CompleteThrough == nil || source.CompleteThrough.Before(at) {
				complete = false
			}
		}
		if !found {
			complete = false
		}
		if !applied {
			available = false
		}
	}
	if !available {
		return "unavailable"
	}
	if !complete {
		return "partial"
	}
	return "ready"
}

func (s *Service) CreateAnalysisContext(ctx context.Context, p Principal, org string, filters Filters) (AnalysisContext, error) {
	encoded, err := json.Marshal(filters)
	if err != nil {
		return AnalysisContext{}, err
	}
	if err := validateSchema("Filters", encoded); err != nil {
		return AnalysisContext{}, err
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	rangeNow, previous, err := resolvePeriod(filters.Period, now)
	if err != nil {
		return AnalysisContext{}, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return AnalysisContext{}, err
	}
	defer func() { _ = tx.Rollback() }()
	scope, err := authorizeOrganization(ctx, tx, p, org, "analytics:read")
	if err != nil {
		return AnalysisContext{}, err
	}
	state := analysisState{Principal: p, Scope: scope, Context: AnalysisContext{ID: s.contextID(p, org, now.Add(30*time.Minute)), OrganizationID: org, Filters: filters, Range: rangeNow,
		ComparisonRange: previous, AsOf: now, ContentAsOf: now, MetricVersion: "bi-v1", AuthorizationVersion: fmt.Sprintf("grant_%d", scope.GrantRevision),
		ExpiresAt: now.Add(30 * time.Minute), Status: "ready", DataRevision: "empty:" + org}}
	err = tx.QueryRowContext(ctx, `SELECT id,public_id FROM bi_data_revisions WHERE organization_id=$1 AND status='published' ORDER BY id DESC LIMIT 1`, org).
		Scan(&state.Revision, &state.Context.DataRevision)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return AnalysisContext{}, err
	}
	state.Sources, err = readSourceStates(ctx, tx, org)
	if err != nil {
		return AnalysisContext{}, err
	}
	if len(state.Sources) == 0 {
		return AnalysisContext{}, apiError(503, "DATA_UNAVAILABLE", "No BI data sources are configured")
	}
	state.Context.Sources = sourceFreshness(state.Sources, rangeNow)
	if coverage(state.Sources, rangeNow, "usage", "member", "membership", "eligibility") != "ready" {
		state.Context.Status = "partial"
	}
	if rangeNow.CompleteDays == 0 {
		state.Context.Status = "not_observable"
	}
	for _, id := range filters.TeamIDs {
		if _, err := visibleRecord(ctx, tx, state, "team", id); err != nil {
			return AnalysisContext{}, err
		}
	}
	for kind, id := range map[string]string{"application": filters.ApplicationID, "scene": filters.SceneID} {
		if id != "" {
			if _, err := visibleRecord(ctx, tx, state, kind, id); err != nil {
				return AnalysisContext{}, err
			}
		}
	}
	if err := inspectContextCoverage(ctx, tx, &state); err != nil {
		return AnalysisContext{}, err
	}
	snapshot, _ := json.Marshal(state.Context)
	scopeJSON, _ := json.Marshal(scope)
	sourceJSON, _ := json.Marshal(state.Sources)
	var revision any
	if state.Revision > 0 {
		revision = state.Revision
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO bi_analysis_contexts(id,manager_id,organization_id,data_revision,snapshot,scope,source_state,expires_at,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, state.Context.ID, p.ManagerID, org, revision, string(snapshot), string(scopeJSON), string(sourceJSON), state.Context.ExpiresAt, now)
	if err != nil {
		return AnalysisContext{}, err
	}
	if err := tx.Commit(); err != nil {
		return AnalysisContext{}, err
	}
	return state.Context, nil
}

func (s *Service) analysisContext(ctx context.Context, p Principal, org, id, capability string) (analysisState, error) {
	state := analysisState{Principal: p}
	var snapshot, scope, sources []byte
	var revision sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT snapshot,scope,source_state,data_revision FROM bi_analysis_contexts WHERE id=$1 AND manager_id=$2 AND organization_id=$3`, id, p.ManagerID, org).
		Scan(&snapshot, &scope, &sources, &revision)
	if errors.Is(err, sql.ErrNoRows) {
		if s.expiredContextID(p, org, id) {
			return state, apiError(410, "CONTEXT_EXPIRED", "Analysis context expired")
		}
		return state, ErrNotFound
	}
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(snapshot, &state.Context); err != nil {
		return state, err
	}
	if err := json.Unmarshal(scope, &state.Scope); err != nil {
		return state, err
	}
	if err := json.Unmarshal(sources, &state.Sources); err != nil {
		return state, err
	}
	state.Revision = revision.Int64
	if !state.Context.ExpiresAt.After(s.now()) {
		return state, apiError(410, "CONTEXT_EXPIRED", "Analysis context expired")
	}
	current, err := s.AuthorizeOrganization(ctx, p, org, "analytics:read")
	if errors.Is(err, ErrForbidden) || errors.Is(err, ErrNotFound) {
		return state, apiError(403, "CONTEXT_REVOKED", "Analysis scope was revoked")
	}
	if err != nil {
		return state, err
	}
	if !containsAll(current.Capabilities, state.Scope.Capabilities) || (!current.AllTeams && (state.Scope.AllTeams || !containsAll(current.TeamIDs, state.Scope.TeamIDs))) {
		return state, apiError(403, "CONTEXT_REVOKED", "Analysis scope was revoked")
	}
	if !slices.Contains(state.Scope.Capabilities, capability) {
		return state, ErrForbidden
	}
	return state, nil
}

func (s *Service) contextID(p Principal, org string, expires time.Time) string {
	nonce := randomToken("ctx_")
	expiry := strconv.FormatInt(expires.Unix(), 36)
	return nonce + "." + expiry + "." + keyedHash(s.identity, "analysis-context", p.ManagerID, org, nonce, expiry)
}

func (s *Service) expiredContextID(p Principal, org, id string) bool {
	parts := strings.Split(id, ".")
	if len(parts) != 3 {
		return false
	}
	expected := keyedHash(s.identity, "analysis-context", p.ManagerID, org, parts[0], parts[1])
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return false
	}
	expiry, err := strconv.ParseInt(parts[1], 36, 64)
	return err == nil && !time.Unix(expiry, 0).After(s.now())
}
