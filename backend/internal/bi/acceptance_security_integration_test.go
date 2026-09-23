//go:build integration

package bi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAcceptanceP11CachedPagesRecheckSourceAndKnowledgeRestrictions(t *testing.T) {
	s, c, p, snapshot, source := contentFixture(t)
	h := &Handler{service: s}
	r := gin.New()
	r.Use(Headers, func(c *gin.Context) { c.Set(principalKey, p); c.Next() })
	r.GET("/:organization_id/search", h.QueryContent("search"))
	r.GET("/:organization_id/evaluations/:evaluation_id/samples", h.QueryContent("samples"))
	request := func(path, cursor string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		target := "/" + c.OrganizationID + path + "?context_id=" + url.QueryEscape(snapshot.ID) + "&limit=1&cursor=" + url.QueryEscape(cursor)
		r.ServeHTTP(w, httptest.NewRequest("GET", target, nil))
		return w
	}
	next := func(path string) string {
		w := request(path, "")
		require.Equal(t, 200, w.Code, w.Body.String())
		var page struct {
			Cursor string `json:"next_cursor"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
		require.NotEmpty(t, page.Cursor)
		return page.Cursor
	}
	searchCursor := next("/search")
	samplePath := "/evaluations/test:evaluation/samples"
	sampleCursor := next(samplePath)
	source["acl"] = map[string]any{"organization_readable": false, "team_ids": []string{}, "manager_ids": []string{}}
	job, err := s.SubmitImport(context.Background(), c, randomToken("idem_"), completeBatch(t, "content-seed", "page-source-restriction", []any{map[string]any{"kind": "source", "revision": 2, "payload": source}}))
	require.NoError(t, err)
	w, err := s.claimImport(context.Background())
	require.NoError(t, err)
	require.Equal(t, job.ID, w.ID)
	_, problems, err := s.stageImport(context.Background(), w)
	require.NoError(t, err)
	require.Empty(t, problems)
	denied := request(samplePath, sampleCursor)
	require.Equal(t, 403, denied.Code, denied.Body.String())
	require.Contains(t, denied.Body.String(), "CONTEXT_REVOKED")
	require.NotContains(t, denied.Body.String(), "Private question")
	require.NotContains(t, denied.Body.String(), "Another private")
	fresh := request(samplePath, "")
	require.Equal(t, 200, fresh.Code, fresh.Body.String())
	require.Contains(t, fresh.Body.String(), "Generic benchmark")
	require.NotContains(t, fresh.Body.String(), "Private question")
	require.NoError(t, s.processImport(context.Background(), w))
	knowledge := acceptancePayload(t, s, c, "knowledge", "test:knowledge")
	knowledge["acl"] = source["acl"]
	job = applyImport(t, s, c, completeBatch(t, "page-source-restriction", "page-knowledge-restriction", []any{map[string]any{"kind": "knowledge", "revision": 2, "payload": knowledge}}))
	require.Equal(t, "applied", job.Status, job.Errors)
	denied = request("/search", searchCursor)
	require.Equal(t, 403, denied.Code, denied.Body.String())
	require.NotContains(t, denied.Body.String(), "Approved knowledge")
	fresh = request("/search", "")
	require.Equal(t, 200, fresh.Code, fresh.Body.String())
	require.NotContains(t, fresh.Body.String(), "Approved knowledge")
}

func TestAcceptanceP12ConnectorCannotCrossSourceOrganizationOrManageGrants(t *testing.T) {
	s, primary, registration, _ := connectorFixture(t)
	ctx := context.Background()
	_, otherOrg, _, otherToken := connectorFixture(t)
	foreign := applyImport(t, s, otherOrg, importPayload(nil, "foreign", []any{}))
	require.Equal(t, "applied", foreign.Status)
	registration.SourceID, registration.Namespace = "directory", "hr"
	registration.AllowedKinds = []string{"team"}
	_, token, err := RegisterConnector(ctx, integrationDB, registration)
	require.NoError(t, err)
	directory, err := s.AuthorizeConnector(ctx, token)
	require.NoError(t, err)
	h := &Handler{service: s}
	r := gin.New()
	r.Use(Headers, h.ConnectorAuth)
	r.POST("/imports", h.SubmitImport)
	r.GET("/imports/:batch_id", h.GetImport)
	r.GET("/checkpoints/:source_id", h.GetCheckpoint)
	request := func(method, path, credential string, body []byte) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+credential)
		req.Header.Set("Idempotency-Key", randomToken("idem_"))
		r.ServeHTTP(w, req)
		return w
	}
	require.Equal(t, 403, request("POST", "/imports", token, importPayload(nil, "wrong-source", []any{})).Code)
	require.Equal(t, 404, request("GET", "/imports/"+foreign.ID, token, nil).Code)
	require.Equal(t, 404, request("GET", "/checkpoints/gateway", token, nil).Code)
	body := func(records []any) []byte {
		var batch map[string]any
		require.NoError(t, json.Unmarshal(importPayload(nil, "directory-first", records), &batch))
		batch["source_id"] = "directory"
		raw, err := json.Marshal(batch)
		require.NoError(t, err)
		return raw
	}
	role := analyticRecord("role", map[string]any{"id": "hr:role", "name": "Engineer", "status": "active"})
	require.Equal(t, 403, request("POST", "/imports", token, body([]any{role})).Code)
	grant := analyticRecord("manager_grant", map[string]any{"id": "hr:grant", "role": "org_admin"})
	require.Equal(t, 400, request("POST", "/imports", token, body([]any{grant})).Code)
	job := applyImport(t, s, directory, body([]any{analyticRecord("team", map[string]any{"id": "test:foreign-team", "name": "Forged team", "status": "active"})}))
	require.Equal(t, "rejected", job.Status)
	require.Equal(t, "FORBIDDEN_SOURCE", job.Errors[0].Code)
	cp, err := s.ImportCheckpoint(ctx, directory, directory.SourceID)
	require.NoError(t, err)
	require.Nil(t, cp.Checkpoint)
	job = applyImport(t, s, directory, body([]any{analyticRecord("team", map[string]any{"id": "hr:team", "name": "Directory team", "status": "active"})}))
	require.Equal(t, "applied", job.Status, job.Errors)
	require.Equal(t, 404, request("GET", "/imports/"+job.ID, otherToken, nil).Code)
	var organization, source string
	require.NoError(t, integrationDB.QueryRow(`SELECT organization_id,source_id FROM bi_entities WHERE organization_id=$1 AND kind='team' AND id='hr:team'`, primary.OrganizationID).Scan(&organization, &source))
	require.Equal(t, primary.OrganizationID, organization)
	require.Equal(t, "directory", source)
	registration.AllowedKinds = []string{"team", "member"}
	_, _, err = RegisterConnector(ctx, integrationDB, registration)
	require.ErrorContains(t, err, "CONFLICT")
}

func TestAcceptanceP18ConcurrentImportAdmissionHasOneBatch(t *testing.T) {
	for _, sameKey := range []bool{false, true} {
		name := "different_keys"
		if sameKey {
			name = "same_key"
		}
		t.Run(name, func(t *testing.T) {
			s, c, _, _ := connectorFixture(t)
			ctx := context.Background()
			const contenders = 8
			type outcome struct {
				job ImportJob
				err error
			}
			results := make(chan outcome, contenders)
			start := make(chan struct{})
			key := randomToken("idem_")
			var ready, done sync.WaitGroup
			ready.Add(contenders)
			done.Add(contenders)
			for i := 0; i < contenders; i++ {
				go func() {
					defer done.Done()
					candidate := key
					if !sameKey {
						candidate = randomToken("idem_")
					}
					ready.Done()
					<-start
					job, err := s.SubmitImport(ctx, c, candidate, importPayload(nil, "concurrent-first", []any{}))
					results <- outcome{job, err}
				}()
			}
			ready.Wait()
			close(start)
			done.Wait()
			close(results)
			accepted, busy := 0, 0
			ids := map[string]bool{}
			for result := range results {
				if result.err != nil {
					require.ErrorContains(t, result.err, "IMPORT_BUSY")
					busy++
					continue
				}
				accepted++
				ids[result.job.ID] = true
			}
			require.Len(t, ids, 1)
			if sameKey {
				require.Equal(t, contenders, accepted)
				require.Zero(t, busy)
			} else {
				require.Equal(t, 1, accepted)
				require.Equal(t, contenders-1, busy)
			}
			worked, err := s.ProcessNextImport(ctx)
			require.NoError(t, err)
			require.True(t, worked)
			var count int
			require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM bi_data_revisions WHERE organization_id=$1 AND status='published'`, c.OrganizationID).Scan(&count))
			require.Equal(t, 1, count)
			checkpoint, err := s.ImportCheckpoint(ctx, c, c.SourceID)
			require.NoError(t, err)
			require.Equal(t, "concurrent-first", *checkpoint.Checkpoint)
		})
	}
}
