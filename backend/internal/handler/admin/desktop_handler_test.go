package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDesktopAdminProvisioningUsesIndependentIdempotencyScopes(t *testing.T) {
	scopes := []string{
		desktopOrganizationCreateIdempotencyScope,
		desktopMemberCreateIdempotencyScope,
		desktopModelTokenRotateIdempotencyScope,
	}

	require.Equal(t, []string{
		"admin.desktop.organizations.create",
		"admin.desktop.members.create",
		"admin.desktop.model_tokens.rotate",
	}, scopes)
	require.Len(t, map[string]struct{}{scopes[0]: {}, scopes[1]: {}, scopes[2]: {}}, 3)
}

func TestDesktopAdminConflictReasonsUseStableEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		reason string
		err    error
	}{
		{reason: "GATEWAY_USER_ALREADY_ASSIGNED", err: service.ErrDesktopGatewayUserAssigned},
		{reason: "ORGANIZATION_PROVISIONING_LOCKED", err: service.ErrDesktopProvisioningLocked},
		{reason: "ORGANIZATION_DISABLED", err: service.ErrDesktopOrganizationDisabled},
		{reason: "MEMBER_DISABLED", err: service.ErrDesktopMemberDisabled},
		{reason: "MEMBER_LIMIT_REACHED", err: service.ErrDesktopMemberLimitReached},
		{reason: "MEMBER_LIMIT_BELOW_CURRENT_COUNT", err: service.ErrDesktopMemberLimitTooLow},
		{reason: "DESKTOP_MANAGED_API_KEY", err: service.ErrDesktopManagedAPIKey},
		{reason: "MODEL_TOKEN_ROTATION_CONFLICT", err: service.ErrDesktopRotationConflict},
		{reason: "DESKTOP_ORGANIZATION_DEPENDENCY", err: service.ErrDesktopDependency},
	}

	for _, test := range tests {
		t.Run(test.reason, func(t *testing.T) {
			router := gin.New()
			router.GET("/conflict", func(c *gin.Context) { response.ErrorFrom(c, test.err) })
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/conflict", nil))

			require.Equal(t, http.StatusConflict, recorder.Code)
			var envelope struct {
				Code   int    `json:"code"`
				Reason string `json:"reason"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
			require.Equal(t, http.StatusConflict, envelope.Code)
			require.Equal(t, test.reason, envelope.Reason)
		})
	}
}

func TestDesktopAdminBodyLimitReturnsAdminErrorEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.DesktopAdminBodyLimit())
	router.POST("/api/v1/admin/desktop/organizations", NewDesktopHandler(nil).CreateOrganization)

	body := `{"name":"` + strings.Repeat("x", 17*1024) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/desktop/organizations", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	var envelope struct {
		Code   int    `json:"code"`
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, http.StatusRequestEntityTooLarge, envelope.Code)
	require.Equal(t, "PAYLOAD_TOO_LARGE", envelope.Reason)
}

func TestDesktopAdminMemberDTOExposesFullPhone(t *testing.T) {
	payload, err := json.Marshal(desktopMemberFromService(&service.DesktopMember{
		PublicID: "mem_one",
		Name:     "Member",
		Phone:    "+8613800000000",
		Status:   service.DesktopStatusActive,
	}))
	require.NoError(t, err)
	require.JSONEq(t, `{
		"public_id":"mem_one",
		"name":"Member",
		"phone":"+8613800000000",
		"status":"active",
		"model_token_status":"missing",
		"created_at":"0001-01-01T00:00:00Z",
		"updated_at":"0001-01-01T00:00:00Z"
	}`, string(payload))
	require.NotContains(t, string(payload), "masked_phone")
}

func TestDesktopAdminMemberDTOExposesUsage(t *testing.T) {
	usage := &service.DesktopMemberUsage{
		TodayTokens: 100, Last30DaysTokens: 900, TotalTokens: 1200,
		TodayActualCost: 0.1, Last30DaysActualCost: 0.9, TotalActualCost: 1.2,
	}
	dto := desktopMemberFromService(&service.DesktopMember{Usage: usage})

	require.Same(t, usage, dto.Usage)
	payload, err := json.Marshal(dto)
	require.NoError(t, err)
	require.Contains(t, string(payload), `"last_30_days_tokens":900`)
	require.Contains(t, string(payload), `"total_actual_cost":1.2`)
}

func TestDesktopAdminOrganizationDTOExposesMemberLimit(t *testing.T) {
	payload, err := json.Marshal(desktopOrganizationFromService(&service.DesktopOrganization{MemberCount: 2, MemberLimit: 10}, false))
	require.NoError(t, err)
	require.Contains(t, string(payload), `"member_count":2`)
	require.Contains(t, string(payload), `"member_limit":10`)
}

func TestDesktopAdminRejectsInvalidMemberLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, method := range []string{http.MethodPost, http.MethodPatch} {
		for _, limit := range []string{"0", "-1", "1.5"} {
			t.Run(method+"/"+limit, func(t *testing.T) {
				router := gin.New()
				handler := NewDesktopHandler(nil)
				router.POST("/organizations", handler.CreateOrganization)
				router.PATCH("/organizations", handler.UpdateOrganization)
				request := httptest.NewRequest(method, "/organizations", strings.NewReader(`{"name":"Test","code":"test","gateway_user_id":1,"group_id":1,"member_limit":`+limit+`}`))
				request.Header.Set("Content-Type", "application/json")
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, request)
				require.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
			})
		}
	}
}

func TestDesktopAdminConversationReportingContract(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		raw, err := json.Marshal(map[string]any{"conversation_reporting_enabled": enabled})
		require.NoError(t, err)
		var create desktopCreateOrganizationRequest
		require.NoError(t, json.Unmarshal(raw, &create))
		require.Equal(t, enabled, create.ConversationReportingEnabled)
		var update desktopUpdateOrganizationRequest
		require.NoError(t, json.Unmarshal(raw, &update))
		require.NotNil(t, update.ConversationReportingEnabled)
		require.Equal(t, enabled, *update.ConversationReportingEnabled)
		output, err := json.Marshal(desktopOrganizationFromService(&service.DesktopOrganization{ConversationReportingEnabled: enabled}, true))
		require.NoError(t, err)
		var fields map[string]any
		require.NoError(t, json.Unmarshal(output, &fields))
		require.Equal(t, enabled, fields["conversation_reporting_enabled"])
	}
	var omitted desktopUpdateOrganizationRequest
	require.NoError(t, json.Unmarshal([]byte(`{}`), &omitted))
	require.Nil(t, omitted.ConversationReportingEnabled)
}
