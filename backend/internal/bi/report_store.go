package bi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"time"
	"unicode/utf8"
)

type storedReport struct {
	Summary              ReportSummary
	ManagerID            string
	State                analysisState
	Dependencies         []recordKey
	RequiredCapabilities []string
	Evidence             []Target
	Text                 *string
	GeneratedAt          *time.Time
}

func command(ctx context.Context, s *Service, p Principal, org, operation, key string, payload any, perform func(*sql.Tx) (any, error)) (json.RawMessage, error) {
	if !utf8.ValidString(key) || utf8.RuneCountInString(key) < 16 || utf8.RuneCountInString(key) > 128 {
		return nil, invalid("Idempotency-Key", "An idempotency key of 16–128 characters is required")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	payloadHash := tokenHash(string(encoded))
	keyHash := tokenHash(key)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	lock, _ := json.Marshal([]string{p.ManagerID, org, operation, keyHash})
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, string(lock)); err != nil {
		return nil, err
	}
	var hash string
	var result json.RawMessage
	err = tx.QueryRowContext(ctx, `SELECT payload_hash,result FROM bi_command_receipts WHERE manager_id=$1 AND organization_id=$2 AND operation=$3 AND key_hash=$4 AND expires_at>$5`, p.ManagerID, org, operation, keyHash, s.now().UTC()).Scan(&hash, &result)
	if err == nil {
		if hash != payloadHash {
			return nil, apiError(409, "IDEMPOTENCY_CONFLICT", "Idempotency key was used for different content")
		}
		return result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	value, err := perform(tx)
	if err != nil {
		return nil, err
	}
	result, err = json.Marshal(value)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO bi_command_receipts(manager_id,organization_id,operation,key_hash,payload_hash,result,expires_at)
		VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(manager_id,organization_id,operation,key_hash) DO UPDATE SET payload_hash=EXCLUDED.payload_hash,result=EXCLUDED.result,expires_at=EXCLUDED.expires_at`,
		p.ManagerID, org, operation, keyHash, payloadHash, string(result), s.now().UTC().Add(24*time.Hour))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func readStoredReport(ctx context.Context, q queryer, org, id string) (storedReport, error) {
	var report storedReport
	var snapshot, scope, sources, deps, caps, evidence []byte
	var revision sql.NullInt64
	var created, expires time.Time
	err := q.QueryRowContext(ctx, `SELECT id,organization_id,manager_id,title,status,created_at,expires_at,failure_code,snapshot,scope,source_state,data_revision,
		dependencies,required_capabilities,evidence,text,generated_at FROM bi_reports WHERE id=$1 AND organization_id=$2`, id, org).
		Scan(&report.Summary.ID, &report.Summary.OrganizationID, &report.ManagerID, &report.Summary.Title, &report.Summary.Status, &created, &expires, &report.Summary.FailureCode,
			&snapshot, &scope, &sources, &revision, &deps, &caps, &evidence, &report.Text, &report.GeneratedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return report, ErrNotFound
	}
	if err != nil {
		return report, err
	}
	report.Summary.CreatedAt = created.UTC().Format(time.RFC3339Nano)
	report.Summary.ExpiresAt = expires.UTC().Format(time.RFC3339Nano)
	report.State.Revision = revision.Int64
	for _, decode := range []struct {
		raw    []byte
		target any
	}{{snapshot, &report.State.Context}, {scope, &report.State.Scope}, {sources, &report.State.Sources}, {deps, &report.Dependencies}, {caps, &report.RequiredCapabilities}, {evidence, &report.Evidence}} {
		if err := json.Unmarshal(decode.raw, decode.target); err != nil {
			return report, err
		}
	}
	return report, nil
}

func requiredReportScope(state analysisState) OrganizationScope {
	scope := state.Scope
	if len(state.Context.Filters.TeamIDs) > 0 {
		scope.AllTeams = false
		scope.TeamIDs = append([]string{}, state.Context.Filters.TeamIDs...)
	}
	return scope
}

func scopeIncludes(current, required OrganizationScope) bool {
	return current.AllTeams || (!required.AllTeams && containsAll(current.TeamIDs, required.TeamIDs))
}

func (s *Service) reportAccess(ctx context.Context, p Principal, report storedReport, action string) (OrganizationScope, error) {
	expires, err := time.Parse(time.RFC3339Nano, report.Summary.ExpiresAt)
	if err != nil {
		return OrganizationScope{}, err
	}
	if !expires.After(s.now()) {
		return OrganizationScope{}, ErrNotFound
	}
	scope, err := s.AuthorizeOrganization(ctx, p, report.Summary.OrganizationID, action)
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrForbidden) {
		return scope, ErrNotFound
	}
	if err != nil {
		return scope, err
	}
	if !scopeIncludes(scope, report.State.Scope) || !containsAll(scope.Capabilities, report.RequiredCapabilities) {
		return scope, ErrNotFound
	}
	state := analysisState{Principal: p, Scope: scope, Revision: -1, Context: AnalysisContext{OrganizationID: report.Summary.OrganizationID, Filters: report.State.Context.Filters}}
	for _, dependency := range report.Dependencies {
		if _, err := visibleRecord(ctx, s.db, state, dependency.Kind, dependency.ID); err != nil {
			return scope, err
		}
	}
	return scope, nil
}

func (s *Service) ReadReport(ctx context.Context, p Principal, org, id, action string) (ReportDetail, error) {
	report, err := readStoredReport(ctx, s.db, org, id)
	if err != nil {
		return ReportDetail{}, err
	}
	scope, err := s.reportAccess(ctx, p, report, "reports:read")
	if err != nil {
		return ReportDetail{}, err
	}
	if !slices.Contains(scope.Capabilities, action) {
		return ReportDetail{}, ErrNotFound
	}
	ready := report.Summary.Status == "ready"
	result := ReportDetail{Report: report.Summary, Context: report.State.Context, DefinitionVersion: "bi-v1", Evidence: report.Evidence, CanCopy: ready && slices.Contains(scope.Capabilities, "reports:copy"), CanShare: ready && slices.Contains(scope.Capabilities, "reports:share")}
	if ready {
		result.Text = report.Text
	}
	return result, nil
}

func (s *Service) CreateReport(ctx context.Context, p Principal, org, key, contextID string, title *string) (ReportSummary, error) {
	if _, err := s.AuthorizeOrganization(ctx, p, org, "reports:create"); err != nil {
		return ReportSummary{}, err
	}
	input := struct {
		ContextID string  `json:"context_id"`
		Title     *string `json:"title,omitempty"`
	}{contextID, title}
	encoded, _ := json.Marshal(input)
	if err := validateSchema("CreateReport", encoded); err != nil {
		return ReportSummary{}, err
	}
	raw, err := command(ctx, s, p, org, "createReport", key, input, func(tx *sql.Tx) (any, error) {
		state, err := s.analysisContext(ctx, p, org, contextID, "reports:create")
		if err != nil {
			return nil, err
		}
		now := s.now().UTC().Truncate(time.Microsecond)
		months := s.config.ReportRetentionMonths
		if months == 0 {
			months = 24
		}
		name := state.Context.Range.StartDate + " 管理简报"
		if title != nil {
			name = *title
		}
		summary := ReportSummary{ID: randomToken("report_"), OrganizationID: org, Title: name, Status: "queued", CreatedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.AddDate(0, months, 0).Format(time.RFC3339Nano)}
		state.Scope = requiredReportScope(state)
		if !state.Scope.AllTeams && len(state.Context.Filters.TeamIDs) == 0 {
			state.Context.Filters.TeamIDs = append([]string{}, state.Scope.TeamIDs...)
		}
		snapshot, _ := json.Marshal(state.Context)
		scope, _ := json.Marshal(state.Scope)
		sources, _ := json.Marshal(state.Sources)
		var revision any
		if state.Revision > 0 {
			revision = state.Revision
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO bi_reports(id,organization_id,manager_id,title,status,created_at,expires_at,snapshot,scope,source_state,data_revision)
			VALUES($1,$2,$3,$4,'queued',$5,$6,$7,$8,$9,$10)`, summary.ID, org, p.ManagerID, name, now, now.AddDate(0, months, 0), string(snapshot), string(scope), string(sources), revision)
		return summary, err
	})
	if err != nil {
		return ReportSummary{}, err
	}
	var result ReportSummary
	if err := json.Unmarshal(raw, &result); err != nil {
		return result, err
	}
	report, err := readStoredReport(ctx, s.db, org, result.ID)
	if err != nil {
		return result, err
	}
	_, err = s.reportAccess(ctx, p, report, "reports:create")
	return result, err
}
