//go:build integration

package bi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBIPersonalContentHTTPFlow(t *testing.T) {
	s, connector, p, _, _ := contentFixture(t)
	h := &Handler{service: s}
	r := gin.New()
	r.Use(Headers, func(c *gin.Context) { c.Set(principalKey, p); c.Next() })
	base := "/organizations/:organization_id/me"
	r.PUT(base+"/favorites/:knowledge_id", h.PutFavorite)
	r.DELETE(base+"/favorites/:knowledge_id", h.DeleteFavorite)
	r.GET(base+"/knowledge-notes/:knowledge_id", h.GetNote)
	r.PUT(base+"/knowledge-notes/:knowledge_id", h.PutNote)
	r.DELETE(base+"/knowledge-notes/:knowledge_id", h.DeleteNote)

	request := func(method, path, body, header, value string, status int) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if header != "" {
			req.Header.Set(header, value)
		}
		r.ServeHTTP(w, req)
		require.Equal(t, status, w.Code, w.Body.String())
		return w
	}
	path := "/organizations/" + connector.OrganizationID + "/me"
	request("PUT", path+"/favorites/test:knowledge", "", "", "", 200)
	notePath := path + "/knowledge-notes/test:knowledge"
	created := request("PUT", notePath, `{"text":"Review this source"}`, "If-None-Match", "*", 200)
	require.NotEmpty(t, created.Header().Get("ETag"))
	request("GET", notePath, "", "", "", 200)
	request("DELETE", notePath, "", "If-Match", created.Header().Get("ETag"), 204)
	request("DELETE", path+"/favorites/test:knowledge", "", "", "", 204)
}

func TestBIReportMutationHTTPFlow(t *testing.T) {
	s, connector, p, snapshot, _ := contentFixture(t)
	h := &Handler{service: s}
	r := gin.New()
	r.Use(Headers, func(c *gin.Context) { c.Set(principalKey, p); c.Next() })
	r.POST("/organizations/:organization_id/reports", h.CreateReport)
	r.POST("/organizations/:organization_id/reports/:report_id/shares", h.CreateShare)
	r.DELETE("/organizations/:organization_id/shares/:share_id", h.DeleteShare)

	request := func(method, path, body string, status int) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Idempotency-Key", randomToken("idem_"))
		r.ServeHTTP(w, req)
		require.Equal(t, status, w.Code, w.Body.String())
		return w
	}
	path := "/organizations/" + connector.OrganizationID
	var report ReportSummary
	require.NoError(t, json.Unmarshal(request("POST", path+"/reports", `{"context_id":"`+snapshot.ID+`"}`, 202).Body.Bytes(), &report))
	require.NotEmpty(t, report.ID)
	worked, err := s.ProcessNextReport(context.Background())
	require.NoError(t, err)
	require.True(t, worked)
	var share Share
	require.NoError(t, json.Unmarshal(request("POST", path+"/reports/"+report.ID+"/shares", `{"expires_in_seconds":60}`, 201).Body.Bytes(), &share))
	require.NotEmpty(t, share.ID)
	request("DELETE", path+"/shares/"+share.ID, "", 204)
}
