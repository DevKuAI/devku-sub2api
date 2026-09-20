package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type resourceReadRepository struct {
	service.DesktopResourceRepository
	filter service.DesktopResourceFilter
	err    error
}

func (r *resourceReadRepository) List(_ context.Context, f service.DesktopResourceFilter) ([]service.DesktopResourceRecord, int64, error) {
	r.filter = f
	return []service.DesktopResourceRecord{}, 0, r.err
}
func (r *resourceReadRepository) Get(context.Context, string, bool) (*service.DesktopResourceRecord, error) {
	return &service.DesktopResourceRecord{}, r.err
}
func (r *resourceReadRepository) Artifact(context.Context, string, string, string) (*service.DesktopResourceArtifact, error) {
	return &service.DesktopResourceArtifact{SizeBytes: 3, SHA256: strings.Repeat("a", 64)}, r.err
}

type resourceDownloadStorage struct{ service.DesktopResourceStorage }

func (resourceDownloadStorage) Open(context.Context, string) (io.ReadCloser, int64, error) {
	return io.NopCloser(strings.NewReader("ZIP")), 3, nil
}

func (resourceDownloadStorage) Presign(context.Context, string, time.Duration) (string, error) {
	return "https://private.example.com/resource.zip?signature=example", nil
}

func TestDesktopResourceReadContract(t *testing.T) {
	for _, test := range []struct {
		path     string
		err      error
		status   int
		contains string
	}{
		{"/resources?scope=enterprise", nil, 200, `"data":[]`},
		{"/resources?scope=public&limit=no&offset=no", nil, 200, `"data":[]`},
		{"/resources?scope=other", nil, 422, "VALIDATION_FAILED"},
		{"/resources?kind=other", nil, 422, "VALIDATION_FAILED"},
		{"/resources?offset=-1", nil, 422, "VALIDATION_FAILED"},
		{"/resources/id", service.ErrResourceNotFound, 404, "RESOURCE_NOT_FOUND"},
		{"/resources/id/versions/1.0.0/artifacts/darwin-arm64", nil, 200, "ZIP"},
		{"/resources/id/versions/1.0.0/artifacts/darwin-arm64", service.ErrResourceNotFound, 404, "RESOURCE_NOT_FOUND"},
		{"/resources/id/versions/1.0.0/artifacts/darwin-arm64/download-url", nil, 200, `"expiresIn":300`},
		{"/resources/id/versions/1.0.0/artifacts/darwin-arm64/download-url", service.ErrResourceNotFound, 404, "RESOURCE_NOT_FOUND"},
	} {
		t.Run(test.path+test.contains, func(t *testing.T) {
			repo := &resourceReadRepository{err: test.err}
			svc := service.NewDesktopResourceService(repo, resourceDownloadStorage{}, nil)
			h := NewDesktopResourceHandler(svc, &service.DesktopService{})
			router := gin.New()
			router.Use(func(c *gin.Context) { c.Set("desktop_authorization", &service.DesktopAuthorization{}); c.Next() })
			router.GET("/resources", h.List)
			router.GET("/resources/:resource_id", h.Get)
			router.GET("/resources/:resource_id/versions/:version/artifacts/:platform", h.Download)
			router.POST("/resources/:resource_id/versions/:version/artifacts/:platform/download-url", h.DownloadURL)
			w := httptest.NewRecorder()
			method := http.MethodGet
			if strings.HasSuffix(test.path, "/download-url") {
				method = http.MethodPost
			}
			router.ServeHTTP(w, httptest.NewRequest(method, test.path, nil))
			require.Equal(t, test.status, w.Code)
			require.Contains(t, w.Body.String(), test.contains)
			if strings.HasSuffix(test.path, "/download-url") {
				require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
			}
			if test.contains == "ZIP" {
				require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
				require.Equal(t, strings.Repeat("a", 64), w.Header().Get("X-Artifact-SHA256"))
				require.Empty(t, w.Header().Get("Location"))
			}
			if strings.Contains(test.path, "limit=no") {
				require.Equal(t, 50, repo.filter.Limit)
				require.Zero(t, repo.filter.Offset)
			}
		})
	}
}
