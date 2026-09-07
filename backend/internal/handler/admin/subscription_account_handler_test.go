//go:build unit

package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func subscriptionActionRequest(method, id string, userID int64, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if userID > 0 {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID})
		}
		c.Next()
	})
	router.Handle(method, "/subscription-accounts/:id/action", handler)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(method, "/subscription-accounts/"+id+"/action?user_id=123", nil))
	return recorder
}

func boundOpenAISubscription() *service.Account {
	return &service.Account{
		ID: 42, BoundUserID: int64Pointer(123), Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "private-token"},
		Extra:       map[string]any{"workspace_id": "private-workspace", "codex_5h_used_percent": 13.0, "codex_7d_used_percent": 35.0},
	}
}

func TestSubscriptionAccountActionsRequireBinding(t *testing.T) {
	for _, test := range []struct {
		name     string
		userID   int64
		id       string
		unbound  bool
		platform string
		err      error
		want     int
	}{
		{name: "unauthenticated", id: "42", want: http.StatusUnauthorized},
		{name: "invalid_id", userID: 123, id: "0", want: http.StatusBadRequest},
		{name: "other_user", userID: 456, id: "42", want: http.StatusNotFound},
		{name: "unbound", userID: 123, id: "42", unbound: true, want: http.StatusNotFound},
		{name: "unsupported_platform", userID: 123, id: "42", platform: service.PlatformAnthropic, want: http.StatusBadRequest},
		{name: "missing", userID: 123, id: "42", err: service.ErrAccountNotFound, want: http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			account := boundOpenAISubscription()
			if test.unbound {
				account.BoundUserID = nil
			}
			if test.platform != "" {
				account.Platform = test.platform
			}
			admin := &openAIResetAdminServiceStub{account: account, err: test.err}
			quota := successfulOpenAIQuotaWorkflowStub()
			oauth := &OpenAIOAuthHandler{adminService: admin, quotaService: quota}
			accounts := &AccountHandler{adminService: admin}
			for _, action := range []struct {
				method  string
				handler gin.HandlerFunc
			}{
				{http.MethodGet, accounts.GetMySubscriptionAccountUsage},
				{http.MethodPost, accounts.RefreshMySubscriptionAccountUsage},
				{http.MethodPost, oauth.RefreshMySubscriptionQuota},
				{http.MethodPost, oauth.ResetMySubscriptionQuota},
			} {
				recorder := subscriptionActionRequest(action.method, test.id, test.userID, action.handler)
				require.Equal(t, test.want, recorder.Code, recorder.Body.String())
			}
			require.Zero(t, quota.queryCalls)
			require.Zero(t, quota.resetCalls)
			require.Zero(t, quota.cacheCalls)
		})
	}
}

func TestSubscriptionQuotaRefreshReturnsOnlyCreditsAndExpiry(t *testing.T) {
	quota := successfulOpenAIQuotaWorkflowStub()
	quota.queryResult.UserID = "private-user"
	quota.queryResult.AccountID = "private-account"
	quota.queryResult.Email = "private@example.test"
	quota.queryResult.RateLimitResetCredits = &service.OpenAIRateLimitResetCredits{
		AvailableCount: 2,
		Credits:        []service.OpenAIRateLimitResetCreditDetail{{ExpiresAt: "2099-10-04T01:56:00Z"}, {ExpiresAt: "2099-10-05T01:56:00Z"}},
	}
	handler := &OpenAIOAuthHandler{
		adminService: &openAIResetAdminServiceStub{account: boundOpenAISubscription()}, quotaService: quota,
	}
	recorder := subscriptionActionRequest(http.MethodPost, "42", 123, handler.RefreshMySubscriptionQuota)
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.JSONEq(t, `{"fetched_at":123,"cache_persisted":true,"rate_limit_reset_credits":{"available_count":2,"credits":[{"expires_at":"2099-10-04T01:56:00Z"},{"expires_at":"2099-10-05T01:56:00Z"}]}}`, string(payload.Data))
	require.Equal(t, 1, quota.queryCalls)
	require.Equal(t, 1, quota.cacheCalls)
}

func TestSubscriptionQuotaResetPreservesSuccessAndRedactsInternalData(t *testing.T) {
	for _, cacheFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "cache_failure"}[cacheFails], func(t *testing.T) {
			quota := successfulOpenAIQuotaWorkflowStub()
			quota.resetResult.Credit = &service.OpenAIQuotaResetCredit{ID: "private-credit"}
			quota.queryResult.UserID = "private-user"
			if cacheFails {
				quota.cacheErr = errors.New("cache write failed")
			}
			recoverer := &openAIAccountStateRecovererStub{}
			handler := &OpenAIOAuthHandler{
				adminService: &openAIResetAdminServiceStub{account: boundOpenAISubscription()},
				quotaService: quota, rateLimitService: recoverer,
			}
			recorder := subscriptionActionRequest(http.MethodPost, "42", 123, handler.ResetMySubscriptionQuota)
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, 1, quota.resetCalls)
			require.Equal(t, 1, recoverer.calls)
			var payload struct {
				Data openAIQuotaResetResponse `json:"data"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
			require.Equal(t, "success", payload.Data.Code)
			require.Equal(t, 1, payload.Data.WindowsReset)
			require.Equal(t, !cacheFails, payload.Data.CacheRefreshed)
			require.True(t, payload.Data.AccountStateRecovered)
			require.Nil(t, payload.Data.Account)
			require.Nil(t, payload.Data.Credit)
			require.NotContains(t, recorder.Body.String(), "private")
			if cacheFails {
				require.Equal(t, service.OpenAIQuotaResetWarningCacheRefreshFailed, payload.Data.WarningCode)
				require.Nil(t, payload.Data.Quota)
			} else {
				require.NotNil(t, payload.Data.Quota)
				require.Zero(t, payload.Data.Quota.RateLimitResetCredits.AvailableCount)
			}
		})
	}
}

func TestSubscriptionQuotaResetRejectsShadowBeforeConsumingCredit(t *testing.T) {
	account := boundOpenAISubscription()
	account.ParentAccountID = int64Pointer(7)
	quota := successfulOpenAIQuotaWorkflowStub()
	handler := &OpenAIOAuthHandler{adminService: &openAIResetAdminServiceStub{account: account}, quotaService: quota}
	recorder := subscriptionActionRequest(http.MethodPost, "42", 123, handler.ResetMySubscriptionQuota)
	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Zero(t, quota.resetCalls)
}

func TestSubscriptionAccountUsageReadsOnlyTheBoundAccount(t *testing.T) {
	for _, refresh := range []bool{false, true} {
		repo := &accountBindingUsageLogRepo{stats: []*usagestats.AccountStats{
			{Requests: 38, Tokens: 2000000}, {Requests: 2700, Tokens: 231500000},
		}}
		handler := &AccountHandler{
			adminService:        &openAIResetAdminServiceStub{account: boundOpenAISubscription()},
			accountUsageService: service.NewAccountUsageService(nil, repo, nil, nil, nil, nil, nil, nil, nil, nil, nil),
		}
		action := handler.GetMySubscriptionAccountUsage
		method := http.MethodGet
		if refresh {
			action = handler.RefreshMySubscriptionAccountUsage
			method = http.MethodPost
		}
		recorder := subscriptionActionRequest(method, "42", 123, action)
		require.Equal(t, http.StatusOK, recorder.Code)
		var payload struct {
			Data subscriptionAccountUsage `json:"data"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
		require.Equal(t, 13.0, payload.Data.FiveHour.Utilization)
		require.Equal(t, int64(38), payload.Data.FiveHour.WindowStats.Requests)
		require.Equal(t, int64(2700), payload.Data.SevenDay.WindowStats.Requests)
		require.NotContains(t, recorder.Body.String(), "private")
	}
}
