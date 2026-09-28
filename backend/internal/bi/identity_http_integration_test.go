//go:build integration

package bi

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBIIdentityHTTPFlow(t *testing.T) {
	s, _, userIDs := authFixture(t)
	h := &Handler{service: s}
	r := gin.New()
	r.Use(Headers)
	r.POST("/wechat", h.Login)
	r.POST("/bindings/approve", func(c *gin.Context) { c.Set(webUserKey, userIDs[0]); h.ApproveBinding(c) })
	r.POST("/bindings/exchange", h.ExchangeBinding)
	r.GET("/bindings", func(c *gin.Context) { c.Set(webUserKey, userIDs[0]); h.ListBindings(c) })
	r.POST("/refresh", h.Refresh)
	r.POST("/logout", h.Logout)

	request := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		require.Equal(t, 200, w.Code, w.Body.String())
		return w
	}
	var challenge BindingChallenge
	require.NoError(t, json.Unmarshal(request("POST", "/wechat", `{"code":"first-login","client_version":"1.0"}`).Body.Bytes(), &challenge))
	require.NotEmpty(t, challenge.BindingTicket)
	require.NotEmpty(t, challenge.UserCode)
	request("POST", "/bindings/approve", `{"user_code":"`+challenge.UserCode+`","confirm_binding":true}`)

	var session Session
	require.NoError(t, json.Unmarshal(request("POST", "/bindings/exchange", `{"binding_ticket":"`+challenge.BindingTicket+`","code":"second-login"}`).Body.Bytes(), &session))
	require.NotEmpty(t, session.RefreshToken)
	bindings := request("GET", "/bindings", "")
	require.Contains(t, bindings.Body.String(), `"items":[`)

	var refreshed Session
	require.NoError(t, json.Unmarshal(request("POST", "/refresh", `{"refresh_token":"`+session.RefreshToken+`"}`).Body.Bytes(), &refreshed))
	require.NotEmpty(t, refreshed.RefreshToken)
	request("POST", "/logout", `{"refresh_token":"`+refreshed.RefreshToken+`"}`)
}
