//go:build integration

package bi

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUsageCorrectionsRevalidatePublishedRatings(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		change     func(map[string]any)
	}{
		{"member", "INVALID_IDENTITY", func(p map[string]any) { p["member_id"] = "test:two" }},
		{"actor", "INVALID_IDENTITY", func(p map[string]any) { p["actor_type"], p["member_id"] = "automatic", nil }},
		{"time", "INVALID_INTERVAL", func(p map[string]any) { p["occurred_at"] = "2026-09-22T04:00:00Z" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, c, p, oldContext, _ := contentFixture(t)
			usage := acceptancePayload(t, s, c, "usage", "test:one:1")
			tc.change(usage)
			job := applyImport(t, s, c, completeBatch(t, "content-seed", "correct-usage", []any{map[string]any{"kind": "usage", "revision": 2, "payload": usage}}))
			require.Equal(t, "rejected", job.Status)
			require.Equal(t, tc.code, job.Errors[0].Code)
			require.Equal(t, 0, *job.Errors[0].RecordIndex)
			checkpoint, err := s.ImportCheckpoint(context.Background(), c, c.SourceID)
			require.NoError(t, err)
			require.Equal(t, "content-seed", *checkpoint.Checkpoint)
			current, err := s.CreateAnalysisContext(context.Background(), p, c.OrganizationID, oldContext.Filters)
			require.NoError(t, err)
			require.Equal(t, oldContext.DataRevision, current.DataRevision)
			stats := contentFrame(t, s, c, p, current).statistics(analysisSelection{MemberID: "test:two"}).stats
			require.EqualValues(t, 0, *stats.HelpfulInteractions)
		})
	}
}

func TestUsageCorrectionUsesFinalRatingAndKeepsOldContext(t *testing.T) {
	for _, retract := range []bool{false, true} {
		t.Run(map[bool]string{false: "correct-rating", true: "retract-rating"}[retract], func(t *testing.T) {
			s, c, p, oldContext, _ := contentFixture(t)
			usage := acceptancePayload(t, s, c, "usage", "test:one:1")
			usage["member_id"] = "test:two"
			rating := acceptancePayload(t, s, c, "rating", "test:rating")
			rating["member_id"] = "test:two"
			change := any(map[string]any{"kind": "rating", "revision": 2, "payload": rating})
			if retract {
				change = map[string]any{"kind": "tombstone", "entity_kind": "rating", "entity_id": "test:rating", "revision": 2, "effective_at": "2026-09-23T00:00:00Z", "reason": "corrected"}
			}
			job := applyImport(t, s, c, completeBatch(t, "content-seed", "correct-together", []any{change, map[string]any{"kind": "usage", "revision": 2, "payload": usage}}))
			require.Equal(t, "applied", job.Status, job.Errors)
			current, err := s.CreateAnalysisContext(context.Background(), p, c.OrganizationID, oldContext.Filters)
			require.NoError(t, err)
			require.NotEqual(t, oldContext.DataRevision, current.DataRevision)
			oldStats := contentFrame(t, s, c, p, oldContext).statistics(analysisSelection{MemberID: "test:one"}).stats
			require.EqualValues(t, 1, *oldStats.HelpfulInteractions)
			frame := contentFrame(t, s, c, p, current)
			one, two := frame.statistics(analysisSelection{MemberID: "test:one"}).stats, frame.statistics(analysisSelection{MemberID: "test:two"}).stats
			require.EqualValues(t, 0, *one.HelpfulInteractions)
			expected := int64(1)
			if retract {
				expected = 0
			}
			require.Equal(t, expected, *two.HelpfulInteractions)
		})
	}
}

func TestUsageTimeCorrectionRevalidatesReferencesAndRetries(t *testing.T) {
	for _, tc := range []struct{ kind, date string }{{"reference", "2026-08-31T04:00:00Z"}, {"retry", "2026-09-16T04:00:00Z"}} {
		t.Run(tc.kind, func(t *testing.T) {
			s, c, p, oldContext, _ := contentFixture(t)
			dependent := []any{}
			if tc.kind == "reference" {
				version := acceptancePayload(t, s, c, "knowledge_version", "test:knowledge:v1")
				version["id"], version["version"] = "test:knowledge:later", "later"
				version["created_at"], version["valid_from"] = "2026-09-01T00:00:00Z", "2026-09-01T00:00:00Z"
				dependent = append(dependent, analyticRecord("knowledge_version", version), analyticRecord("reference", map[string]any{"id": "test:reference:later", "usage_event_id": "test:one:1", "knowledge_version_id": "test:knowledge:later"}))
			} else {
				retry := acceptancePayload(t, s, c, "usage", "test:one:2")
				retry["retry_of"] = "test:one:1"
				dependent = append(dependent, map[string]any{"kind": "usage", "revision": 2, "payload": retry})
				// Keep rating time after both attempts so only retry ordering can reject.
				rating := acceptancePayload(t, s, c, "rating", "test:rating")
				rating["rated_at"] = "2026-09-20T04:00:00Z"
				dependent = append(dependent, map[string]any{"kind": "rating", "revision": 2, "payload": rating})
			}
			job := applyImport(t, s, c, completeBatch(t, "content-seed", "dependent-seed", dependent))
			require.Equal(t, "applied", job.Status, job.Errors)
			usage := acceptancePayload(t, s, c, "usage", "test:one:1")
			usage["occurred_at"] = tc.date
			job = applyImport(t, s, c, completeBatch(t, "dependent-seed", "invalid-date", []any{map[string]any{"kind": "usage", "revision": 2, "payload": usage}}))
			require.Equal(t, "rejected", job.Status)
			require.Equal(t, "INVALID_REFERENCE", job.Errors[0].Code)
			require.Equal(t, 0, *job.Errors[0].RecordIndex)
			current, err := s.CreateAnalysisContext(context.Background(), p, c.OrganizationID, oldContext.Filters)
			require.NoError(t, err)
			frame := contentFrame(t, s, c, p, current)
			require.Equal(t, "1500", *frame.statistics(analysisSelection{}).stats.Tokens.Total)
			corrections := []any{map[string]any{"kind": "usage", "revision": 2, "payload": usage}}
			expectedTokens := "1500"
			if tc.kind == "reference" {
				reference := acceptancePayload(t, s, c, "reference", "test:reference:later")
				reference["knowledge_version_id"] = "test:knowledge:v1"
				corrections = append(corrections, map[string]any{"kind": "reference", "revision": 2, "payload": reference})
				expectedTokens = "1400"
			} else {
				retry := acceptancePayload(t, s, c, "usage", "test:one:2")
				retry["occurred_at"] = "2026-09-17T04:00:00Z"
				corrections = append(corrections, map[string]any{"kind": "usage", "revision": 3, "payload": retry})
			}
			job = applyImport(t, s, c, completeBatch(t, "dependent-seed", "correct-together", corrections))
			require.Equal(t, "applied", job.Status, job.Errors)
			latest, err := s.CreateAnalysisContext(context.Background(), p, c.OrganizationID, oldContext.Filters)
			require.NoError(t, err)
			latestFrame := contentFrame(t, s, c, p, latest)
			require.Equal(t, expectedTokens, *latestFrame.statistics(analysisSelection{}).stats.Tokens.Total)
			require.Equal(t, "1500", *contentFrame(t, s, c, p, oldContext).statistics(analysisSelection{}).stats.Tokens.Total)
		})
	}
}

func TestUsageCorrectionsPreserveRetractionsAndClosedVersionHistory(t *testing.T) {
	s, c, p, oldContext, _ := contentFixture(t)
	version := acceptancePayload(t, s, c, "knowledge_version", "test:knowledge:v1")
	version["valid_to"] = "2026-09-16T00:00:00Z"
	job := applyImport(t, s, c, completeBatch(t, "content-seed", "closed-version", []any{map[string]any{"kind": "knowledge_version", "revision": 2, "payload": version}}))
	require.Equal(t, "applied", job.Status, job.Errors)
	usage := acceptancePayload(t, s, c, "usage", "test:one:1")
	usage["tokens"] = map[string]any{"encoding": "exclusive_buckets", "input": "120", "output": "0", "cache_read": "0", "cache_write": "0"}
	job = applyImport(t, s, c, completeBatch(t, "closed-version", "correct-tokens", []any{map[string]any{"kind": "usage", "revision": 2, "payload": usage}}))
	require.Equal(t, "applied", job.Status, job.Errors)
	current, err := s.CreateAnalysisContext(context.Background(), p, c.OrganizationID, oldContext.Filters)
	require.NoError(t, err)
	frame := contentFrame(t, s, c, p, current)
	require.Equal(t, "1520", *frame.statistics(analysisSelection{}).stats.Tokens.Total)
	evidence, err := frame.knowledgeEvidence("test:knowledge")
	require.NoError(t, err)
	require.Len(t, evidence, 1)
	job = applyImport(t, s, c, completeBatch(t, "correct-tokens", "retract-usage", []any{map[string]any{"kind": "tombstone", "entity_kind": "usage", "entity_id": "test:one:1", "revision": 3, "effective_at": "2026-09-23T00:00:00Z", "reason": "corrected"}}))
	require.Equal(t, "applied", job.Status, job.Errors)
	current, err = s.CreateAnalysisContext(context.Background(), p, c.OrganizationID, oldContext.Filters)
	require.NoError(t, err)
	frame = contentFrame(t, s, c, p, current)
	require.Equal(t, "1400", *frame.statistics(analysisSelection{}).stats.Tokens.Total)
	remaining, err := frame.knowledgeEvidence("test:knowledge")
	require.NoError(t, err)
	require.Empty(t, remaining)
	require.Equal(t, "1500", *contentFrame(t, s, c, p, oldContext).statistics(analysisSelection{}).stats.Tokens.Total)
}

func TestUsageCorrectionCannotRewriteAnotherSourcesRating(t *testing.T) {
	s, c, _, _ := analysisFixture(t)
	ctx := context.Background()
	_, token, err := RegisterConnector(ctx, s.db, ConnectorRegistration{OrganizationID: c.OrganizationID, SourceID: "feedback", Namespace: "feedback", AllowedKinds: []string{"rating", "tombstone"}, ExpiresAt: time.Now().Add(24 * time.Hour)})
	require.NoError(t, err)
	other, err := s.AuthorizeConnector(ctx, token)
	require.NoError(t, err)
	rating := map[string]any{"id": "feedback:rating", "usage_event_id": "test:one:2", "member_id": "test:one", "value": "negative", "rated_at": "2026-09-17T04:00:00Z"}
	foreignBatch := func(expected *string, checkpoint string, records []any) []byte {
		var body map[string]any
		require.NoError(t, json.Unmarshal(importPayload(expected, checkpoint, records), &body))
		body["source_id"] = other.SourceID
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		return raw
	}
	job := applyImport(t, s, other, foreignBatch(nil, "feedback-seed", []any{analyticRecord("rating", rating)}))
	require.Equal(t, "applied", job.Status, job.Errors)
	usage := acceptancePayload(t, s, c, "usage", "test:one:2")
	usage["member_id"] = "test:two"
	change := map[string]any{"kind": "usage", "revision": 2, "payload": usage}
	job = applyImport(t, s, c, completeBatch(t, "analytics-seed", "identity-corrected", []any{change}))
	require.Equal(t, "rejected", job.Status)
	require.Equal(t, "INVALID_IDENTITY", job.Errors[0].Code)
	job = applyImport(t, s, c, completeBatch(t, "analytics-seed", "foreign-rating-write", []any{change, map[string]any{"kind": "rating", "revision": 2, "payload": rating}}))
	require.Equal(t, "rejected", job.Status)
	require.Equal(t, "FORBIDDEN_SOURCE", job.Errors[0].Code)
	require.Equal(t, "test:one", acceptancePayload(t, s, other, "rating", "feedback:rating")["member_id"])
	job = applyImport(t, s, other, foreignBatch(ptr("feedback-seed"), "feedback-retracted", []any{map[string]any{"kind": "tombstone", "entity_kind": "rating", "entity_id": "feedback:rating", "revision": 2, "effective_at": "2026-09-23T00:00:00Z", "reason": "corrected"}}))
	require.Equal(t, "applied", job.Status, job.Errors)
	job = applyImport(t, s, c, completeBatch(t, "analytics-seed", "identity-corrected", []any{change}))
	require.Equal(t, "applied", job.Status, job.Errors)
	rating["member_id"] = "test:two"
	job = applyImport(t, s, other, foreignBatch(ptr("feedback-retracted"), "feedback-corrected", []any{map[string]any{"kind": "rating", "revision": 3, "payload": rating}}))
	require.Equal(t, "applied", job.Status, job.Errors)
}

func TestUsageTimeCorrectionRespectsSameBatchClosedAndRetractedVersion(t *testing.T) {
	s, c, _, _, _ := contentFixture(t)
	version := acceptancePayload(t, s, c, "knowledge_version", "test:knowledge:v1")
	version["valid_to"] = "2026-09-16T00:00:00Z"
	usage := acceptancePayload(t, s, c, "usage", "test:one:1")
	usage["occurred_at"] = "2026-09-17T04:00:00Z"
	rating := acceptancePayload(t, s, c, "rating", "test:rating")
	rating["rated_at"] = "2026-09-18T04:00:00Z"
	job := applyImport(t, s, c, completeBatch(t, "content-seed", "close-and-correct", []any{
		map[string]any{"kind": "knowledge_version", "revision": 2, "payload": version},
		map[string]any{"kind": "tombstone", "entity_kind": "knowledge_version", "entity_id": "test:knowledge:v1", "revision": 3, "effective_at": "2026-09-23T00:00:00Z", "reason": "revoked"},
		map[string]any{"kind": "usage", "revision": 2, "payload": usage},
		map[string]any{"kind": "rating", "revision": 2, "payload": rating},
	}))
	require.Equal(t, "rejected", job.Status)
	require.Equal(t, "INVALID_REFERENCE", job.Errors[0].Code)
	require.Equal(t, 2, *job.Errors[0].RecordIndex)
}
