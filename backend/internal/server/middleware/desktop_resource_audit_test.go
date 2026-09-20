package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDesktopResourceAuditOmitsZIPAndRetainsPublicationMetadata(t *testing.T) {
	for _, route := range []string{"/api/v1/admin/desktop/resources", "/api/v1/admin/desktop/resources/validate"} {
		repo := &auditCaptureRepository{}
		svc := service.NewAuditLogService(repo, nil)
		svc.Start()
		r := gin.New()
		r.Use(gin.HandlerFunc(NewAuditLogMiddleware(svc)))
		r.POST(route, func(c *gin.Context) {
			raw, err := io.ReadAll(c.Request.Body)
			require.NoError(t, err)
			require.Equal(t, "ZIP-CANARY", string(raw))
			SetAuditExtra(c, map[string]any{"resource_id": "res_one", "sha256": strings.Repeat("a", 64)})
			c.Status(200)
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, route, strings.NewReader("ZIP-CANARY"))
		req.Header.Set("Content-Type", "application/zip")
		r.ServeHTTP(w, req)
		svc.Stop()
		repo.mu.Lock()
		require.Len(t, repo.logs, 1)
		require.Equal(t, "<binary body omitted>", repo.logs[0].RequestBody)
		require.Equal(t, "res_one", repo.logs[0].Extra["resource_id"])
		require.Equal(t, strings.Repeat("a", 64), repo.logs[0].Extra["sha256"])
		repo.mu.Unlock()
	}
}

func TestDesktopResourceSignedURLIsNotAudited(t *testing.T) {
	repo := &auditCaptureRepository{}
	svc := service.NewAuditLogService(repo, nil)
	svc.Start()
	r := gin.New()
	r.Use(gin.HandlerFunc(NewAuditLogMiddleware(svc)))
	r.POST("/api/v1/admin/desktop/resources/:resource_id/versions/:version/artifacts/:platform/download-url", func(c *gin.Context) {
		SetAuditExtra(c, map[string]any{"resource_id": "res_one", "version": "1.0.0", "platform": "any"})
		c.JSON(200, gin.H{"data": gin.H{"url": "https://private.example.com/file?X-Amz-Signature=sensitive-signature-canary"}})
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/admin/desktop/resources/res_one/versions/1.0.0/artifacts/any/download-url", nil))
	require.Equal(t, http.StatusOK, w.Code)
	svc.Stop()
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.logs, 1)
	raw, err := json.Marshal(repo.logs[0])
	require.NoError(t, err)
	require.Contains(t, string(raw), "res_one")
	require.NotContains(t, string(raw), "sensitive-signature-canary")
}
