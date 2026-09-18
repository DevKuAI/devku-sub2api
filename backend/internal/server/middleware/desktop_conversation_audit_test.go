package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDesktopConversationReadsAuditOnlyMetadata(t *testing.T) {
	for route, action := range map[string]string{
		"/api/v1/admin/desktop/organizations/:organization_id/conversation-records/:record_id": "admin.desktop.conversation.read",
		"/api/v1/desktop/organization/conversation-records/:record_id":                         "desktop.organization.conversation.read",
	} {
		t.Run(action, func(t *testing.T) {
			repository := &auditCaptureRepository{}
			auditService := service.NewAuditLogService(repository, nil)
			auditService.Start()
			router := gin.New()
			router.Use(gin.HandlerFunc(NewAuditLogMiddleware(auditService)))
			router.GET(route, func(c *gin.Context) {
				SetAuditExtra(c, map[string]any{"record_id": "record-one", "organization_id": "org-one", "prompts": "private-body-canary"})
				c.JSON(http.StatusOK, gin.H{"text": "private-body-canary"})
			})
			request := httptest.NewRequest(http.MethodGet, route, nil)
			request.Header.Set("Authorization", "Bearer secret-token-canary")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, 200, response.Code)
			auditService.Stop()
			repository.mu.Lock()
			defer repository.mu.Unlock()
			require.Len(t, repository.logs, 1)
			require.Equal(t, action, repository.logs[0].Action)
			require.Empty(t, repository.logs[0].RequestBody)
			raw, err := json.Marshal(repository.logs[0])
			require.NoError(t, err)
			require.Contains(t, string(raw), "record-one")
			require.NotContains(t, string(raw), "private-body-canary")
			require.NotContains(t, string(raw), "secret-token-canary")
		})
	}
}
