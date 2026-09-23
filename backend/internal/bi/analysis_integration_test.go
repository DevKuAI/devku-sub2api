//go:build integration

package bi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func analyticRecord(kind string, payload map[string]any) any {
	return map[string]any{"kind": kind, "revision": 1, "payload": payload}
}

func analysisFixture(t *testing.T) (*Service, Connector, Principal, AnalysisContext) {
	t.Helper()
	s, connector, _, _ := connectorFixture(t)
	s.now = func() time.Time { return time.Date(2026, 9, 23, 8, 0, 0, 0, time.UTC) }
	_, err := integrationDB.Exec(`UPDATE desktop_organizations SET created_at='2026-08-01T00:00:00Z' WHERE public_id=$1`, connector.OrganizationID)
	require.NoError(t, err)
	var p Principal
	require.NoError(t, integrationDB.QueryRow(`SELECT m.id,m.user_id FROM bi_managers m JOIN bi_manager_grants g ON g.manager_id=m.id WHERE g.organization_id=$1`, connector.OrganizationID).Scan(&p.ManagerID, &p.UserID))
	caps, _ := json.Marshal(Capabilities)
	_, err = integrationDB.Exec(`UPDATE bi_manager_grants SET capabilities=$2 WHERE manager_id=$1`, p.ManagerID, string(caps))
	require.NoError(t, err)
	records := []any{}
	for _, id := range []string{"a", "b"} {
		records = append(records, analyticRecord("team", map[string]any{"id": "test:" + id, "name": "Team " + id, "status": "active"}))
	}
	records = append(records, analyticRecord("role", map[string]any{"id": "test:role", "name": "Engineer", "status": "active"}))
	acl := map[string]any{"organization_readable": true, "team_ids": []string{}, "manager_ids": []string{}}
	records = append(records,
		analyticRecord("application", map[string]any{"id": "test:app", "name": "Application", "type": "tool", "summary": "Application", "current_version_id": "test:app:v1", "team_ids": []string{"test:a", "test:b"}, "scene_ids": []string{"test:scene"}, "acl": acl}),
		analyticRecord("application_version", map[string]any{"id": "test:app:v1", "application_id": "test:app", "version": "1", "released_at": "2026-08-01T00:00:00Z", "knowledge_version_ids": []string{}}),
		analyticRecord("scene", map[string]any{"id": "test:scene", "name": "Scene", "category": "work", "summary": "Scene", "classification": "confirmed", "acl": acl}))
	for _, id := range []string{"one", "two", "mover", "leaver"} {
		var disabled any
		status := "active"
		if id == "leaver" {
			disabled = "2026-09-17T00:00:00Z"
			status = "disabled"
		}
		records = append(records, analyticRecord("member", map[string]any{"id": "test:" + id, "name": id, "status": status, "enabled_at": "2026-08-01T00:00:00Z", "disabled_at": disabled}))
		var validTo any
		if id == "mover" || id == "leaver" {
			validTo = "2026-09-17T00:00:00Z"
		}
		records = append(records, analyticRecord("membership", map[string]any{"id": "test:membership:" + id, "member_id": "test:" + id, "team_id": "test:a", "role_id": "test:role", "valid_from": "2026-08-01T00:00:00Z", "valid_to": validTo, "primary": true}))
		if id == "mover" {
			records = append(records, analyticRecord("membership", map[string]any{"id": "test:membership:moved", "member_id": "test:mover", "team_id": "test:b", "role_id": "test:role", "valid_from": "2026-09-17T00:00:00Z", "valid_to": nil, "primary": true}))
		}
		records = append(records, analyticRecord("eligibility", map[string]any{"id": "test:eligible:" + id, "member_id": "test:" + id, "application_id": nil, "scene_id": nil, "valid_from": "2026-08-01T00:00:00Z", "valid_to": nil}))
	}
	addUsage := func(id, date, actor, member, team, outcome string, input *string) {
		var memberID any
		if member != "" {
			memberID = "test:" + member
		}
		tokens := map[string]any{"encoding": "unavailable", "input": nil, "output": nil, "cache_read": nil, "cache_write": nil}
		if input != nil {
			tokens = map[string]any{"encoding": "exclusive_buckets", "input": *input, "output": "0", "cache_read": "0", "cache_write": "0"}
		}
		records = append(records, analyticRecord("usage", map[string]any{"id": "test:" + id, "occurred_at": date + "T04:00:00Z", "actor_type": actor, "member_id": memberID, "team_id": "test:" + team, "application_id": "test:app", "application_version_id": "test:app:v1", "scene_id": "test:scene", "requested_model": "model", "outcome": outcome, "duration_ms": nil, "retry_of": nil, "tokens": tokens}))
	}
	for _, member := range []string{"one", "two"} {
		for index, date := range []string{"2026-08-25", "2026-09-01", "2026-09-08"} {
			addUsage(fmt.Sprintf("history:%s:%d", member, index), date, "human", member, "a", "succeeded", ptr("10"))
		}
	}
	addUsage("first:mover", "2026-09-01", "human", "mover", "a", "succeeded", ptr("10"))
	addUsage("one:1", "2026-09-15", "human", "one", "a", "succeeded", ptr("100"))
	addUsage("one:2", "2026-09-16", "human", "one", "a", "succeeded", ptr("100"))
	addUsage("mover:1", "2026-09-15", "human", "mover", "a", "succeeded", ptr("100"))
	addUsage("mover:2", "2026-09-18", "human", "mover", "b", "succeeded", ptr("200"))
	addUsage("leaver:1", "2026-09-15", "human", "leaver", "a", "succeeded", ptr("300"))
	addUsage("automatic:1", "2026-09-16", "automatic", "", "a", "succeeded", ptr("400"))
	addUsage("unknown:1", "2026-09-16", "unknown", "", "a", "succeeded", ptr("500"))
	addUsage("failed:1", "2026-09-19", "human", "one", "a", "failed", nil)
	records = append(records, analyticRecord("rating", map[string]any{"id": "test:rating", "usage_event_id": "test:one:1", "member_id": "test:one", "value": "helpful", "rated_at": "2026-09-16T05:00:00Z"}))
	var batch map[string]any
	require.NoError(t, json.Unmarshal(importPayload(nil, "analytics-seed", records), &batch))
	batch["complete_through"] = "2026-09-23T00:00:00Z"
	batch["history_start_date"] = "2026-08-01"
	batch["initial_backfill_complete"] = true
	raw, _ := json.Marshal(batch)
	job := applyImport(t, s, connector, raw)
	require.Equal(t, "applied", job.Status, job.Errors)
	snapshot, err := s.CreateAnalysisContext(context.Background(), p, connector.OrganizationID, Filters{Period: "week", TeamIDs: []string{"test:a"}})
	require.NoError(t, err)
	return s, connector, p, snapshot
}

func TestAnalysisCohortTokensAndRetentionRemainDistinct(t *testing.T) {
	s, c, p, snapshot := analysisFixture(t)
	ctx := context.Background()
	state, err := s.analysisContext(ctx, p, c.OrganizationID, snapshot.ID, "analytics:read")
	require.NoError(t, err)
	f, err := s.loadAnalysis(ctx, state)
	require.NoError(t, err)
	stats := f.statistics(analysisSelection{}).stats
	require.EqualValues(t, 2, *stats.EligibleMembers.Numerator)
	require.EqualValues(t, 1, *stats.ActiveMembers.Numerator)
	require.EqualValues(t, 2, *stats.StableMembers.Numerator)
	require.Equal(t, "1500", *stats.Tokens.Total)
	require.Equal(t, "600", *stats.Tokens.Human)
	require.Equal(t, "400", *stats.Tokens.Automatic)
	require.Equal(t, "500", *stats.Tokens.Unknown)
	require.Equal(t, "400", *stats.Tokens.NonCohortHumanTokens)
	require.EqualValues(t, 7, *stats.Requests)
	require.EqualValues(t, 1, stats.Tokens.UnmeasuredRequests)
	require.EqualValues(t, 1, *stats.RatedInteractions)
	adoption := f.adoption(analysisSelection{})
	for _, bucket := range adoption.Frequency {
		require.Nil(t, bucket.Members)
	}
	var moved *RetentionCohort
	for i := range adoption.Retention {
		if adoption.Retention[i].FirstWeek == "2026-08-31" {
			moved = &adoption.Retention[i]
		}
	}
	require.NotNil(t, moved)
	require.EqualValues(t, 1, moved.Size)
	require.EqualValues(t, 1, *moved.Cells[1].Retained)
	for name, value := range map[string]any{"UsageStats": stats, "Adoption": adoption, "TokenAnalysis": f.tokenAnalysis(analysisSelection{}), "Trend": f.trend(analysisSelection{}, "day")} {
		require.NoError(t, schemaOutput(name, value), name)
	}
}

func TestAnalysisContextIsImmutableAndNeverWidens(t *testing.T) {
	s, c, p, snapshot := analysisFixture(t)
	ctx := context.Background()
	wrong := p
	wrong.ManagerID = "someone-else"
	_, err := s.analysisContext(ctx, wrong, c.OrganizationID, snapshot.ID, "analytics:read")
	require.ErrorIs(t, err, ErrNotFound)
	_, err = s.CreateAnalysisContext(ctx, p, c.OrganizationID, Filters{Period: "week", TeamIDs: []string{"not-visible"}})
	require.ErrorIs(t, err, ErrNotFound)
	state, err := s.analysisContext(ctx, p, c.OrganizationID, snapshot.ID, "analytics:read")
	require.NoError(t, err)
	oldRevision := state.Revision
	expected := "analytics-seed"
	var late map[string]any
	require.NoError(t, json.Unmarshal(importPayload(&expected, "late-heartbeat", []any{}), &late))
	late["complete_through"] = "2026-09-23T00:00:00Z"
	late["history_start_date"] = "2026-08-01"
	late["initial_backfill_complete"] = true
	raw, _ := json.Marshal(late)
	job := applyImport(t, s, c, raw)
	require.Equal(t, "applied", job.Status)
	state, err = s.analysisContext(ctx, p, c.OrganizationID, snapshot.ID, "analytics:read")
	require.NoError(t, err)
	require.Equal(t, oldRevision, state.Revision)
	_, err = integrationDB.Exec(`UPDATE bi_manager_grants SET all_teams=FALSE,team_ids='["test:b"]',revision=revision+1 WHERE manager_id=$1 AND organization_id=$2`, p.ManagerID, c.OrganizationID)
	require.NoError(t, err)
	_, err = s.analysisContext(ctx, p, c.OrganizationID, snapshot.ID, "analytics:read")
	require.ErrorContains(t, err, "CONTEXT_REVOKED")
	s.now = func() time.Time { return snapshot.ExpiresAt.Add(time.Second) }
	_, err = s.analysisContext(ctx, p, c.OrganizationID, snapshot.ID, "analytics:read")
	require.ErrorContains(t, err, "CONTEXT_EXPIRED")
}

func TestAnalysisHTTPResponsesFollowContract(t *testing.T) {
	s, c, p, snapshot := analysisFixture(t)
	h := &Handler{service: s}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Headers, func(c *gin.Context) { c.Set(principalKey, p); c.Next() })
	base := "/organizations/:organization_id"
	tests := []struct{ op, path, schema string }{
		{"overview", "/analytics/overview", "Overview"}, {"adoption", "/analytics/adoption", "Adoption"}, {"tokens", "/analytics/tokens", "TokenAnalysis"},
		{"feedback", "/analytics/feedback", "FeedbackAnalysis"}, {"assets", "/analytics/assets", "AssetStats"}, {"trend", "/analytics/trend", "Trend"},
		{"comparisons", "/analytics/comparisons", "ComparisonRowPage"}, {"members", "/members", "MemberRowPage"}, {"member", "/members/test:one", "MemberDetail"},
		{"team", "/teams/test:a", "TeamDetail"}, {"applications", "/applications", "ApplicationRowPage"}, {"application", "/applications/test:app", "ApplicationDetail"},
		{"scenes", "/scenes", "SceneRowPage"}, {"scene", "/scenes/test:scene", "SceneDetail"},
	}
	for _, tc := range tests {
		path := tc.path
		switch tc.op {
		case "member":
			path = "/members/:member_id"
		case "team":
			path = "/teams/:team_id"
		case "application":
			path = "/applications/:application_id"
		case "scene":
			path = "/scenes/:scene_id"
		}
		r.GET(base+path, h.QueryAnalysis(tc.op))
	}
	for _, tc := range tests {
		t.Run(tc.op, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/organizations/"+c.OrganizationID+tc.path+"?context_id="+url.QueryEscape(snapshot.ID), nil))
			require.Equal(t, 200, w.Code, w.Body.String())
			require.NoError(t, validateSchema(tc.schema, w.Body.Bytes()), w.Body.String())
		})
	}
}

func TestAnalysisPaginationPreservesTotalsAndBindsEntity(t *testing.T) {
	s, c, p, _ := analysisFixture(t)
	acl := map[string]any{"organization_readable": true, "team_ids": []string{}, "manager_ids": []string{}}
	records := []any{}
	for i := 0; i < 121; i++ {
		id := fmt.Sprintf("test:many:%03d", i)
		records = append(records,
			analyticRecord("application", map[string]any{"id": id, "name": id, "type": "tool", "summary": "", "current_version_id": id + ":v1", "team_ids": []string{"test:a"}, "scene_ids": []string{"test:scene"}, "acl": acl}),
			analyticRecord("application_version", map[string]any{"id": id + ":v1", "application_id": id, "version": "1", "released_at": "2026-08-01T00:00:00Z", "knowledge_version_ids": []string{}}),
			analyticRecord("usage", map[string]any{"id": id + ":usage", "occurred_at": "2026-09-15T04:00:00Z", "actor_type": "human", "member_id": "test:one", "team_id": "test:a", "application_id": id, "application_version_id": id + ":v1", "scene_id": "test:scene", "requested_model": fmt.Sprintf("model-%03d", i), "outcome": "succeeded", "duration_ms": nil, "retry_of": nil, "tokens": map[string]any{"encoding": "exclusive_buckets", "input": "1", "output": "0", "cache_read": "0", "cache_write": "0"}}),
			analyticRecord("rating", map[string]any{"id": id + ":rating", "usage_event_id": id + ":usage", "member_id": "test:one", "value": "helpful", "rated_at": "2026-09-16T04:00:00Z"}))
	}
	expected := "analytics-seed"
	var batch map[string]any
	require.NoError(t, json.Unmarshal(importPayload(&expected, "many-applications", records), &batch))
	batch["complete_through"] = "2026-09-23T00:00:00Z"
	batch["history_start_date"] = "2026-08-01"
	batch["initial_backfill_complete"] = true
	raw, _ := json.Marshal(batch)
	job := applyImport(t, s, c, raw)
	require.Equal(t, "applied", job.Status, job.Errors)
	snapshot, err := s.CreateAnalysisContext(context.Background(), p, c.OrganizationID, Filters{Period: "week", TeamIDs: []string{"test:a"}})
	require.NoError(t, err)
	h := &Handler{service: s}
	r := gin.New()
	r.Use(Headers, func(c *gin.Context) { c.Set(principalKey, p); c.Next() })
	r.GET("/:organization_id/tokens", h.QueryAnalysis("tokens"))
	r.GET("/:organization_id/feedback", h.QueryAnalysis("feedback"))
	r.GET("/:organization_id/members/:member_id", h.QueryAnalysis("member"))
	request := func(path, cursor string) map[string]json.RawMessage {
		w := httptest.NewRecorder()
		target := "/" + c.OrganizationID + path + "?context_id=" + url.QueryEscape(snapshot.ID) + "&limit=100"
		if cursor != "" {
			target += "&cursor=" + url.QueryEscape(cursor)
		}
		r.ServeHTTP(w, httptest.NewRequest("GET", target, nil))
		require.Equal(t, 200, w.Code, w.Body.String())
		var result map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
		return result
	}
	for _, tc := range []struct{ path, list, summary string }{{"/tokens", "models", "totals"}, {"/feedback", "by_application", "rated"}, {"/members/test:one", "application_usage", "member"}} {
		first := request(tc.path, "")
		var cursor string
		require.NoError(t, json.Unmarshal(first["next_cursor"], &cursor))
		require.NotEmpty(t, cursor)
		second := request(tc.path, cursor)
		require.JSONEq(t, string(first[tc.summary]), string(second[tc.summary]))
		var a, b []json.RawMessage
		require.NoError(t, json.Unmarshal(first[tc.list], &a))
		require.NoError(t, json.Unmarshal(second[tc.list], &b))
		require.Len(t, a, 100)
		require.Len(t, b, 22)
		require.JSONEq(t, "null", string(second["next_cursor"]))
		if tc.path == "/members/test:one" {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/"+c.OrganizationID+"/members/test:two?context_id="+snapshot.ID+"&limit=100&cursor="+url.QueryEscape(cursor), nil))
			require.Equal(t, 400, w.Code)
		}
	}
}

func TestAnalysisUnavailableZeroAndPermissionAreDifferent(t *testing.T) {
	s, c, _, _ := connectorFixture(t)
	ctx := context.Background()
	var p Principal
	require.NoError(t, integrationDB.QueryRow(`SELECT m.id,m.user_id FROM bi_managers m JOIN bi_manager_grants g ON g.manager_id=m.id WHERE g.organization_id=$1`, c.OrganizationID).Scan(&p.ManagerID, &p.UserID))
	snapshot, err := s.CreateAnalysisContext(ctx, p, c.OrganizationID, Filters{Period: "week", TeamIDs: []string{}})
	require.NoError(t, err)
	state, err := s.analysisContext(ctx, p, c.OrganizationID, snapshot.ID, "analytics:read")
	require.NoError(t, err)
	f, err := s.loadAnalysis(ctx, state)
	require.NoError(t, err)
	overview, err := f.overview()
	require.NoError(t, err)
	require.Nil(t, overview.Stats.Requests)
	require.Nil(t, overview.Stats.Tokens.Total)
	require.Nil(t, overview.Assets)
	require.NoError(t, schemaOutput("Overview", overview))
	require.Equal(t, "unavailable", snapshot.Sources[0].Status)
	// A trustworthy empty heartbeat makes zero observable without creating people or calls.
	var batch map[string]any
	require.NoError(t, json.Unmarshal(importPayload(nil, "complete-empty", []any{}), &batch))
	batch["complete_through"] = s.now().UTC().Add(-time.Second).Format(time.RFC3339Nano)
	batch["history_start_date"] = "2020-01-01"
	batch["initial_backfill_complete"] = true
	raw, _ := json.Marshal(batch)
	job := applyImport(t, s, c, raw)
	require.Equal(t, "applied", job.Status, job.Errors)
	snapshot, err = s.CreateAnalysisContext(ctx, p, c.OrganizationID, Filters{Period: "week", TeamIDs: []string{}})
	require.NoError(t, err)
	state, err = s.analysisContext(ctx, p, c.OrganizationID, snapshot.ID, "analytics:read")
	require.NoError(t, err)
	f, err = s.loadAnalysis(ctx, state)
	require.NoError(t, err)
	stats := f.statistics(analysisSelection{}).stats
	require.EqualValues(t, 0, *stats.Requests)
	require.Equal(t, "0", *stats.Tokens.Total)
	require.Nil(t, stats.AdoptionRate.Value)
	require.Equal(t, "zero_denominator", *stats.AdoptionRate.Reason)
	require.Equal(t, "empty", snapshot.Status)
	_, err = s.analysisContext(ctx, p, c.OrganizationID, snapshot.ID, "knowledge:read")
	require.ErrorIs(t, err, ErrForbidden)
	h := &Handler{service: s}
	r := gin.New()
	r.Use(Headers, func(c *gin.Context) { c.Set(principalKey, p); c.Next() })
	r.POST("/:organization_id/analysis-contexts", h.CreateAnalysisContext)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/"+c.OrganizationID+"/analysis-contexts", strings.NewReader(`{"period":"week","team_ids":[],"application_id":null}`)))
	require.Equal(t, 400, w.Code)
}

func TestSecondaryMembershipFiltersPeopleWithoutChangingPrimaryStatistics(t *testing.T) {
	s, c, p, _ := analysisFixture(t)
	record := analyticRecord("membership", map[string]any{"id": "test:secondary", "member_id": "test:mover", "team_id": "test:a", "role_id": "test:role", "valid_from": "2026-09-17T00:00:00Z", "valid_to": nil, "primary": false})
	job := applyImport(t, s, c, completeBatch(t, "analytics-seed", "secondary-membership", []any{record}))
	require.Equal(t, "applied", job.Status, job.Errors)
	snapshot, err := s.CreateAnalysisContext(context.Background(), p, c.OrganizationID, Filters{Period: "week", TeamIDs: []string{}})
	require.NoError(t, err)
	state, err := s.analysisContext(context.Background(), p, c.OrganizationID, snapshot.ID, "members:read")
	require.NoError(t, err)
	f, err := s.loadAnalysis(context.Background(), state)
	require.NoError(t, err)
	require.EqualValues(t, 3, *f.statistics(analysisSelection{}).stats.EligibleMembers.Numerator)
	require.EqualValues(t, 2, *f.statistics(analysisSelection{TeamID: "test:a"}).stats.EligibleMembers.Numerator)
	rows, err := f.memberRows("", analysisSelection{TeamID: "test:a"})
	require.NoError(t, err)
	require.Len(t, rows, 3)
	for _, row := range rows {
		if row.ID == "test:mover" {
			require.Equal(t, "test:b", row.Team.ID)
			require.Equal(t, "300", *row.Tokens)
		}
	}
}
