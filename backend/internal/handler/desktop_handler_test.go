package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDesktopBodyLimitReturnsDesktopErrorEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.StrictBodyLimit(8 * 1024))
	router.POST("/api/desktop/v1/auth/login", NewDesktopHandler(nil).Login)

	body := `{"organization_code":"desktop","name":"` + strings.Repeat("x", 9*1024) + `","phone":"13800000000"}`
	request := httptest.NewRequest(http.MethodPost, "/api/desktop/v1/auth/login", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Installation-ID", "11111111-1111-4111-8111-111111111111")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	var envelope struct {
		Error struct {
			Code      string            `json:"code"`
			Message   string            `json:"message"`
			RequestID string            `json:"request_id"`
			Retryable bool              `json:"retryable"`
			Details   map[string]string `json:"details"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "PAYLOAD_TOO_LARGE", envelope.Error.Code)
	require.NotEmpty(t, envelope.Error.Message)
	require.False(t, envelope.Error.Retryable)
	require.NotNil(t, envelope.Error.Details)
}

func TestDesktopMeReturnsFullPhone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	authorized := &service.DesktopAuthorizedMember{
		Member:       &service.DesktopMember{PublicID: "mem_one", Name: "Member", Phone: "+8613800000000"},
		Organization: &service.DesktopOrganization{PublicID: "org_one", Code: "desktop", Name: "Desktop"},
	}
	desktop := service.NewDesktopService(nil, nil, nil, nil, nil, nil, cfg)
	router := gin.New()
	router.GET("/api/desktop/v1/me", func(c *gin.Context) {
		c.Set("desktop_authorized_member", authorized)
		NewDesktopHandler(desktop).Me(c)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/desktop/v1/me", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	var envelope struct {
		Data struct {
			Phone string `json:"phone"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, "+8613800000000", envelope.Data.Phone)
	require.NotContains(t, recorder.Body.String(), "masked_phone")
}
