//go:build integration

package bi

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func connectorFixture(t *testing.T) (*Service, Connector, ConnectorRegistration, string) {
	t.Helper()
	s, _, ids := authFixture(t)
	ctx := context.Background()
	var groupID int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO groups(name) VALUES($1) RETURNING id`, randomToken("group_")).Scan(&groupID))
	org := fmt.Sprintf("org_import_%d", ids[0])
	_, err := integrationDB.Exec(`INSERT INTO desktop_organizations(public_id,code,name,gateway_user_id,group_id) VALUES($1,$2,'Import test',$3,$4)`, org, fmt.Sprintf("bi%d", ids[0]), ids[0], groupID)
	require.NoError(t, err)
	zero := int64(0)
	_, err = s.SaveGrant(ctx, org, GrantInput{UserID: ids[0], Role: "org_admin", AllTeams: true, TeamIDs: []string{}, Capabilities: []string{"analytics:read"}, ExpectedRevision: &zero}, ids[1], "setup")
	require.NoError(t, err)
	registration := ConnectorRegistration{OrganizationID: org, SourceID: "gateway", Namespace: "test", AllowedKinds: append([]string{}, RecordKinds...), ExpiresAt: time.Now().Add(time.Hour)}
	_, token, err := RegisterConnector(ctx, integrationDB, registration)
	require.NoError(t, err)
	connector, err := s.AuthorizeConnector(ctx, token)
	require.NoError(t, err)
	return s, connector, registration, token
}

func importPayload(expected *string, checkpoint string, records []any) []byte {
	raw, _ := json.Marshal(map[string]any{"source_id": "gateway", "schema_version": "1", "expected_checkpoint": expected, "checkpoint": checkpoint,
		"records": records, "complete_through": nil, "history_start_date": "2026-09-01", "initial_backfill_complete": false})
	return raw
}

func applyImport(t *testing.T, s *Service, connector Connector, raw []byte) ImportJob {
	t.Helper()
	ctx := context.Background()
	job, err := s.SubmitImport(ctx, connector, randomToken("idem_"), raw)
	require.NoError(t, err)
	for i := 0; i < 20; i++ {
		worked, err := s.ProcessNextImport(ctx)
		require.NoError(t, err)
		require.True(t, worked)
		job, err = s.ImportJob(ctx, connector, job.ID)
		require.NoError(t, err)
		if job.Status == "applied" || job.Status == "rejected" {
			return job
		}
	}
	t.Fatal("import did not finish")
	return job
}

func TestImportAdmissionSerializesSourceAndSurvivesCredentialRotation(t *testing.T) {
	s, c, registration, _ := connectorFixture(t)
	ctx := context.Background()
	key := randomToken("idem_")
	body := importPayload(nil, "cursor-1", []any{})
	first, err := s.SubmitImport(ctx, c, key, body)
	require.NoError(t, err)
	_, err = s.SubmitImport(ctx, c, randomToken("idem_"), body)
	require.ErrorContains(t, err, "IMPORT_BUSY")
	_, newToken, err := RegisterConnector(ctx, integrationDB, registration)
	require.NoError(t, err)
	rotated, err := s.AuthorizeConnector(ctx, newToken)
	require.NoError(t, err)
	replay, err := s.SubmitImport(ctx, rotated, key, body)
	require.NoError(t, err)
	require.Equal(t, first, replay)
	_, err = s.SubmitImport(ctx, c, key, importPayload(nil, "different", []any{}))
	require.ErrorContains(t, err, "IDEMPOTENCY_CONFLICT")
	worked, err := s.ProcessNextImport(ctx)
	require.NoError(t, err)
	require.True(t, worked)
	job, err := s.ImportJob(ctx, c, first.ID)
	require.NoError(t, err)
	require.Equal(t, "applied", job.Status)
	replay, err = s.SubmitImport(ctx, c, key, body)
	require.NoError(t, err)
	require.Equal(t, first, replay)
	checkpoint, err := s.ImportCheckpoint(ctx, c, c.SourceID)
	require.NoError(t, err)
	require.Equal(t, "cursor-1", *checkpoint.Checkpoint)
	require.Nil(t, checkpoint.CompleteThrough)
	_, err = s.SubmitImport(ctx, c, randomToken("idem_"), importPayload(nil, "cursor-2", []any{}))
	require.ErrorContains(t, err, "CHECKPOINT_CONFLICT")
	checkpointValue := "cursor-1"
	_, err = s.SubmitImport(ctx, c, randomToken("idem_"), importPayload(&checkpointValue, "cursor-1", []any{}))
	require.ErrorContains(t, err, "CHECKPOINT_CONFLICT")
	job = applyImport(t, s, c, importPayload(&checkpointValue, "cursor-2", []any{}))
	require.Equal(t, "applied", job.Status)
	other := c
	other.OrganizationID = "other-organization"
	_, err = s.ImportJob(ctx, other, first.ID)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestImportsResolveCyclicVersionsAndRejectWholeInvalidBatches(t *testing.T) {
	s, c, _, _ := connectorFixture(t)
	acl := map[string]any{"organization_readable": true, "team_ids": []string{}, "manager_ids": []string{}}
	app := map[string]any{"kind": "application", "revision": 1, "payload": map[string]any{"id": "test:app", "name": "App", "type": "tool", "summary": "", "current_version_id": "test:app:v1", "team_ids": []string{}, "scene_ids": []string{}, "acl": acl}}
	version := map[string]any{"kind": "application_version", "revision": 1, "payload": map[string]any{"id": "test:app:v1", "application_id": "test:app", "version": "1", "released_at": "2026-09-01T00:00:00Z", "knowledge_version_ids": []string{}}}
	job := applyImport(t, s, c, importPayload(nil, "first", []any{app, version, app}))
	require.Equal(t, "applied", job.Status, job.Errors)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM bi_entity_versions WHERE organization_id=$1`, c.OrganizationID).Scan(&count))
	require.Equal(t, 2, count)
	expected := "first"
	bad := map[string]any{"kind": "membership", "revision": 1, "payload": map[string]any{"id": "test:membership", "member_id": "test:missing", "team_id": "test:missing-team", "role_id": "test:missing-role", "valid_from": "2026-09-01T00:00:00Z", "valid_to": nil, "primary": true}}
	team := map[string]any{"kind": "team", "revision": 1, "payload": map[string]any{"id": "test:new-team", "name": "Team", "status": "active"}}
	job = applyImport(t, s, c, importPayload(&expected, "second", []any{team, bad}))
	require.Equal(t, "rejected", job.Status)
	require.Equal(t, "INVALID_REFERENCE", job.Errors[0].Code)
	require.Equal(t, 1, *job.Errors[0].RecordIndex)
	checkpoint, err := s.ImportCheckpoint(context.Background(), c, c.SourceID)
	require.NoError(t, err)
	require.Equal(t, "first", *checkpoint.Checkpoint)
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM bi_entity_versions WHERE organization_id=$1`, c.OrganizationID).Scan(&count))
	require.Equal(t, 2, count)
	job = applyImport(t, s, c, importPayload(&expected, "second", []any{team}))
	require.Equal(t, "applied", job.Status)
}

func TestImportRecoveryPublishesOnlyAfterViewsAreBuilt(t *testing.T) {
	s, c, _, _ := connectorFixture(t)
	ctx := context.Background()
	usage := map[string]any{"kind": "usage", "revision": 1, "payload": map[string]any{"id": "test:usage", "occurred_at": "2026-09-20T00:00:00Z", "actor_type": "unknown",
		"member_id": nil, "team_id": nil, "application_id": nil, "application_version_id": nil, "scene_id": nil, "requested_model": "model", "outcome": "succeeded", "duration_ms": nil, "retry_of": nil,
		"tokens": map[string]any{"encoding": "exclusive_buckets", "input": "100", "output": "20", "cache_read": "40", "cache_write": "10"}}}
	job, err := s.SubmitImport(ctx, c, randomToken("idem_"), importPayload(nil, "after-recovery", []any{usage}))
	require.NoError(t, err)
	w, err := s.claimImport(ctx)
	require.NoError(t, err)
	require.Equal(t, job.ID, w.ID)
	revision, problems, err := s.stageImport(ctx, w)
	require.NoError(t, err)
	require.Empty(t, problems)
	checkpoint, err := s.ImportCheckpoint(ctx, c, c.SourceID)
	require.NoError(t, err)
	require.Nil(t, checkpoint.Checkpoint)
	var visible int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM bi_entity_versions v JOIN bi_data_revisions d ON d.id=v.data_revision WHERE v.organization_id=$1 AND d.status='published'`, c.OrganizationID).Scan(&visible))
	require.Zero(t, visible)
	_, err = integrationDB.Exec(`UPDATE bi_import_batches SET lease_until=NOW()-INTERVAL '1 second' WHERE id=$1`, job.ID)
	require.NoError(t, err)
	replacement, err := s.claimImport(ctx)
	require.NoError(t, err)
	require.NotNil(t, replacement)
	require.ErrorIs(t, s.buildImport(ctx, w, revision), errLeaseLost)
	require.NoError(t, s.processImport(ctx, replacement))
	final, err := s.ImportJob(ctx, c, job.ID)
	require.NoError(t, err)
	require.Equal(t, "applied", final.Status)
	var input, total string
	require.NoError(t, integrationDB.QueryRow(`SELECT input_tokens::text,(input_tokens+output_tokens)::text FROM bi_usage_facts WHERE organization_id=$1`, c.OrganizationID).Scan(&input, &total))
	require.Equal(t, "150", input)
	require.Equal(t, "170", total)
}

func TestSourceHistoryAndACLRestrictionsSurviveFailedPublication(t *testing.T) {
	s, c, _, _ := connectorFixture(t)
	ctx := context.Background()
	acl := map[string]any{"organization_readable": true, "team_ids": []string{}, "manager_ids": []string{}}
	source := map[string]any{"kind": "source", "revision": 1, "payload": map[string]any{"id": "test:source", "version_id": "test:source:v1", "title": "Original source", "type": "document", "updated_at": "2026-09-01T00:00:00Z", "body": []string{"original"}, "acl": acl}}
	knowledge := map[string]any{"kind": "knowledge", "revision": 1, "payload": map[string]any{"id": "test:knowledge", "title": "Knowledge", "summary": "", "status": "valid", "current_version_id": "test:knowledge:v1", "owner_id": nil, "owner_name": nil, "created_at": "2026-09-01T00:00:00Z", "updated_at": "2026-09-01T00:00:00Z", "status_effective_at": "2026-09-01T00:00:00Z", "application_ids": []string{}, "scene_ids": []string{}, "acl": acl}}
	version := map[string]any{"kind": "knowledge_version", "revision": 1, "payload": map[string]any{"id": "test:knowledge:v1", "knowledge_id": "test:knowledge", "version": "1", "created_at": "2026-09-01T00:00:00Z", "valid_from": "2026-09-01T00:00:00Z", "valid_to": nil, "change_summary": "initial", "applicability": "all", "body": []string{"knowledge body"}, "source_ids": []string{"test:source"}}}
	first := applyImport(t, s, c, importPayload(nil, "history-1", []any{version, knowledge, source}))
	require.Equal(t, "applied", first.Status, first.Errors)
	source["revision"] = 2
	sourcePayload := source["payload"].(map[string]any)
	sourcePayload["version_id"] = "test:source:v2"
	sourcePayload["body"] = []string{"replacement"}
	version["revision"] = 2
	version["payload"].(map[string]any)["valid_to"] = "2026-09-21T00:00:00Z"
	expected := "history-1"
	second := applyImport(t, s, c, importPayload(&expected, "history-2", []any{source, version}))
	require.Equal(t, "applied", second.Status, second.Errors)
	var linkedVersion string
	require.NoError(t, integrationDB.QueryRow(`SELECT l.source_version_id FROM bi_source_version_links l JOIN bi_data_revisions d ON d.id=l.parent_data_revision WHERE l.organization_id=$1 AND d.public_id=$2`, c.OrganizationID, *second.DataRevision).Scan(&linkedVersion))
	require.Equal(t, "test:source:v1", linkedVersion)
	source["revision"] = 3
	sourcePayload["acl"] = map[string]any{"organization_readable": false, "team_ids": []string{}, "manager_ids": []string{}}
	expected = "history-2"
	body := importPayload(&expected, "history-3", []any{source})
	job, err := s.SubmitImport(ctx, c, randomToken("idem_"), body)
	require.NoError(t, err)
	w, err := s.claimImport(ctx)
	require.NoError(t, err)
	require.Equal(t, job.ID, w.ID)
	_, problems, err := s.stageImport(ctx, w)
	require.NoError(t, err)
	require.Empty(t, problems)
	var denials int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM bi_acl_denials WHERE organization_id=$1`, c.OrganizationID).Scan(&denials))
	require.Positive(t, denials)
	require.NoError(t, s.rejectImport(ctx, w, []ImportError{{Code: "INTERNAL_ERROR", Message: "Simulated view failure"}}))
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM bi_acl_denials WHERE organization_id=$1`, c.OrganizationID).Scan(&denials))
	require.Positive(t, denials)
	checkpoint, err := s.ImportCheckpoint(ctx, c, c.SourceID)
	require.NoError(t, err)
	require.Equal(t, "history-2", *checkpoint.Checkpoint)
	repair := applyImport(t, s, c, body)
	require.Equal(t, "applied", repair.Status, repair.Errors)
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM bi_acl_denials WHERE organization_id=$1`, c.OrganizationID).Scan(&denials))
	require.Zero(t, denials)
}

func TestMultipleEntityRevisionsInOneBatchKeepSourceVersions(t *testing.T) {
	s, c, _, _ := connectorFixture(t)
	acl := map[string]any{"organization_readable": true, "team_ids": []string{}, "manager_ids": []string{}}
	records := []any{}
	for _, revision := range []int{2, 1} {
		records = append(records, map[string]any{"kind": "source", "revision": revision, "payload": map[string]any{"id": "test:source", "version_id": fmt.Sprintf("test:source:v%d", revision), "title": "Source", "type": "document", "updated_at": "2026-09-01T00:00:00Z", "body": []string{fmt.Sprintf("body-%d", revision)}, "acl": acl}})
	}
	job := applyImport(t, s, c, importPayload(nil, "first", records))
	require.Equal(t, "applied", job.Status, job.Errors)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(DISTINCT payload->>'version_id') FROM bi_entity_versions WHERE organization_id=$1 AND kind='source'`, c.OrganizationID).Scan(&count))
	require.Equal(t, 2, count)
	conflicting := records[1].(map[string]any)
	conflicting["revision"] = 3
	conflicting["payload"].(map[string]any)["body"] = []string{"changed historic content"}
	expected := "first"
	job = applyImport(t, s, c, importPayload(&expected, "second", []any{conflicting}))
	require.Equal(t, "rejected", job.Status)
	require.Equal(t, "IMMUTABLE_VERSION", job.Errors[0].Code)
}
