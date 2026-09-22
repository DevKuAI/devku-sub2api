package handler_test

import (
	"context"
	"encoding/json"
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

type conversationStatisticsIdentity struct {
	service.DesktopRepository
	organization *service.DesktopOrganization
	scopedUserID int64
	err          error
}

func (r *conversationStatisticsIdentity) ScopedToGatewayUser(id int64) service.DesktopRepository {
	r.scopedUserID = id
	return r
}
func (r *conversationStatisticsIdentity) GetOrganizationForGatewayUser(context.Context, int64) (*service.DesktopOrganization, error) {
	return r.organization, r.err
}
func (r *conversationStatisticsIdentity) GetOrganization(context.Context, string) (*service.DesktopOrganization, error) {
	return r.organization, r.err
}

type conversationStatisticsRecords struct {
	service.DesktopConversationRepository
	calls          int
	filters        service.DesktopConversationFilters
	organizationID int64
	err            error
}

func (r *conversationStatisticsRecords) Statistics(_ context.Context, organizationID int64, filters service.DesktopConversationFilters, _ service.DesktopConversationPeriods) (*service.DesktopConversationStatistics, error) {
	r.calls++
	r.organizationID, r.filters = organizationID, filters
	return &service.DesktopConversationStatistics{Total: service.DesktopConversationCounts{RecordCount: 123, PromptCount: 456}}, r.err
}

func TestDesktopConversationStatisticsHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name                              string
		managed, authenticated, reporting bool
		query                             string
		want                              int
		storageErr                        bool
	}{
		{"admin", false, true, false, "?client=workbuddy&member_search=Member", 200, false},
		{"managed", true, true, true, "?organization_id=org_other&client=workbuddy", 200, false},
		{"unauthenticated", true, false, true, "", 401, false},
		{"reporting disabled", true, true, false, "", 403, false},
		{"invalid UUID", false, true, true, "?record_id=invalid", 422, false},
		{"invalid time", true, true, true, "?received_from=invalid", 422, false},
		{"reversed range", false, true, true, "?received_from=2026-09-22T00:00:00Z&received_to=2026-09-21T00:00:00Z", 422, false},
		{"storage unavailable", false, true, true, "", 503, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			identity := &conversationStatisticsIdentity{organization: &service.DesktopOrganization{ID: 7, PublicID: "org_owned", ConversationReportingEnabled: tc.reporting}}
			records := &conversationStatisticsRecords{}
			if tc.storageErr {
				records.err = service.ErrDesktopConversationStorage
			}
			svc := service.NewDesktopService(identity, nil, nil, nil, nil, nil, &config.Config{}, nil, records, nil)
			router := gin.New()
			router.Use(func(c *gin.Context) {
				if tc.authenticated {
					c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
				}
				c.Next()
			})
			path := "/api/v1/admin/desktop/organizations/org_owned/conversation-records/statistics"
			if tc.managed {
				path = "/api/v1/desktop/organization/conversation-records/statistics"
				router.GET(path, handler.NewDesktopHandler(svc).ManagedConversationStatistics)
			} else {
				router.GET("/api/v1/admin/desktop/organizations/:organization_id/conversation-records/statistics", adminhandler.NewDesktopHandler(svc).ConversationStatistics)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path+tc.query, nil))
			require.Equal(t, tc.want, response.Code, response.Body.String())
			require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
			var envelope struct {
				Code int                                   `json:"code"`
				Data service.DesktopConversationStatistics `json:"data"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
			if tc.want == 200 {
				require.Zero(t, envelope.Code)
				require.EqualValues(t, 123, envelope.Data.Total.RecordCount)
				require.EqualValues(t, 456, envelope.Data.Total.PromptCount)
				require.NotEmpty(t, envelope.Data.Timezone)
				require.False(t, envelope.Data.AsOf.IsZero())
				require.EqualValues(t, 7, records.organizationID)
				require.Equal(t, "workbuddy", records.filters.Client)
				if tc.managed {
					require.EqualValues(t, 42, identity.scopedUserID)
				}
			} else if !tc.storageErr {
				require.Zero(t, records.calls)
			}
		})
	}
}
