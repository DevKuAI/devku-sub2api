package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type accountBindingAdminService struct {
	*stubAdminService
	accounts          []service.Account
	listedUserIDs     []int64
	boundAccountID    int64
	boundUserID       *int64
	expectedUserID    *int64
	bindAccountResult *service.Account
	bindAccountError  error
	parents           []*service.Account
	parentIDs         []int64
}

type accountBindingConcurrencyCache struct {
	service.ConcurrencyCache
	counts map[int64]int
	ids    []int64
	err    error
}

func (c *accountBindingConcurrencyCache) GetAccountConcurrencyBatch(_ context.Context, ids []int64) (map[int64]int, error) {
	c.ids = append(c.ids, ids...)
	return c.counts, c.err
}

type accountBindingUsageLogRepo struct {
	service.UsageLogRepository
	stats  []*usagestats.AccountStats
	starts []time.Time
}

func (r *accountBindingUsageLogRepo) GetAccountWindowStats(_ context.Context, _ int64, start time.Time) (*usagestats.AccountStats, error) {
	r.starts = append(r.starts, start)
	return r.stats[len(r.starts)-1], nil
}

func (s *accountBindingAdminService) ListAccountsByBoundUserID(_ context.Context, userID int64) ([]service.Account, error) {
	s.listedUserIDs = append(s.listedUserIDs, userID)
	return s.accounts, nil
}

func (s *accountBindingAdminService) GetAccountsByIDs(_ context.Context, ids []int64) ([]*service.Account, error) {
	s.parentIDs = append(s.parentIDs, ids...)
	return s.parents, nil
}

func (s *accountBindingAdminService) BindAccountUser(_ context.Context, accountID int64, userID, expectedUserID *int64) (*service.Account, error) {
	s.boundAccountID = accountID
	s.boundUserID = copyOptionalInt64(userID)
	s.expectedUserID = copyOptionalInt64(expectedUserID)
	return s.bindAccountResult, s.bindAccountError
}

func newAccountBindingHandler(service *accountBindingAdminService) *AccountHandler {
	return NewAccountHandler(service, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
}

func TestAccountHandlerBindUserSupportsBindingAndRemoval(t *testing.T) {
	for _, test := range []struct {
		name               string
		body               string
		wantUserID         *int64
		wantExpectedUserID *int64
	}{
		{name: "bind", body: `{"bound_user_id":42,"expected_bound_user_id":null}`, wantUserID: int64Pointer(42)},
		{name: "remove", body: `{"bound_user_id":null,"expected_bound_user_id":42}`, wantExpectedUserID: int64Pointer(42)},
	} {
		t.Run(test.name, func(t *testing.T) {
			stub := &accountBindingAdminService{
				stubAdminService:  newStubAdminService(),
				bindAccountResult: &service.Account{ID: 7, Name: "Subscription", Status: service.StatusActive},
			}
			router := gin.New()
			router.PUT("/accounts/:id/binding", newAccountBindingHandler(stub).BindUser)

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPut, "/accounts/7/binding", bytes.NewBufferString(test.body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, int64(7), stub.boundAccountID)
			if test.wantUserID == nil {
				require.Nil(t, stub.boundUserID)
			} else {
				require.Equal(t, *test.wantUserID, *stub.boundUserID)
			}
			if test.wantExpectedUserID == nil {
				require.Nil(t, stub.expectedUserID)
			} else {
				require.Equal(t, *test.wantExpectedUserID, *stub.expectedUserID)
			}
		})
	}
}

func TestAccountHandlerBindUserRequiresExplicitBindingValue(t *testing.T) {
	for _, body := range []string{
		`{"expected_bound_user_id":null}`,
		`{"bound_user_id":42}`,
	} {
		stub := &accountBindingAdminService{
			stubAdminService:  newStubAdminService(),
			bindAccountResult: &service.Account{ID: 7},
		}
		router := gin.New()
		router.PUT("/accounts/:id/binding", newAccountBindingHandler(stub).BindUser)

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/accounts/7/binding", bytes.NewBufferString(body))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)

		require.Equal(t, http.StatusBadRequest, recorder.Code)
		require.Zero(t, stub.boundAccountID)
	}
}

func TestAccountHandlerBindUserReturnsConflictForStaleBinding(t *testing.T) {
	stub := &accountBindingAdminService{
		stubAdminService: newStubAdminService(),
		bindAccountError: service.ErrAccountBindingConflict,
	}
	router := gin.New()
	router.PUT("/accounts/:id/binding", newAccountBindingHandler(stub).BindUser)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/accounts/7/binding", bytes.NewBufferString(
		`{"bound_user_id":null,"expected_bound_user_id":42}`,
	))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Contains(t, recorder.Body.String(), "ACCOUNT_BINDING_CONFLICT")
}

func TestAccountHandlerListMySubscriptionAccountsUsesAuthenticatedUserAndRedactsInternals(t *testing.T) {
	now := time.Date(2026, 9, 4, 1, 2, 3, 0, time.UTC)
	proxyID := int64(99)
	stub := &accountBindingAdminService{
		stubAdminService: newStubAdminService(),
		accounts: []service.Account{{
			ID:       8,
			Name:     "Read-only subscription",
			Platform: service.PlatformOpenAI,
			Type:     service.AccountTypeOAuth,
			Status:   service.StatusActive,
			Credentials: map[string]any{
				"access_token":            "secret",
				"auth_mode":               "agent_identity",
				"plan_type":               "pro",
				"subscription_expires_at": "2026-09-13T00:00:00Z",
			},
			Extra: map[string]any{
				"workspace_id": "internal", "privacy_mode": "training_off",
				"codex_reset_credit_snapshot": map[string]any{
					"available_count": 2,
					"credits":         []any{map[string]any{"id": "secret-credit", "expires_at": "2099-10-04T01:56:00Z"}},
				},
			},
			Concurrency:  100,
			ProxyID:      &proxyID,
			ErrorMessage: "internal upstream error",
			CreatedAt:    now,
		}},
	}
	handler := newAccountBindingHandler(stub)
	concurrencyCache := &accountBindingConcurrencyCache{counts: map[int64]int{8: 1, 999: 77}}
	handler.concurrencyService = service.NewConcurrencyService(concurrencyCache)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 123})
		c.Next()
	})
	router.GET("/subscription-accounts", handler.ListMySubscriptionAccounts)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/subscription-accounts", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, []int64{123}, stub.listedUserIDs)
	var payload struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Len(t, payload.Data, 1)
	require.Equal(t, "Read-only subscription", payload.Data[0]["name"])
	require.Equal(t, "agent_identity", payload.Data[0]["auth_mode"])
	require.Equal(t, "pro", payload.Data[0]["plan_type"])
	require.Equal(t, "training_off", payload.Data[0]["privacy_mode"])
	require.Equal(t, "2026-09-13T00:00:00Z", payload.Data[0]["subscription_expires_at"])
	require.Nil(t, payload.Data[0]["expires_at"])
	require.Equal(t, "auto", payload.Data[0]["openai_compact_state"])
	require.Equal(t, float64(1), payload.Data[0]["current_concurrency"])
	require.Equal(t, []int64{8}, concurrencyCache.ids)
	require.Equal(t, false, payload.Data[0]["is_shadow"])
	require.Equal(t, map[string]any{
		"available_count": float64(2),
		"credits":         []any{map[string]any{"expires_at": "2099-10-04T01:56:00Z"}},
	}, payload.Data[0]["reset_credits"])
	for _, forbidden := range []string{"credentials", "extra", "proxy_id", "error_message", "bound_user_id", "bound_user", "concurrency", "parent_account_id"} {
		_, exists := payload.Data[0][forbidden]
		require.False(t, exists, "response must not expose %s", forbidden)
	}
	require.NotContains(t, recorder.Body.String(), "secret")
	require.NotContains(t, recorder.Body.String(), "internal")
}

func TestAccountHandlerListMySubscriptionAccountsInheritsOnlyMissingSubscriptionFields(t *testing.T) {
	parentID := int64(7)
	stub := &accountBindingAdminService{
		stubAdminService: newStubAdminService(),
		accounts: []service.Account{
			{ID: 8, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, ParentAccountID: &parentID},
			{
				ID: 9, Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, ParentAccountID: &parentID,
				Credentials: map[string]any{"plan_type": "plus", "subscription_expires_at": "2026-09-28T00:00:00Z"},
				Extra:       map[string]any{"privacy_mode": "training_on"},
			},
		},
		parents: []*service.Account{{
			ID:   parentID,
			Name: "private parent name",
			Credentials: map[string]any{
				"access_token": "parent-secret", "plan_type": "pro", "subscription_expires_at": "2026-09-13T00:00:00Z",
			},
			Extra: map[string]any{"privacy_mode": "training_off", "workspace_id": "private workspace"},
		}},
	}
	recorder := requestSubscriptionAccounts(newAccountBindingHandler(stub))
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Data []subscriptionAccountResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Len(t, payload.Data, 2)
	require.Equal(t, []int64{parentID}, stub.parentIDs)
	require.Equal(t, "pro", payload.Data[0].PlanType)
	require.Equal(t, "training_off", payload.Data[0].PrivacyMode)
	require.Equal(t, "2026-09-13T00:00:00Z", payload.Data[0].SubscriptionExpiresAt)
	require.Equal(t, "plus", payload.Data[1].PlanType)
	require.Equal(t, "training_on", payload.Data[1].PrivacyMode)
	require.Equal(t, "2026-09-28T00:00:00Z", payload.Data[1].SubscriptionExpiresAt)
	require.NotContains(t, recorder.Body.String(), "parent-secret")
	require.NotContains(t, recorder.Body.String(), "private")
}

func TestAccountHandlerListMySubscriptionAccountsCompactStates(t *testing.T) {
	for _, test := range []struct {
		name     string
		platform string
		kind     string
		extra    map[string]any
		want     string
	}{
		{name: "unknown", platform: service.PlatformOpenAI, kind: service.AccountTypeOAuth, want: "auto"},
		{name: "supported", platform: service.PlatformOpenAI, kind: service.AccountTypeAPIKey, extra: map[string]any{"openai_compact_supported": true}, want: "active"},
		{name: "unsupported", platform: service.PlatformOpenAI, kind: service.AccountTypeOAuth, extra: map[string]any{"openai_compact_supported": false}, want: "blocked"},
		{name: "force_on", platform: service.PlatformOpenAI, kind: service.AccountTypeOAuth, extra: map[string]any{"openai_compact_mode": "force_on", "openai_compact_supported": false}, want: "active"},
		{name: "force_off", platform: service.PlatformOpenAI, kind: service.AccountTypeOAuth, extra: map[string]any{"openai_compact_mode": "force_off", "openai_compact_supported": true}, want: "blocked"},
		{name: "other_platform", platform: service.PlatformAnthropic, kind: service.AccountTypeOAuth},
		{name: "other_type", platform: service.PlatformOpenAI, kind: service.AccountTypeSetupToken},
	} {
		t.Run(test.name, func(t *testing.T) {
			stub := &accountBindingAdminService{
				stubAdminService: newStubAdminService(),
				accounts:         []service.Account{{ID: 8, Platform: test.platform, Type: test.kind, Extra: test.extra}},
			}
			recorder := requestSubscriptionAccounts(newAccountBindingHandler(stub))
			require.Equal(t, http.StatusOK, recorder.Code)
			var payload struct {
				Data []subscriptionAccountResponse `json:"data"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
			require.Len(t, payload.Data, 1)
			require.Equal(t, test.want, payload.Data[0].OpenAICompactState)
			if test.want == "" {
				require.NotContains(t, recorder.Body.String(), "openai_compact_state")
			}
		})
	}
}

func TestAccountHandlerListMySubscriptionAccountsDistinguishesZeroAndUnavailableConcurrency(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "zero"},
		{name: "unavailable", err: errors.New("cache unavailable")},
	} {
		t.Run(test.name, func(t *testing.T) {
			stub := &accountBindingAdminService{
				stubAdminService: newStubAdminService(),
				accounts:         []service.Account{{ID: 8, Name: "Subscription"}},
			}
			handler := newAccountBindingHandler(stub)
			handler.concurrencyService = service.NewConcurrencyService(&accountBindingConcurrencyCache{
				counts: map[int64]int{8: 0}, err: test.err,
			})
			recorder := requestSubscriptionAccounts(handler)
			require.Equal(t, http.StatusOK, recorder.Code)
			var payload struct {
				Data []subscriptionAccountResponse `json:"data"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
			require.Len(t, payload.Data, 1)
			require.Equal(t, "Subscription", payload.Data[0].Name)
			if test.err != nil {
				require.Nil(t, payload.Data[0].CurrentConcurrency)
			} else {
				require.NotNil(t, payload.Data[0].CurrentConcurrency)
				require.Zero(t, *payload.Data[0].CurrentConcurrency)
			}
		})
	}
}

func requestSubscriptionAccounts(handler *AccountHandler) *httptest.ResponseRecorder {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 123})
		c.Next()
	})
	router.GET("/subscription-accounts", handler.ListMySubscriptionAccounts)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/subscription-accounts", nil))
	return recorder
}

func TestAccountHandlerListMySubscriptionAccountsIncludesReadOnlyUsageWindows(t *testing.T) {
	stub := &accountBindingAdminService{
		stubAdminService: newStubAdminService(),
		accounts: []service.Account{{
			ID:       8,
			Name:     "Codex subscription",
			Platform: service.PlatformOpenAI,
			Type:     service.AccountTypeOAuth,
			Status:   service.StatusActive,
			Extra: map[string]any{
				"codex_5h_used_percent": 0.0,
				"codex_7d_used_percent": 7.0,
			},
		}},
	}
	usageRepo := &accountBindingUsageLogRepo{stats: []*usagestats.AccountStats{
		{Requests: 660, Tokens: 60100000, Cost: 54.82, StandardCost: 48.00, UserCost: 54.82},
		{Requests: 2300, Tokens: 195800000, Cost: 175.07, StandardCost: 160.00, UserCost: 175.07},
	}}
	handler := newAccountBindingHandler(stub)
	handler.accountUsageService = service.NewAccountUsageService(
		nil, usageRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 123})
		c.Next()
	})
	router.GET("/subscription-accounts", handler.ListMySubscriptionAccounts)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/subscription-accounts?include_usage=true", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, usageRepo.starts, 2)
	var payload struct {
		Data []struct {
			Usage *subscriptionAccountUsage `json:"usage"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Len(t, payload.Data, 1)
	require.NotNil(t, payload.Data[0].Usage)
	require.NotNil(t, payload.Data[0].Usage.FiveHour)
	require.NotNil(t, payload.Data[0].Usage.SevenDay)
	require.Equal(t, 0.0, payload.Data[0].Usage.FiveHour.Utilization)
	require.Equal(t, 7.0, payload.Data[0].Usage.SevenDay.Utilization)
	require.Equal(t, &service.WindowStats{
		Requests: 660, Tokens: 60100000, Cost: 54.82, StandardCost: 48.00, UserCost: 54.82,
	}, payload.Data[0].Usage.FiveHour.WindowStats)
	require.Equal(t, &service.WindowStats{
		Requests: 2300, Tokens: 195800000, Cost: 175.07, StandardCost: 160.00, UserCost: 175.07,
	}, payload.Data[0].Usage.SevenDay.WindowStats)
}

func TestAccountHandlerListMySubscriptionAccountsRequiresAuthentication(t *testing.T) {
	stub := &accountBindingAdminService{stubAdminService: newStubAdminService()}
	router := gin.New()
	router.GET("/subscription-accounts", newAccountBindingHandler(stub).ListMySubscriptionAccounts)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/subscription-accounts", nil))

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Empty(t, stub.listedUserIDs)
}

func int64Pointer(value int64) *int64 {
	return &value
}

func copyOptionalInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
