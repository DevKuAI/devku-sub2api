//go:build integration

package bi

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func acceptancePayload(t *testing.T, s *Service, c Connector, kind, id string) map[string]any {
	t.Helper()
	r, err := recordAt(context.Background(), s.db, c.OrganizationID, kind, id, -1)
	require.NoError(t, err)
	require.NotNil(t, r)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(r.Raw, &payload))
	return payload
}

func TestAcceptanceA04MoreUsageAndNegativeFeedbackRemainSeparate(t *testing.T) {
	s, c, p, _, _ := contentFixture(t)
	records := []any{
		analyticRecord("rating", map[string]any{"id": "test:rating:previous", "usage_event_id": "test:history:one:2", "member_id": "test:one", "value": "helpful", "rated_at": "2026-09-09T04:00:00Z"}),
		analyticRecord("rating", map[string]any{"id": "test:rating:negative", "usage_event_id": "test:one:2", "member_id": "test:one", "value": "negative", "rated_at": "2026-09-17T04:00:00Z"}),
	}
	job := applyImport(t, s, c, completeBatch(t, "content-seed", "feedback-growth", records))
	require.Equal(t, "applied", job.Status, job.Errors)
	snapshot, err := s.CreateAnalysisContext(context.Background(), p, c.OrganizationID, Filters{Period: "week", TeamIDs: []string{"test:a"}})
	require.NoError(t, err)
	f := contentFrame(t, s, c, p, snapshot)
	current := f.statistics(analysisSelection{ApplicationID: "test:app"}).stats
	previous := f.calculate(snapshot.ComparisonRange, analysisSelection{ApplicationID: "test:app"}).stats
	require.EqualValues(t, 2, *previous.Requests)
	require.EqualValues(t, 7, *current.Requests)
	require.Equal(t, 1.0, *previous.HelpfulRate.Value)
	require.Equal(t, 0.5, *current.HelpfulRate.Value)
	feedback, err := f.feedback(analysisSelection{ApplicationID: "test:app"})
	require.NoError(t, err)
	require.EqualValues(t, 2, *feedback.Rated)
	require.EqualValues(t, 1, *feedback.Helpful)
	require.EqualValues(t, 1, *feedback.Negative)
	require.EqualValues(t, 2, *feedback.Coverage.Numerator)
	require.EqualValues(t, 5, *feedback.Coverage.Denominator)
	require.InDelta(t, 0.4, *feedback.Coverage.Value, 1e-12)
	require.Len(t, feedback.ByApplication, 1)
	require.Equal(t, "test:app", feedback.ByApplication[0].Application.ID)
	require.EqualValues(t, 1, *feedback.ByApplication[0].Negative)
}

func TestAcceptanceA05CurrentExpiryDoesNotRewriteReferenceTime(t *testing.T) {
	s, c, p, original, _ := contentFixture(t)
	knowledge := acceptancePayload(t, s, c, "knowledge", "test:knowledge")
	knowledge["status"] = "expired"
	knowledge["status_effective_at"] = "2026-09-16T00:00:00Z"
	knowledge["updated_at"] = "2026-09-16T00:00:00Z"
	job := applyImport(t, s, c, completeBatch(t, "content-seed", "knowledge-expired", []any{map[string]any{"kind": "knowledge", "revision": 2, "payload": knowledge}}))
	require.Equal(t, "applied", job.Status, job.Errors)
	snapshot, err := s.CreateAnalysisContext(context.Background(), p, c.OrganizationID, original.Filters)
	require.NoError(t, err)
	f := contentFrame(t, s, c, p, snapshot)
	assets, err := f.assets(analysisSelection{})
	require.NoError(t, err)
	require.EqualValues(t, 1, *assets.StaleReferencedNow)
	require.EqualValues(t, 0, *assets.ExpiredAtUse)
	evidence, err := f.knowledgeEvidence("test:knowledge")
	require.NoError(t, err)
	require.Len(t, evidence, 1)
	require.False(t, evidence[0].ExpiredAtUse)
	require.Equal(t, "test:knowledge:v1", evidence[0].KnowledgeVersionID)
	require.Equal(t, "test:app", evidence[0].Application.ID)
	job = applyImport(t, s, c, completeBatch(t, "knowledge-expired", "expired-reference", []any{
		analyticRecord("reference", map[string]any{"id": "test:reference:expired", "usage_event_id": "test:one:2", "knowledge_version_id": "test:knowledge:v1"}),
	}))
	require.Equal(t, "applied", job.Status, job.Errors)
	latest, err := s.CreateAnalysisContext(context.Background(), p, c.OrganizationID, original.Filters)
	require.NoError(t, err)
	assets, err = contentFrame(t, s, c, p, latest).assets(analysisSelection{})
	require.NoError(t, err)
	require.EqualValues(t, 1, *assets.ExpiredAtUse)
	usage, err := contentFrame(t, s, c, p, latest).knowledgeUsage("test:knowledge")
	require.NoError(t, err)
	require.EqualValues(t, 2, *usage.References)
	require.EqualValues(t, 1, *usage.ExpiredAtUseReferences)
	oldAssets, err := contentFrame(t, s, c, p, original).assets(analysisSelection{})
	require.NoError(t, err)
	require.EqualValues(t, 0, *oldAssets.StaleReferencedNow)
	require.EqualValues(t, 1, *oldAssets.ReusedValidKnowledge)
}

func TestAcceptanceA06MultipleVersionsKeepEvidenceButCountOneEvent(t *testing.T) {
	s, c, p, original, _ := contentFixture(t)
	version := acceptancePayload(t, s, c, "knowledge_version", "test:knowledge:v1")
	version["id"], version["version"], version["body"] = "test:knowledge:v2", "2", []string{"Second version"}
	version["created_at"], version["valid_from"] = "2026-09-01T00:00:00Z", "2026-09-01T00:00:00Z"
	job := applyImport(t, s, c, completeBatch(t, "content-seed", "multiple-versions", []any{
		analyticRecord("knowledge_version", version),
		analyticRecord("reference", map[string]any{"id": "test:reference:v2", "usage_event_id": "test:one:1", "knowledge_version_id": "test:knowledge:v2"}),
		analyticRecord("reference", map[string]any{"id": "test:reference:v2:duplicate", "usage_event_id": "test:one:1", "knowledge_version_id": "test:knowledge:v2"}),
	}))
	require.Equal(t, "applied", job.Status, job.Errors)
	snapshot, err := s.CreateAnalysisContext(context.Background(), p, c.OrganizationID, original.Filters)
	require.NoError(t, err)
	f := contentFrame(t, s, c, p, snapshot)
	evidence, err := f.knowledgeEvidence("test:knowledge")
	require.NoError(t, err)
	require.Len(t, evidence, 2)
	require.Equal(t, evidence[0].EventID, evidence[1].EventID)
	require.ElementsMatch(t, []string{"test:knowledge:v1", "test:knowledge:v2"}, []string{evidence[0].KnowledgeVersionID, evidence[1].KnowledgeVersionID})
	usage, err := f.knowledgeUsage("test:knowledge")
	require.NoError(t, err)
	require.EqualValues(t, 1, *usage.References)
	require.EqualValues(t, 1, *usage.Rated)
	require.EqualValues(t, 1, *usage.Helpful)
}

func TestAcceptanceA14RatingRevisionReplacesOneVoteAndNotesStaySeparate(t *testing.T) {
	s, c, p, original, _ := contentFixture(t)
	rating := acceptancePayload(t, s, c, "rating", "test:rating")
	rating["value"], rating["rated_at"] = "negative", "2026-09-22T04:00:00Z"
	job := applyImport(t, s, c, completeBatch(t, "content-seed", "rating-revised", []any{map[string]any{"kind": "rating", "revision": 2, "payload": rating}}))
	require.Equal(t, "applied", job.Status, job.Errors)
	snapshot, err := s.CreateAnalysisContext(context.Background(), p, c.OrganizationID, original.Filters)
	require.NoError(t, err)
	for _, tc := range []struct {
		snapshot AnalysisContext
		helpful  int64
	}{{original, 1}, {snapshot, 0}} {
		f := contentFrame(t, s, c, p, tc.snapshot)
		stats := f.statistics(analysisSelection{}).stats
		require.EqualValues(t, 1, *stats.RatedInteractions)
		require.Equal(t, tc.helpful, *stats.HelpfulInteractions)
		usage, err := f.knowledgeUsage("test:knowledge")
		require.NoError(t, err)
		require.EqualValues(t, 1, *usage.Rated)
		require.Equal(t, tc.helpful, *usage.Helpful)
	}
	note, err := s.mutateNote(context.Background(), p, c.OrganizationID, "test:knowledge", "helpful", "", "*", false)
	require.NoError(t, err)
	_, err = s.mutateNote(context.Background(), p, c.OrganizationID, "test:knowledge", "negative", `"`+note.Revision+`"`, "", false)
	require.NoError(t, err)
	stats := contentFrame(t, s, c, p, snapshot).statistics(analysisSelection{}).stats
	require.EqualValues(t, 1, *stats.RatedInteractions)
	require.EqualValues(t, 0, *stats.HelpfulInteractions)
	require.Equal(t, 0.0, *stats.HelpfulRate.Value)
}
