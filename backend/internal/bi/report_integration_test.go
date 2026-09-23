//go:build integration

package bi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func readyReport(t *testing.T, s *Service, c Connector, p Principal, snapshot AnalysisContext) ReportSummary {
	t.Helper()
	summary, err := s.CreateReport(context.Background(), p, c.OrganizationID, randomToken("idem_"), snapshot.ID, nil)
	require.NoError(t, err)
	for i := 0; i < 20; i++ {
		worked, err := s.ProcessNextReport(context.Background())
		require.NoError(t, err)
		require.True(t, worked)
		detail, err := s.ReadReport(context.Background(), p, c.OrganizationID, summary.ID, "reports:read")
		require.NoError(t, err)
		if detail.Report.Status == "ready" {
			return detail.Report
		}
		if detail.Report.Status == "failed" {
			t.Fatalf("report failed: %+v", detail.Report)
		}
	}
	t.Fatal("report did not finish")
	return summary
}

func TestReportReceiptsAndArchiveSurviveContextExpiry(t *testing.T) {
	s, c, p, snapshot, _ := contentFixture(t)
	ctx := context.Background()
	key := randomToken("idem_")
	first, err := s.CreateReport(ctx, p, c.OrganizationID, key, snapshot.ID, ptr("管理简报"))
	require.NoError(t, err)
	replay, err := s.CreateReport(ctx, p, c.OrganizationID, key, snapshot.ID, ptr("管理简报"))
	require.NoError(t, err)
	require.Equal(t, first, replay)
	_, err = s.CreateReport(ctx, p, c.OrganizationID, key, snapshot.ID, ptr("Changed title"))
	require.ErrorContains(t, err, "IDEMPOTENCY_CONFLICT")
	worked, err := s.ProcessNextReport(ctx)
	require.NoError(t, err)
	require.True(t, worked)
	before, err := s.ReadReport(ctx, p, c.OrganizationID, first.ID, "reports:read")
	require.NoError(t, err)
	require.Equal(t, "ready", before.Report.Status)
	require.NotNil(t, before.Text)
	s.now = func() time.Time { return snapshot.ExpiresAt.Add(time.Minute) }
	replay, err = s.CreateReport(ctx, p, c.OrganizationID, key, snapshot.ID, ptr("管理简报"))
	require.NoError(t, err)
	require.Equal(t, first, replay)
	after, err := s.ReadReport(ctx, p, c.OrganizationID, first.ID, "reports:read")
	require.NoError(t, err)
	require.Equal(t, before.Text, after.Text)
	require.NoError(t, schemaOutput("ReportDetail", after))
	_, err = s.analysisContext(ctx, p, c.OrganizationID, snapshot.ID, "analytics:read")
	require.ErrorContains(t, err, "CONTEXT_EXPIRED")
	_, err = s.CleanupEphemeralState(ctx)
	require.NoError(t, err)
	_, err = s.analysisContext(ctx, p, c.OrganizationID, snapshot.ID, "analytics:read")
	require.ErrorContains(t, err, "CONTEXT_EXPIRED")
	foreign := p
	foreign.ManagerID = "another-manager"
	_, err = s.analysisContext(ctx, foreign, c.OrganizationID, snapshot.ID, "analytics:read")
	require.ErrorIs(t, err, ErrNotFound)
	archived, err := s.ReadReport(ctx, p, c.OrganizationID, first.ID, "reports:read")
	require.NoError(t, err)
	require.Equal(t, before.Text, archived.Text)
}

func TestShareChecksCreatorRecipientAndCurrentEvidence(t *testing.T) {
	s, c, p, snapshot, source := contentFixture(t)
	ctx := context.Background()
	report := readyReport(t, s, c, p, snapshot)
	var recipientUser int64
	require.NoError(t, integrationDB.QueryRow(`SELECT user_id FROM bi_managers WHERE user_id<>$1 AND id IN (SELECT manager_id FROM bi_security_events WHERE action='grant.save' AND target_id=$2) LIMIT 1`, p.UserID, p.ManagerID+":"+c.OrganizationID).Scan(&recipientUser))
	zero := int64(0)
	grant, err := s.SaveGrant(ctx, c.OrganizationID, GrantInput{UserID: recipientUser, Role: "viewer", AllTeams: true, TeamIDs: []string{}, Capabilities: []string{"reports:read", "knowledge:read", "sources:read"}, ExpectedRevision: &zero}, p.UserID, "recipient")
	require.NoError(t, err)
	recipient := Principal{ManagerID: grant.ManagerID, UserID: recipientUser}
	key := randomToken("idem_")
	share, err := s.CreateShare(ctx, p, c.OrganizationID, report.ID, key, 604800)
	require.NoError(t, err)
	replay, err := s.CreateShare(ctx, p, c.OrganizationID, report.ID, key, 604800)
	require.NoError(t, err)
	require.Equal(t, share, replay)
	_, err = s.CreateShare(ctx, p, c.OrganizationID, report.ID, key, 60)
	require.ErrorContains(t, err, "IDEMPOTENCY_CONFLICT")
	resolved, err := s.ResolveShare(ctx, recipient, share.ID)
	require.NoError(t, err)
	require.NoError(t, schemaOutput("SharedReport", resolved))
	require.False(t, resolved.Report.CanCopy)
	require.False(t, resolved.Report.CanShare)
	_, err = s.reportShares(ctx, recipient, c.OrganizationID, report.ID)
	require.ErrorIs(t, err, ErrForbidden)
	_, err = s.ResolveShare(ctx, Principal{ManagerID: "unrelated", UserID: 999999}, share.ID)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = integrationDB.Exec(`UPDATE bi_manager_grants SET capabilities=capabilities-'reports:share' WHERE manager_id=$1 AND organization_id=$2`, p.ManagerID, c.OrganizationID)
	require.NoError(t, err)
	_, err = s.ResolveShare(ctx, recipient, share.ID)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = s.ReadReport(ctx, recipient, c.OrganizationID, report.ID, "reports:read")
	require.NoError(t, err)
	caps, _ := json.Marshal(Capabilities)
	_, err = integrationDB.Exec(`UPDATE bi_manager_grants SET capabilities=$3 WHERE manager_id=$1 AND organization_id=$2`, p.ManagerID, c.OrganizationID, string(caps))
	require.NoError(t, err)
	source["acl"] = map[string]any{"organization_readable": false, "team_ids": []string{}, "manager_ids": []string{}}
	job := applyImport(t, s, c, completeBatch(t, "content-seed", "source-withdrawal", []any{map[string]any{"kind": "source", "revision": 2, "payload": source}}))
	require.Equal(t, "applied", job.Status, job.Errors)
	_, err = s.ReadReport(ctx, p, c.OrganizationID, report.ID, "reports:read")
	require.ErrorIs(t, err, ErrNotFound)
	_, err = s.ResolveShare(ctx, recipient, share.ID)
	require.ErrorIs(t, err, ErrNotFound)
	require.NoError(t, s.RevokeShare(ctx, p, c.OrganizationID, share.ID))
	require.NoError(t, s.RevokeShare(ctx, p, c.OrganizationID, share.ID))
}

func TestReportGenerationLeaseFencesAnInterruptedWorker(t *testing.T) {
	s, c, p, snapshot, _ := contentFixture(t)
	ctx := context.Background()
	summary, err := s.CreateReport(ctx, p, c.OrganizationID, randomToken("idem_"), snapshot.ID, nil)
	require.NoError(t, err)
	first, err := s.claimReport(ctx)
	require.NoError(t, err)
	require.Equal(t, summary.ID, first.ID)
	_, err = integrationDB.Exec(`UPDATE bi_reports SET lease_until=$2 WHERE id=$1`, summary.ID, s.now().Add(-time.Second))
	require.NoError(t, err)
	second, err := s.claimReport(ctx)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.ErrorIs(t, s.generateReport(ctx, first), errLeaseLost)
	require.NoError(t, s.generateReport(ctx, second))
	detail, err := s.ReadReport(ctx, p, c.OrganizationID, summary.ID, "reports:read")
	require.NoError(t, err)
	require.Equal(t, "ready", detail.Report.Status)
}

func TestReportHTTPResponseShapes(t *testing.T) {
	s, c, p, snapshot, _ := contentFixture(t)
	report := readyReport(t, s, c, p, snapshot)
	share, err := s.CreateShare(context.Background(), p, c.OrganizationID, report.ID, randomToken("idem_"), 60)
	require.NoError(t, err)
	h := &Handler{service: s}
	r := gin.New()
	r.Use(Headers, func(c *gin.Context) { c.Set(principalKey, p); c.Next() })
	r.GET("/:organization_id/reports", h.ListReports)
	r.GET("/:organization_id/reports/:report_id", h.GetReport)
	r.GET("/:organization_id/reports/:report_id/text", h.GetReportText)
	r.GET("/:organization_id/reports/:report_id/shares", h.ListShares)
	r.GET("/shares/:share_id", h.ResolveShare)
	for _, tc := range []struct{ path, schema string }{{"/" + c.OrganizationID + "/reports", "ReportSummaryPage"}, {"/" + c.OrganizationID + "/reports/" + report.ID, "ReportDetail"}, {"/" + c.OrganizationID + "/reports/" + report.ID + "/text", "ReportText"}, {"/" + c.OrganizationID + "/reports/" + report.ID + "/shares", "SharePage"}, {"/shares/" + share.ID, "SharedReport"}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		require.Equal(t, 200, w.Code, w.Body.String())
		require.NoError(t, validateSchema(tc.schema, w.Body.Bytes()), w.Body.String())
	}
}

func TestReportHistoricalReferenceSourceRevocation(t *testing.T) {
	s, c, p, _, source := contentFixture(t)
	ctx := context.Background()
	clone := func(value any) map[string]any {
		raw, err := json.Marshal(value)
		require.NoError(t, err)
		var copy map[string]any
		require.NoError(t, json.Unmarshal(raw, &copy))
		return copy
	}
	newSource := clone(source)
	newSource["id"] = "test:source-new"
	newSource["version_id"] = "test:source-new:v1"
	oldVersion, err := recordAt(ctx, s.db, c.OrganizationID, "knowledge_version", "test:knowledge:v1", -1)
	require.NoError(t, err)
	var version map[string]any
	require.NoError(t, json.Unmarshal(oldVersion.Raw, &version))
	version["id"] = "test:knowledge:v2"
	version["version"] = "2"
	version["source_ids"] = []string{"test:source-new"}
	oldKnowledge, err := recordAt(ctx, s.db, c.OrganizationID, "knowledge", "test:knowledge", -1)
	require.NoError(t, err)
	var knowledge map[string]any
	require.NoError(t, json.Unmarshal(oldKnowledge.Raw, &knowledge))
	knowledge["current_version_id"] = "test:knowledge:v2"
	job := applyImport(t, s, c, completeBatch(t, "content-seed", "new-current-version", []any{analyticRecord("source", newSource), analyticRecord("knowledge_version", version), map[string]any{"kind": "knowledge", "revision": 2, "payload": knowledge}}))
	require.Equal(t, "applied", job.Status, job.Errors)
	snapshot, err := s.CreateAnalysisContext(ctx, p, c.OrganizationID, Filters{Period: "week", TeamIDs: []string{"test:a"}})
	require.NoError(t, err)
	report := readyReport(t, s, c, p, snapshot)
	source["acl"] = map[string]any{"organization_readable": false, "team_ids": []string{}, "manager_ids": []string{}}
	job = applyImport(t, s, c, completeBatch(t, "new-current-version", "historical-source-revoked", []any{map[string]any{"kind": "source", "revision": 2, "payload": source}}))
	require.Equal(t, "applied", job.Status, job.Errors)
	_, err = s.ReadReport(ctx, p, c.OrganizationID, report.ID, "reports:read")
	require.ErrorIs(t, err, ErrNotFound)
}
