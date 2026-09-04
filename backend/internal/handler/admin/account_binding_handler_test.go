package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
}

func (s *accountBindingAdminService) ListAccountsByBoundUserID(_ context.Context, userID int64) ([]service.Account, error) {
	s.listedUserIDs = append(s.listedUserIDs, userID)
	return s.accounts, nil
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
			ID:           8,
			Name:         "Read-only subscription",
			Platform:     service.PlatformOpenAI,
			Type:         service.AccountTypeOAuth,
			Status:       service.StatusActive,
			Credentials:  map[string]any{"access_token": "secret"},
			Extra:        map[string]any{"workspace_id": "internal"},
			ProxyID:      &proxyID,
			ErrorMessage: "internal upstream error",
			CreatedAt:    now,
		}},
	}
	handler := newAccountBindingHandler(stub)
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
	for _, forbidden := range []string{"credentials", "extra", "proxy_id", "error_message", "bound_user_id", "bound_user"} {
		_, exists := payload.Data[0][forbidden]
		require.False(t, exists, "response must not expose %s", forbidden)
	}
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
