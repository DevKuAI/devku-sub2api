package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

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
	windows        service.DesktopUsageWindows
	err            error
}

func (r *organizationUsageHTTPRepository) GetDesktopOrganizationUsage(_ context.Context, organizationID int64, windows service.DesktopUsageWindows) (*service.DesktopOrganizationUsageStatistics, error) {
	r.organizationID = organizationID
	r.windows = windows
	return &service.DesktopOrganizationUsageStatistics{
		Total:    service.DesktopOrganizationUsagePeriod{TotalTokens: 1234, ActualCost: 1.25},
		Selected: service.DesktopOrganizationUsagePeriod{TotalTokens: 234, ActualCost: 0.25},
		Previous: service.DesktopOrganizationUsagePeriod{TotalTokens: 123, ActualCost: 0.15},
		Models:   []service.DesktopUsageModel{{Model: "model-one", CostRank: 2, TokenRank: 1}},
		MemberModels: []service.DesktopUsageMemberModel{{
			MemberID: "mem_one", Name: "Member One", Deleted: true, Model: "model-one", Requests: 2, TotalTokens: 234,
			DesktopUsageBreakdown: service.DesktopUsageBreakdown{InputTokens: 10, OutputTokens: 20, CacheCreationTokens: 30, CacheReadTokens: 174},
		}},
	}, r.err
}

func TestDesktopOrganizationUsageStatisticsHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name                                            string
		managed, authenticated, missing, storageFailure bool
		query                                           string
		want                                            int
	}{
		{name: "admin", authenticated: true, want: 200},
		{name: "managed without conversation reporting", managed: true, authenticated: true, want: 200},
		{name: "admin selected period", authenticated: true, query: "&days=7", want: 200},
		{name: "managed custom period", managed: true, authenticated: true, query: "&from=2020-01-01&to=2020-01-07", want: 200},
		{name: "invalid selected period", managed: true, authenticated: true, query: "&days=8", want: 422},
		{name: "mixed selected periods", authenticated: true, query: "&days=7&from=2020-01-01&to=2020-01-07", want: 422},
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
			router.ServeHTTP(result, httptest.NewRequest(http.MethodGet, path+"?organization_id=org_other&page=9&search=nobody"+tc.query, nil))
			require.Equal(t, tc.want, result.Code, result.Body.String())
			require.Equal(t, "no-store", result.Header().Get("Cache-Control"))
			if tc.want == 200 {
				require.Contains(t, result.Body.String(), `"total_tokens":1234`)
				require.Contains(t, result.Body.String(), `"actual_cost":1.25`)
				require.Contains(t, result.Body.String(), `"selected":{"total_tokens":234,"actual_cost":0.25}`)
				require.Contains(t, result.Body.String(), `"token_rank":1`)
				require.Contains(t, result.Body.String(), `"member_models":[{"member_id":"mem_one","name":"Member One","deleted":true,"model":"model-one","requests":2,"total_tokens":234,"input_tokens":10,"output_tokens":20,"cache_creation_tokens":30,"cache_read_tokens":174}]`)
				require.EqualValues(t, 7, usage.organizationID)
				if tc.query == "&days=7" {
					require.Equal(t, 7, usage.windows.Selected.Days)
				}
				if tc.query == "&from=2020-01-01&to=2020-01-07" {
					require.Equal(t, "2020-01-01", usage.windows.Selected.StartDate)
				}
				if tc.managed {
					require.EqualValues(t, 42, identity.scopedUserID)
				}
			} else if !tc.storageFailure {
				require.Zero(t, usage.organizationID)
			}
		})
	}
}
