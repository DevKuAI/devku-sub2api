//go:build integration

package bi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/require"
)

func validateAdminSub2apiResponse(t *testing.T, raw []byte, schema string) {
	t.Helper()
	spec, err := os.ReadFile("../../../docs/mobile-bi/admin.openapi.json")
	require.NoError(t, err)
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(spec))
	require.NoError(t, err)
	compiler := jsonschema.NewCompiler()
	const uri = "https://schemas.devku.invalid/bi-admin-sync.json"
	require.NoError(t, compiler.AddResource(uri, document))
	contract, err := compiler.Compile(uri + "#/components/schemas/" + schema)
	require.NoError(t, err)
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	require.NoError(t, err)
	require.NoError(t, contract.Validate(value), string(raw))
}

func seedAdminGatewayUsage(t *testing.T, s *Service, c Connector) (int64, time.Time) {
	t.Helper()
	var orgID, userID, accountID, memberID, keyID int64
	require.NoError(t, integrationDB.QueryRow(`SELECT d.id,d.gateway_user_id FROM desktop_organizations d JOIN bi_organizations o ON o.desktop_organization_id=d.id WHERE o.id=$1`, c.OrganizationID).Scan(&orgID, &userID))
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO accounts(name,platform,type,credentials) VALUES($1,'openai','apikey','{}') RETURNING id`, randomToken("account_")).Scan(&accountID))
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO desktop_members(public_id,organization_id,name,name_normalized,phone) VALUES($1,$2,'Test','Test','+8613800000000') RETURNING id`, "mem_"+randomToken("")[:24], orgID).Scan(&memberID))
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO api_keys(user_id,key,name) VALUES($1,$2,'test') RETURNING id`, userID, randomToken("sk_")).Scan(&keyID))
	_, err := integrationDB.Exec(`INSERT INTO desktop_member_api_keys(member_id,api_key_id) VALUES($1,$2)`, memberID, keyID)
	require.NoError(t, err)
	when := s.now().UTC().Add(-time.Hour)
	_, err = integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,requested_model,input_tokens,output_tokens,created_at,billing_mode) VALUES($1,$2,$3,$4,'model','model',100,20,$5,'token')`, userID, keyID, accountID, randomToken("req_"), when)
	require.NoError(t, err)
	return userID, when.Add(-time.Hour)
}

func TestAdminSub2apiSyncHTTPImportsRealLogsAndDeduplicates(t *testing.T) {
	s, c, _, token := connectorFixture(t)
	actor, since := seedAdminGatewayUsage(t, s, c)
	// Logs from another enterprise must not be copied by this source.
	otherService, other, _, _ := connectorFixture(t)
	seedAdminGatewayUsage(t, otherService, other)
	h := &Handler{service: s}
	r := gin.New()
	r.POST("/sources/:source_id/sync-sub2api", func(c *gin.Context) { h.AdminSyncSub2api(c, actor) })
	input := AdminSub2apiSyncInput{OrganizationID: c.OrganizationID, ConnectorToken: token, Since: since, Limit: 500}
	body, err := json.Marshal(input)
	require.NoError(t, err)
	var response struct{ Data AdminSub2apiSyncResult }
	raw := adminMutationRequest(t, r, "POST", "/sources/"+c.SourceID+"/sync-sub2api", string(body))
	validateAdminSub2apiResponse(t, raw, "AdminSub2apiSyncResponse")
	require.NoError(t, json.Unmarshal(raw, &response))
	require.Equal(t, "queued", response.Data.Status)
	require.Equal(t, 1, response.Data.Records)
	require.NotNil(t, response.Data.Job)
	id := response.Data.Job.ID
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/sources/"+c.SourceID+"/sync-sub2api", strings.NewReader(string(body))))
	require.Equal(t, 409, w.Code)
	require.Contains(t, w.Body.String(), "IMPORT_BUSY")
	validateAdminSub2apiResponse(t, w.Body.Bytes(), "Error")
	for i := 0; i < 20; i++ {
		_, err := s.ProcessNextImport(context.Background())
		require.NoError(t, err)
		job, err := s.ImportJob(context.Background(), c, id)
		require.NoError(t, err)
		if job.Status == "applied" {
			break
		}
		require.NotEqual(t, "rejected", job.Status, job.Errors)
	}
	job, err := s.ImportJob(context.Background(), c, id)
	require.NoError(t, err)
	require.Equal(t, "applied", job.Status)
	raw = adminMutationRequest(t, r, "POST", "/sources/"+c.SourceID+"/sync-sub2api", string(body))
	validateAdminSub2apiResponse(t, raw, "AdminSub2apiSyncResponse")
	require.NoError(t, json.Unmarshal(raw, &response))
	require.Equal(t, "idle", response.Data.Status)
	require.Nil(t, response.Data.Job)
	var batches int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM bi_import_batches WHERE organization_id=$1 AND source_id=$2`, c.OrganizationID, c.SourceID).Scan(&batches))
	require.Equal(t, 1, batches)
	var audits string
	require.NoError(t, integrationDB.QueryRow(`SELECT json_agg(metadata)::text FROM bi_security_events WHERE organization_id=$1 AND action='source.sync_sub2api'`, c.OrganizationID).Scan(&audits))
	require.Contains(t, audits, id)
	require.Contains(t, audits, "idle")
	require.NotContains(t, audits, token)
}

func TestAdminSub2apiSyncRejectsInvalidCredentialsScopeAndRange(t *testing.T) {
	s, c, registration, token := connectorFixture(t)
	actor, since := seedAdminGatewayUsage(t, s, c)
	base := AdminSub2apiSyncInput{OrganizationID: c.OrganizationID, ConnectorToken: token, Since: since, Limit: 500}
	ctx := context.Background()
	for _, mutate := range []func(*AdminSub2apiSyncInput){
		func(i *AdminSub2apiSyncInput) { i.OrganizationID = "org_other" },
		func(i *AdminSub2apiSyncInput) { i.ConnectorToken = "invalid" },
		func(i *AdminSub2apiSyncInput) { i.Since = time.Time{} },
		func(i *AdminSub2apiSyncInput) { i.Since = s.now().Add(time.Hour) },
		func(i *AdminSub2apiSyncInput) { i.Limit = 501 },
	} {
		input := base
		mutate(&input)
		_, err := s.syncAdminSub2api(ctx, c.SourceID, input, actor, "invalid-input")
		require.Error(t, err)
	}
	_, err := s.syncAdminSub2api(ctx, "other_source", base, actor, "wrong-source")
	require.ErrorIs(t, err, ErrForbidden)
	registration.AllowedKinds = []string{"member"}
	_, memberToken, err := RegisterConnector(ctx, integrationDB, registration)
	require.NoError(t, err)
	input := base
	input.ConnectorToken = memberToken
	_, err = s.syncAdminSub2api(ctx, c.SourceID, input, actor, "wrong-kind")
	require.ErrorIs(t, err, ErrForbidden)
	_, err = integrationDB.Exec(`UPDATE bi_connector_sources SET complete_through=$3 WHERE organization_id=$1 AND source_id=$2`, c.OrganizationID, c.SourceID, s.now())
	require.NoError(t, err)
	_, err = s.syncAdminSub2api(ctx, c.SourceID, base, actor, "authoritative-source")
	require.Error(t, err)
	_, err = integrationDB.Exec(`UPDATE bi_connector_credentials SET revoked_at=NOW() WHERE id=$1`, c.ID)
	require.NoError(t, err)
	_, err = s.syncAdminSub2api(ctx, c.SourceID, base, actor, "revoked")
	require.Error(t, err)
	_, err = integrationDB.Exec(`UPDATE bi_connector_credentials SET revoked_at=NULL,expires_at=NOW()-INTERVAL '1 second' WHERE id=$1`, c.ID)
	require.NoError(t, err)
	h := &Handler{service: s}
	r := gin.New()
	r.POST("/sources/:source_id/sync-sub2api", func(c *gin.Context) { h.AdminSyncSub2api(c, actor) })
	body, err := json.Marshal(base)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/sources/"+c.SourceID+"/sync-sub2api", strings.NewReader(string(body))))
	// Connector expiry must not be reported as an expired administrator login.
	require.Equal(t, 400, w.Code, w.Body.String())
	validateAdminSub2apiResponse(t, w.Body.Bytes(), "Error")
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM bi_import_batches WHERE organization_id=$1`, c.OrganizationID).Scan(&count))
	require.Zero(t, count)
}

func TestAdminSub2apiSyncRollsBackBatchOnAuditFailure(t *testing.T) {
	s, c, _, token := connectorFixture(t)
	_, since := seedAdminGatewayUsage(t, s, c)
	_, err := s.syncAdminSub2api(context.Background(), c.SourceID, AdminSub2apiSyncInput{OrganizationID: c.OrganizationID, ConnectorToken: token, Since: since, Limit: 500}, -1, "audit-failure")
	require.Error(t, err)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM bi_import_batches WHERE organization_id=$1`, c.OrganizationID).Scan(&count))
	require.Zero(t, count)
}
