package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type organizationUsageHTTPRepository struct {
	service.DesktopUsageRepository
	organizationID int64
	err            error
}

func (r *organizationUsageHTTPRepository) GetDesktopOrganizationUsage(_ context.Context, organizationID int64, _, _, _, _ time.Time) (*service.DesktopOrganizationUsageStatistics, error) {
	r.organizationID = organizationID
	return &service.DesktopOrganizationUsageStatistics{Total: service.DesktopOrganizationUsagePeriod{TotalTokens: 1234, ActualCost: 1.25}}, r.err
}

func TestDesktopOrganizationUsageStatisticsHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name                                            string
		managed, authenticated, missing, storageFailure bool
		want                                            int
	}{
		{name: "admin", authenticated: true, want: 200},
		{name: "managed without conversation reporting", managed: true, authenticated: true, want: 200},
		{name: "unauthenticated", managed: true, want: 401},
		{name: "organization unavailable", managed: true, authenticated: true, missing: true, want: 404},
		{name: "admin organization unavailable", authenticated: true, missing: true, want: 404},
		{name: "storage unavailable", authenticated: true, storageFailure: true, want: 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			identity := &conversationStatisticsIdentity{organization: &service.DesktopOrganization{ID: 7, PublicID: "org_owned", ConversationReportingEnabled: false}}
			if tc.missing {
				identity.err = service.ErrDesktopOrganizationNotFound
			}
			usage := &organizationUsageHTTPRepository{}
			if tc.storageFailure {
				usage.err = errors.New("offline")
			}
			svc := service.NewDesktopService(identity, usage, nil, nil, nil, nil, &config.Config{}, nil, nil, nil)
			router := gin.New()
			router.Use(func(c *gin.Context) {
				if tc.authenticated {
					c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
				}
			})
			path := "/api/v1/admin/desktop/organizations/org_owned/usage/statistics"
			if tc.managed {
				path = "/api/v1/desktop/organization/usage/statistics"
				router.GET(path, handler.NewDesktopHandler(svc).ManagedOrganizationUsageStatistics)
			} else {
				router.GET("/api/v1/admin/desktop/organizations/:organization_id/usage/statistics", adminhandler.NewDesktopHandler(svc).OrganizationUsageStatistics)
			}
			result := httptest.NewRecorder()
			router.ServeHTTP(result, httptest.NewRequest(http.MethodGet, path+"?organization_id=org_other&page=9&search=nobody", nil))
			require.Equal(t, tc.want, result.Code, result.Body.String())
			require.Equal(t, "no-store", result.Header().Get("Cache-Control"))
			if tc.want == 200 {
				require.Contains(t, result.Body.String(), `"total_tokens":1234`)
				require.Contains(t, result.Body.String(), `"actual_cost":1.25`)
				require.EqualValues(t, 7, usage.organizationID)
				if tc.managed {
					require.EqualValues(t, 42, identity.scopedUserID)
				}
			} else if !tc.storageFailure {
				require.Zero(t, usage.organizationID)
			}
		})
	}
}
