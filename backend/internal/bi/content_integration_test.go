//go:build integration

package bi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func completeBatch(t *testing.T, expected, target string, records []any) []byte {
	t.Helper()
	var value map[string]any
	require.NoError(t, json.Unmarshal(importPayload(&expected, target, records), &value))
	value["complete_through"] = "2026-09-23T08:00:00Z"
	value["history_start_date"] = "2026-08-01"
	value["initial_backfill_complete"] = true
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return raw
}

func contentFixture(t *testing.T) (*Service, Connector, Principal, AnalysisContext, map[string]any) {
	t.Helper()
	s, c, p, _ := analysisFixture(t)
	acl := map[string]any{"organization_readable": true, "team_ids": []string{}, "manager_ids": []string{}}
	source := map[string]any{"id": "test:source", "version_id": "test:source:v1", "title": "Original source title", "type": "document", "updated_at": "2026-08-01T00:00:00Z", "body": []string{"Private source text"}, "acl": acl}
	knowledge := map[string]any{"id": "test:knowledge", "title": "Approved knowledge", "summary": "Approved summary", "status": "valid", "current_version_id": "test:knowledge:v1", "owner_id": nil, "owner_name": nil, "created_at": "2026-08-01T00:00:00Z", "updated_at": "2026-08-01T00:00:00Z", "status_effective_at": "2026-08-01T00:00:00Z", "application_ids": []string{"test:app"}, "scene_ids": []string{"test:scene"}, "acl": acl}
	version := map[string]any{"id": "test:knowledge:v1", "knowledge_id": "test:knowledge", "version": "1", "created_at": "2026-08-01T00:00:00Z", "valid_from": "2026-08-01T00:00:00Z", "valid_to": nil, "change_summary": "initial", "applicability": "all", "body": []string{"Approved knowledge text"}, "source_ids": []string{"test:source"}}
	workCase := map[string]any{"id": "test:case", "title": "Case one", "occurred_at": "2026-09-15T04:00:00Z", "team_id": "test:a", "project": "Project", "customer": "Customer", "problem": "Customer issue", "steps": []string{"Use approved knowledge"}, "result": "Resolved", "outcome": "resolved", "application_id": "test:app", "scene_id": "test:scene", "knowledge_version_ids": []string{"test:knowledge:v1"}, "source_ids": []string{"test:source"}, "acl": acl}
	samples := []any{
		map[string]any{"id": "test:sample:1", "question": "Private question from case", "expected": "Answer", "baseline": "passed", "candidate": "passed", "case_link": map[string]any{"id": "test:case", "label": "Case one", "availability": "available"}},
		map[string]any{"id": "test:sample:2", "question": "Another private question", "expected": "Answer", "baseline": "failed", "candidate": "passed", "case_link": map[string]any{"id": "test:case", "label": "Case one", "availability": "available"}},
		map[string]any{"id": "test:sample:3", "question": "Generic benchmark", "expected": "Answer", "baseline": "failed", "candidate": "not_run", "case_link": map[string]any{"id": nil, "label": "No case", "availability": "deleted"}},
	}
	evaluation := map[string]any{"id": "test:evaluation", "title": "Evaluation", "application_id": "test:app", "dataset_version": "test:dataset", "criterion_version": "test:criterion", "criterion": "Correct answer", "evaluated_at": "2026-09-20T00:00:00Z", "baseline_application_version_id": "test:app:v1", "candidate_application_version_id": "test:app:v2", "samples": samples, "acl": acl}
	records := []any{analyticRecord("source", source), analyticRecord("knowledge", knowledge), analyticRecord("knowledge_version", version), analyticRecord("case", workCase),
		analyticRecord("application_version", map[string]any{"id": "test:app:v2", "application_id": "test:app", "version": "2", "released_at": "2026-09-01T00:00:00Z", "knowledge_version_ids": []string{"test:knowledge:v1"}}),
		analyticRecord("evaluation", evaluation), analyticRecord("reference", map[string]any{"id": "test:reference:1", "usage_event_id": "test:one:1", "knowledge_version_id": "test:knowledge:v1"}),
		analyticRecord("reference", map[string]any{"id": "test:reference:duplicate", "usage_event_id": "test:one:1", "knowledge_version_id": "test:knowledge:v1"})}
	job := applyImport(t, s, c, completeBatch(t, "analytics-seed", "content-seed", records))
	require.Equal(t, "applied", job.Status, job.Errors)
	snapshot, err := s.CreateAnalysisContext(context.Background(), p, c.OrganizationID, Filters{Period: "week", TeamIDs: []string{"test:a"}})
	require.NoError(t, err)
	return s, c, p, snapshot, source
}

func contentFrame(t *testing.T, s *Service, c Connector, p Principal, snapshot AnalysisContext) *analysisFrame {
	t.Helper()
	state, err := s.analysisContext(context.Background(), p, c.OrganizationID, snapshot.ID, "knowledge:read")
	require.NoError(t, err)
	f, err := s.loadAnalysis(context.Background(), state)
	require.NoError(t, err)
	return f
}

func TestContentContractsAndActualReferenceDeduplication(t *testing.T) {
	s, c, p, snapshot, _ := contentFixture(t)
	f := contentFrame(t, s, c, p, snapshot)
	knowledge, err := f.knowledgeDetail("test:knowledge")
	require.NoError(t, err)
	require.EqualValues(t, 1, *knowledge.Usage.References)
	references, err := f.knowledgeEvidence("test:knowledge")
	require.NoError(t, err)
	require.Len(t, references, 1)
	workCase, err := f.caseDetail("test:case")
	require.NoError(t, err)
	require.Equal(t, "test:knowledge", workCase.KnowledgeVersions[0].KnowledgeID)
	require.Equal(t, "test:knowledge:v1", workCase.KnowledgeVersions[0].VersionID)
	source, err := f.sourceDetail("knowledge_version", "test:knowledge:v1", "test:source")
	require.NoError(t, err)
	require.Equal(t, []string{"Private source text"}, source.Body)
	evaluation, err := f.evaluationDetail("test:evaluation")
	require.NoError(t, err)
	require.EqualValues(t, 1, evaluation.Baseline.Passed)
	require.EqualValues(t, 2, evaluation.Candidate.Passed)
	require.EqualValues(t, 1, evaluation.Candidate.NotRun)
	for schema, value := range map[string]any{"KnowledgeDetail": knowledge, "CaseDetail": workCase, "SourceDetail": source, "EvaluationDetail": evaluation} {
		require.NoError(t, schemaOutput(schema, value), schema)
	}
	hits, err := f.searchContent("Private source text", "all")
	require.NoError(t, err)
	require.Empty(t, hits)
	_, err = f.sourceDetail("case", "test:not-a-parent", "test:source")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestContentACLIsIndependentOfTheFrozenRevision(t *testing.T) {
	s, c, p, snapshot, source := contentFixture(t)
	ctx := context.Background()
	source["acl"] = map[string]any{"organization_readable": false, "team_ids": []string{}, "manager_ids": []string{}}
	raw := completeBatch(t, "content-seed", "source-restriction", []any{map[string]any{"kind": "source", "revision": 2, "payload": source}})
	job, err := s.SubmitImport(ctx, c, randomToken("idem_"), raw)
	require.NoError(t, err)
	w, err := s.claimImport(ctx)
	require.NoError(t, err)
	require.Equal(t, job.ID, w.ID)
	_, problems, err := s.stageImport(ctx, w)
	require.NoError(t, err)
	require.Empty(t, problems)
	f := contentFrame(t, s, c, p, snapshot)
	_, err = f.sourceDetail("knowledge_version", "test:knowledge:v1", "test:source")
	require.ErrorIs(t, err, ErrNotFound)
	version, err := f.knowledgeVersion("test:knowledge", "test:knowledge:v1")
	require.NoError(t, err)
	require.Nil(t, version.Sources[0].ID)
	require.Equal(t, "来源不可访问", version.Sources[0].Label)
	rawVersion, _ := json.Marshal(version)
	require.NotContains(t, string(rawVersion), "Private source text")
	require.NotContains(t, string(rawVersion), "Original source title")
	_, samples, err := f.evaluationSamples("test:evaluation")
	require.NoError(t, err)
	require.Len(t, samples, 1)
	require.Equal(t, "Generic benchmark", samples[0].Question)
	require.NoError(t, s.processImport(ctx, w))
	// A readable source still cannot be fetched through an unreadable parent.
	source["acl"] = map[string]any{"organization_readable": true, "team_ids": []string{}, "manager_ids": []string{}}
	knowledge, err := recordAt(ctx, s.db, c.OrganizationID, "knowledge", "test:knowledge", -1)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(knowledge.Raw, &payload))
	payload["acl"] = map[string]any{"organization_readable": false, "team_ids": []string{}, "manager_ids": []string{}}
	job = applyImport(t, s, c, completeBatch(t, "source-restriction", "parent-restriction", []any{map[string]any{"kind": "source", "revision": 3, "payload": source}, map[string]any{"kind": "knowledge", "revision": 2, "payload": payload}}))
	require.Equal(t, "applied", job.Status)
	f = contentFrame(t, s, c, p, snapshot)
	_, err = f.sourceDetail("knowledge_version", "test:knowledge:v1", "test:source")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestPersonalNotesEnforceAtomicPreconditionsAndOwnCleanup(t *testing.T) {
	s, c, p, snapshot, _ := contentFixture(t)
	ctx := context.Background()
	_, err := s.mutateNote(ctx, p, c.OrganizationID, "test:knowledge", "note", "", "", false)
	require.ErrorContains(t, err, "PRECONDITION_REQUIRED")
	note, err := s.mutateNote(ctx, p, c.OrganizationID, "test:knowledge", "first", "", "*", false)
	require.NoError(t, err)
	_, err = s.mutateNote(ctx, p, c.OrganizationID, "test:knowledge", "duplicate", "", "*", false)
	require.ErrorContains(t, err, "PRECONDITION_FAILED")
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, text := range []string{"device one", "device two"} {
		wg.Add(1)
		go func(text string) {
			defer wg.Done()
			_, err := s.mutateNote(ctx, p, c.OrganizationID, "test:knowledge", text, `"`+note.Revision+`"`, "", false)
			results <- err
		}(text)
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else {
			var typed *Error
			require.ErrorAs(t, err, &typed)
			require.Equal(t, 412, typed.Status)
			conflict++
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, conflict)
	require.NoError(t, s.SetFavorite(ctx, p, c.OrganizationID, "test:knowledge"))
	require.NoError(t, s.SetFavorite(ctx, p, c.OrganizationID, "test:knowledge"))
	f := contentFrame(t, s, c, p, snapshot)
	favorites, err := f.favorites()
	require.NoError(t, err)
	require.Len(t, favorites, 1)
	require.EqualValues(t, 1, *f.statistics(analysisSelection{}).stats.RatedInteractions)
	note, err = s.ReadNote(ctx, p, c.OrganizationID, "test:knowledge")
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE bi_manager_grants SET revoked_at=NOW() WHERE manager_id=$1 AND organization_id=$2`, p.ManagerID, c.OrganizationID)
	require.NoError(t, err)
	_, err = s.ReadNote(ctx, p, c.OrganizationID, "test:knowledge")
	require.ErrorIs(t, err, ErrNotFound)
	_, err = s.mutateNote(ctx, p, c.OrganizationID, "test:knowledge", "", `"`+note.Revision+`"`, "", true)
	require.NoError(t, err)
	require.NoError(t, s.RemoveFavorite(ctx, p, c.OrganizationID, "test:knowledge"))
	_, err = readNote(ctx, s.db, p, c.OrganizationID, "test:knowledge")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestContentHTTPMatchesSchema(t *testing.T) {
	s, c, p, snapshot, _ := contentFixture(t)
	h := &Handler{service: s}
	r := gin.New()
	r.Use(Headers, func(c *gin.Context) { c.Set(principalKey, p); c.Next() })
	paths := []struct{ op, path, pattern, schema string }{
		{"knowledge", "/knowledge", "/knowledge", "KnowledgeRowPage"}, {"knowledge_detail", "/knowledge/test:knowledge", "/knowledge/:knowledge_id", "KnowledgeDetail"},
		{"versions", "/knowledge/test:knowledge/versions", "/knowledge/:knowledge_id/versions", "VersionSummaryPage"}, {"version", "/knowledge/test:knowledge/versions/test:knowledge:v1", "/knowledge/:knowledge_id/versions/:version_id", "KnowledgeVersion"},
		{"references", "/knowledge/test:knowledge/references", "/knowledge/:knowledge_id/references", "ReferenceEvidencePage"}, {"cases", "/cases", "/cases", "CaseRowPage"}, {"case", "/cases/test:case", "/cases/:case_id", "CaseDetail"},
		{"source", "/sources/test:source&parent_kind=knowledge_version&parent_id=test:knowledge:v1", "/sources/:source_id", "SourceDetail"},
		{"evaluations", "/evaluations", "/evaluations", "EvaluationRowPage"}, {"evaluation", "/evaluations/test:evaluation", "/evaluations/:evaluation_id", "EvaluationDetail"}, {"samples", "/evaluations/test:evaluation/samples", "/evaluations/:evaluation_id/samples", "EvaluationSamplePage"},
		{"search", "/search", "/search", "SearchHitPage"}, {"favorites", "/me/favorites", "/me/favorites", "FavoritePage"},
	}
	for _, tc := range paths {
		r.GET("/:organization_id"+tc.pattern, h.QueryContent(tc.op))
	}
	for _, tc := range paths {
		t.Run(tc.op, func(t *testing.T) {
			parts := strings.SplitN(tc.path, "&", 2)
			target := "/" + c.OrganizationID + parts[0] + "?context_id=" + url.QueryEscape(snapshot.ID)
			if len(parts) == 2 {
				target += "&" + parts[1]
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", target, nil))
			require.Equal(t, 200, w.Code, w.Body.String())
			require.NoError(t, validateSchema(tc.schema, w.Body.Bytes()), w.Body.String())
		})
	}
}

func TestApplicationSearchKeepsPeriodUsageAfterEligibilityEnds(t *testing.T) {
	s, c, p, snapshot, _ := contentFixture(t)
	app := acceptancePayload(t, s, c, "application", "test:app")
	app["team_ids"] = []string{"test:b"}
	changes := []any{map[string]any{"kind": "application", "revision": 2, "payload": app}}
	for _, member := range []string{"one", "two", "mover", "leaver"} {
		eligible := acceptancePayload(t, s, c, "eligibility", "test:eligible:"+member)
		eligible["valid_to"] = "2026-09-14T00:00:00Z"
		changes = append(changes, map[string]any{"kind": "eligibility", "revision": 2, "payload": eligible})
	}
	job := applyImport(t, s, c, completeBatch(t, "content-seed", "application-reassigned", changes))
	require.Equal(t, "applied", job.Status, job.Errors)
	current, err := s.CreateAnalysisContext(context.Background(), p, c.OrganizationID, snapshot.Filters)
	require.NoError(t, err)
	state, err := s.analysisContext(context.Background(), p, c.OrganizationID, current.ID, "knowledge:read")
	require.NoError(t, err)
	for _, kind := range []string{"all", "application"} {
		f, err := s.loadMetadata(context.Background(), state)
		require.NoError(t, err)
		hits, err := f.searchContent("Application", kind)
		require.NoError(t, err)
		require.Len(t, hits, 1)
		require.Equal(t, Target{Kind: "application", ID: "test:app"}, hits[0].Target)
	}
}
