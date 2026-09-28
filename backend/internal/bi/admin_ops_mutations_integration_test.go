//go:build integration

package bi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func adminMutationRequest(t *testing.T, r *gin.Engine, method, path, body string) []byte {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
	require.Equal(t, 200, w.Code, w.Body.String())
	return w.Body.Bytes()
}

func TestBIAdminRetentionAndBindingHTTPMutations(t *testing.T) {
	s, _, userIDs := authFixture(t)
	h := &Handler{service: s}
	r := gin.New()
	r.PUT("/retention", func(c *gin.Context) { h.AdminUpdateRetention(c, userIDs[0]) })
	r.POST("/identities/bindings/:binding_id/revoke", func(c *gin.Context) { h.AdminRevokeBinding(c, userIDs[0]) })
	adminMutationRequest(t, r, "PUT", "/retention", `{"report_months":24,"fact_months":24,"audit_days":180,"ephemeral_days":7}`)
	boundSession(t, s, userIDs[0])
	bindings, err := s.bindings(context.Background(), userIDs[0])
	require.NoError(t, err)
	require.Len(t, bindings, 1)
	adminMutationRequest(t, r, "POST", "/identities/bindings/"+bindings[0].ID+"/revoke", "")
}

func TestBIAdminCredentialHTTPMutations(t *testing.T) {
	s, connector, principal, _ := analysisFixture(t)
	h := &Handler{service: s}
	r := gin.New()
	r.POST("/sources", func(c *gin.Context) { h.AdminIssueCredential(c, principal.UserID) })
	r.POST("/credentials/:credential_id/rotate", func(c *gin.Context) { h.AdminRotateCredential(c, principal.UserID) })
	r.POST("/credentials/:credential_id/revoke", func(c *gin.Context) { h.AdminRevokeCredential(c, principal.UserID) })
	expires := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	body, err := json.Marshal(AdminSourceInput{OrganizationID: connector.OrganizationID, SourceID: connector.SourceID, Namespace: connector.Namespace, AllowedKinds: []string{"usage"}, ExpiresAt: time.Now().Add(time.Hour).UTC()})
	require.NoError(t, err)
	var response struct {
		Data struct {
			CredentialID string `json:"credential_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(adminMutationRequest(t, r, "POST", "/sources", string(body)), &response))
	require.NotEmpty(t, response.Data.CredentialID)
	require.NoError(t, json.Unmarshal(adminMutationRequest(t, r, "POST", "/credentials/"+response.Data.CredentialID+"/rotate", `{"expires_at":"`+expires+`"}`), &response))
	require.NotEmpty(t, response.Data.CredentialID)
	adminMutationRequest(t, r, "POST", "/credentials/"+response.Data.CredentialID+"/revoke", "")
}

func TestBIAdminRetryHTTPMutations(t *testing.T) {
	s, connector, principal, snapshot, _ := contentFixture(t)
	h := &Handler{service: s}
	r := gin.New()
	r.POST("/imports/:batch_id/retry", func(c *gin.Context) { h.AdminRetryImport(c, principal.UserID) })
	r.POST("/reports/:report_id/retry", func(c *gin.Context) { h.AdminRetryReport(c, principal.UserID) })
	bad := map[string]any{"kind": "membership", "revision": 1, "payload": map[string]any{
		"id": "test:retry-invalid", "member_id": "test:missing", "team_id": "test:missing-team", "role_id": "test:missing-role",
		"valid_from": "2026-09-01T00:00:00Z", "valid_to": nil, "primary": true,
	}}
	expected := "content-seed"
	job := applyImport(t, s, connector, importPayload(&expected, "retry-invalid", []any{bad}))
	require.Equal(t, "rejected", job.Status)
	adminMutationRequest(t, r, "POST", "/imports/"+job.ID+"/retry", "")

	report, err := s.CreateReport(context.Background(), principal, connector.OrganizationID, randomToken("idem_"), snapshot.ID, nil)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE bi_reports SET status='failed',failure_code='GENERATION_FAILED' WHERE id=$1`, report.ID)
	require.NoError(t, err)
	adminMutationRequest(t, r, "POST", "/reports/"+report.ID+"/retry", "")
}

func TestBIAdminCleanupHTTPMutation(t *testing.T) {
	s, _, userIDs := authFixture(t)
	s.now = func() time.Time { return time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC) }
	h := &Handler{service: s}
	r := gin.New()
	r.POST("/cleanup/run", func(c *gin.Context) { h.AdminRunCleanup(c, userIDs[0]) })
	adminMutationRequest(t, r, "POST", "/cleanup/run", "")
}
