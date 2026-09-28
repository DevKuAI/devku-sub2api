//go:build integration

package bi

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBIAdminReadQueriesExecuteWithAndWithoutFilters(t *testing.T) {
	ctx := context.Background()
	s := &Service{db: integrationDB, now: time.Now}
	_, err := s.adminOverview(ctx)
	require.NoError(t, err)

	checks := []struct {
		name string
		run  func(bool) error
	}{
		{"bindings", func(filtered bool) error {
			status := ""
			if filtered {
				status = "active"
			}
			_, _, err := s.listAdminBindings(ctx, 1, 20, status)
			return err
		}},
		{"challenges", func(filtered bool) error {
			status := ""
			if filtered {
				status = "pending"
			}
			_, _, err := s.listAdminChallenges(ctx, 1, 20, status)
			return err
		}},
		{"sessions", func(filtered bool) error {
			status := ""
			if filtered {
				status = "active"
			}
			_, _, err := s.listAdminSessions(ctx, 1, 20, status)
			return err
		}},
		{"sources", func(filtered bool) error {
			org, source, namespace := "", "", ""
			if filtered {
				org, source, namespace = "missing", "missing", "missing"
			}
			_, _, err := s.listAdminSources(ctx, 1, 20, org, source, namespace)
			return err
		}},
		{"credentials", func(filtered bool) error {
			source, org := "", ""
			if filtered {
				source, org = "missing", "missing"
			}
			_, _, err := s.listAdminCredentialsPage(ctx, source, org, 1, 20)
			return err
		}},
		{"imports", func(filtered bool) error {
			org, source, status := "", "", ""
			if filtered {
				org, source, status = "missing", "missing", "queued"
			}
			_, _, err := s.listAdminImports(ctx, 1, 20, org, source, status)
			return err
		}},
		{"quality", func(filtered bool) error {
			org := ""
			if filtered {
				org = "missing"
			}
			_, err := s.listAdminQuality(ctx, org)
			return err
		}},
		{"reports", func(filtered bool) error {
			org, status := "", ""
			if filtered {
				org, status = "missing", "queued"
			}
			_, _, err := s.listAdminReports(ctx, 1, 20, org, status)
			return err
		}},
		{"audit", func(filtered bool) error {
			action, requestID := "", ""
			if filtered {
				action, requestID = "missing", "missing"
			}
			_, _, err := s.listAdminAudit(ctx, 1, 20, action, requestID)
			return err
		}},
	}
	for _, check := range checks {
		for _, filtered := range []bool{false, true} {
			name := check.name + "/all"
			if filtered {
				name = check.name + "/filtered"
			}
			t.Run(name, func(t *testing.T) { require.NoError(t, check.run(filtered)) })
		}
	}
}

func TestBIAdminReadHTTPHandlers(t *testing.T) {
	h := &Handler{service: &Service{db: integrationDB, now: time.Now}}
	r := gin.New()
	r.GET("/overview", h.AdminOverview)
	r.GET("/retention", h.AdminGetRetention)
	r.GET("/cleanup", h.AdminCleanupStatus)
	r.GET("/identities/bindings", h.AdminListBindings)
	r.GET("/identities/challenges", h.AdminListChallenges)
	r.GET("/identities/sessions", h.AdminListSessions)
	r.GET("/sources", h.AdminListSources)
	r.GET("/sources/:source_id/credentials", h.AdminListCredentials)
	r.GET("/imports", h.AdminListImports)
	r.GET("/quality", h.AdminListQuality)
	r.GET("/reports", h.AdminListReports)
	r.GET("/audit", h.AdminListAudit)

	for _, path := range []string{
		"/overview", "/retention", "/cleanup",
		"/identities/bindings", "/identities/bindings?status=active",
		"/identities/challenges", "/identities/challenges?status=pending",
		"/identities/sessions", "/identities/sessions?status=active",
		"/sources", "/sources?organization_id=missing&source_id=missing&namespace=missing",
		"/sources/missing/credentials?organization_id=missing",
		"/imports", "/imports?organization_id=missing&source_id=missing&status=queued",
		"/quality", "/quality?organization_id=missing",
		"/reports", "/reports?organization_id=missing&status=queued",
		"/audit", "/audit?action=missing&request_id=missing",
	} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
			require.Equal(t, 200, w.Code, w.Body.String())
		})
	}
}

func TestBIAdminArchiveReadyReport(t *testing.T) {
	s, connector, principal, snapshot, _ := contentFixture(t)
	report := readyReport(t, s, connector, principal, snapshot)
	h := &Handler{service: s}
	r := gin.New()
	r.POST("/reports/:report_id/archive", func(c *gin.Context) { h.AdminArchiveReport(c, principal.UserID) })
	archive := func(id string) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/reports/"+id+"/archive", nil))
		require.Equal(t, 200, w.Code, w.Body.String())
	}
	archive(report.ID)
	stored, err := readStoredReport(context.Background(), integrationDB, connector.OrganizationID, report.ID)
	require.NoError(t, err)
	require.Equal(t, "archived", stored.Summary.Status)
	require.NotNil(t, stored.Text)
	require.NotNil(t, stored.GeneratedAt)
	queued, err := s.CreateReport(context.Background(), principal, connector.OrganizationID, randomToken("idem_"), snapshot.ID, nil)
	require.NoError(t, err)
	archive(queued.ID)
	stored, err = readStoredReport(context.Background(), integrationDB, connector.OrganizationID, queued.ID)
	require.NoError(t, err)
	require.Equal(t, "archived", stored.Summary.Status)
	require.Nil(t, stored.Text)
	require.Nil(t, stored.GeneratedAt)
}
