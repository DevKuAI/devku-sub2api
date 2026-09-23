package bi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestMobileTokensRejectOtherAudiencesExpiryAndSecrets(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	m := newTokenManager(base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", 32))))
	raw, err := m.issue("manager", "session", now)
	require.NoError(t, err)
	claims, err := m.parse(raw, now)
	require.NoError(t, err)
	require.Equal(t, "session", claims.SessionID)
	_, err = m.parse(raw, now.Add(AccessTTL))
	require.ErrorIs(t, err, ErrUnauthenticated)
	for _, audience := range []string{WebAudience, "devku-desktop", "bi-connector", ""} {
		claims.Audience = jwt.ClaimStrings{audience}
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
		require.NoError(t, err)
		_, err = m.parse(token, now)
		require.ErrorIs(t, err, ErrUnauthenticated)
	}
	other := newTokenManager(base64.StdEncoding.EncodeToString([]byte(strings.Repeat("y", 32))))
	_, err = other.parse(raw, now)
	require.ErrorIs(t, err, ErrUnauthenticated)
	require.GreaterOrEqual(t, len(randomToken("")), 32)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWeChatUsesCode2SessionAndNeverReturnsCredentials(t *testing.T) {
	w := newWeChatClient("wx-app", "sensitive-app-secret")
	w.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "https", r.URL.Scheme)
		require.Equal(t, "api.weixin.qq.com", r.URL.Host)
		require.Equal(t, "/sns/jscode2session", r.URL.Path)
		require.Equal(t, "authorization_code", r.URL.Query().Get("grant_type"))
		require.Equal(t, "fresh-code", r.URL.Query().Get("js_code"))
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"openid":"openid-only","session_key":"must-not-escape"}`))}, nil
	})
	identity, err := w.Exchange(context.Background(), "fresh-code")
	require.NoError(t, err)
	require.Equal(t, "openid-only", identity)
	w.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("transport failed with sensitive-app-secret")
	})
	_, err = w.Exchange(context.Background(), "fresh-code")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "sensitive-app-secret")
}

func TestWeChatRejectsCredentialRedirects(t *testing.T) {
	w := newWeChatClient("app", "secret")
	calls := 0
	w.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": {"https://attacker.invalid/"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	_, err := w.Exchange(context.Background(), "code")
	require.Error(t, err)
	require.Equal(t, 1, calls)
}

func TestErrorEnvelopeAndStrictInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Headers)
	r.POST("/body", func(c *gin.Context) {
		var input struct {
			Code string `json:"code"`
		}
		if decodeRequest(c, &input) {
			c.Status(204)
		}
	})
	for _, tc := range []struct {
		body   string
		status int
	}{
		{`{"code":"x","user_id":42}`, 400},
		{`{"code":"x"} {"code":"y"}`, 400},
		{`{"code":"` + strings.Repeat("x", 9000) + `"}`, 413},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/body", strings.NewReader(tc.body)))
		require.Equal(t, tc.status, w.Code)
		var body Error
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.NotEmpty(t, body.RequestID)
		require.Equal(t, body.RequestID, w.Header().Get("X-Request-ID"))
		require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
		require.NotNil(t, body.Details)
	}
}

func TestCursorCannotCrossUserOperationOrPageSize(t *testing.T) {
	s := &Service{identity: []byte(strings.Repeat("x", 32))}
	raw := s.encodeCursor(pageCursor{Snapshot: "snapshot", Offset: 20, Limit: 20}, 1, "listOrganizations")
	_, err := s.decodeCursor(raw, 1, "listOrganizations", 20)
	require.NoError(t, err)
	for _, tc := range []struct {
		user      int64
		operation string
		limit     int
	}{
		{2, "listOrganizations", 20}, {1, "listAccountBindings", 20}, {1, "listOrganizations", 100},
	} {
		_, err := s.decodeCursor(raw, tc.user, tc.operation, tc.limit)
		require.Error(t, err)
	}
}
