package admin

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type adminDesktopUpdateRepoStub struct {
	mu            sync.Mutex
	createCalls   int
	publishCalls  int
	withdrawCalls int
}

func (r *adminDesktopUpdateRepoStub) Create(_ context.Context, release *service.DesktopUpdateRelease) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.createCalls++
	release.CreatedAt = time.Now().UTC()
	release.UpdatedAt = release.CreatedAt
	return nil
}

func (*adminDesktopUpdateRepoStub) List(context.Context, pagination.PaginationParams, service.DesktopUpdateListFilters) ([]service.DesktopUpdateRelease, *pagination.PaginationResult, error) {
	return nil, &pagination.PaginationResult{}, nil
}

func (*adminDesktopUpdateRepoStub) Get(_ context.Context, publicID string) (*service.DesktopUpdateRelease, error) {
	return &service.DesktopUpdateRelease{PublicID: publicID, Version: "1.2.3", Status: service.DesktopUpdateStatusDraft}, nil
}

type adminDesktopUpdateStorageStub struct{}

func (*adminDesktopUpdateStorageStub) Upload(_ context.Context, key, _ string, body io.Reader, _ int64) (string, error) {
	_, err := io.Copy(io.Discard, body)
	return "https://downloads.example.com/" + key, err
}

func TestDesktopUpdateArtifactUploadAcceptsMultipartBundle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &adminDesktopUpdateRepoStub{}
	cfg := &config.Config{DesktopUpdateStorage: config.DesktopUpdateStorageConfig{
		Endpoint: "https://oss-cn-hangzhou.aliyuncs.com", Region: "cn-hangzhou", Bucket: "desktop-releases",
		AccessKeyID: "ak", SecretAccessKey: "sk", Prefix: "desktop-updates/", PublicBaseURL: "https://downloads.example.com", MaxUploadBytes: 1024,
	}}
	updates := service.ProvideDesktopUpdateService(repo, func(context.Context, *config.DesktopUpdateStorageConfig) (service.DesktopUpdateArtifactStorage, error) {
		return &adminDesktopUpdateStorageStub{}, nil
	}, cfg)
	router := gin.New()
	router.MaxMultipartMemory = 1
	router.POST("/updates/:release_id/artifacts/:platform", NewDesktopUpdateHandler(updates).UploadArtifact)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "DevKu.dmg")
	require.NoError(t, err)
	_, err = part.Write([]byte("artifact"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	request := httptest.NewRequest(http.MethodPost, "/updates/upd_one/artifacts/darwin-aarch64", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"file_name":"DevKu.dmg"`)
	require.Contains(t, recorder.Body.String(), `"sha256":`)
	uploaded := request.MultipartForm.File["file"][0]
	_, err = uploaded.Open()
	require.Error(t, err, "multipart temporary file should be removed after the request")
}

func (*adminDesktopUpdateRepoStub) UpdateDraft(_ context.Context, publicID string, input service.DesktopUpdateDraftInput) (*service.DesktopUpdateRelease, error) {
	return &service.DesktopUpdateRelease{PublicID: publicID, Version: input.Version, Notes: input.Notes, Artifacts: input.Artifacts, Status: service.DesktopUpdateStatusDraft}, nil
}

func (r *adminDesktopUpdateRepoStub) Publish(_ context.Context, publicID string, _ int64, publishedAt time.Time) (*service.DesktopUpdateRelease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.publishCalls++
	return &service.DesktopUpdateRelease{PublicID: publicID, Version: "1.2.3", Status: service.DesktopUpdateStatusPublished, PublishedAt: &publishedAt}, nil
}

func (r *adminDesktopUpdateRepoStub) Withdraw(_ context.Context, publicID string, _ int64, reason string, withdrawnAt time.Time) (*service.DesktopUpdateRelease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.withdrawCalls++
	publishedAt := withdrawnAt.Add(-time.Hour)
	return &service.DesktopUpdateRelease{
		PublicID: publicID, Version: "1.2.3", Status: service.DesktopUpdateStatusWithdrawn,
		PublishedAt: &publishedAt, WithdrawnAt: &withdrawnAt, WithdrawalReason: &reason,
	}, nil
}

func (*adminDesktopUpdateRepoStub) ListPublished(context.Context) ([]service.DesktopUpdateRelease, error) {
	return nil, nil
}

func adminDesktopUpdateDraftBody() []byte {
	return []byte(`{
		"version":"1.2.3",
		"notes":"Release notes",
		"artifacts":{
			"darwin-aarch64":{"url":"https://example.com/arm.tar.gz","signature":"arm-signature"},
			"darwin-x86_64":{"url":"https://example.com/x64.tar.gz","signature":"x64-signature"},
			"windows-x86_64":{"url":"https://example.com/app.zip","signature":"windows-signature"}
		}
	}`)
}

func TestDesktopUpdateLifecycleWritesRequireIdempotencyKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := service.DefaultIdempotencyCoordinator()
	cfg := service.DefaultIdempotencyConfig()
	cfg.ObserveOnly = true
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(newMemoryIdempotencyRepoStub(), cfg))
	t.Cleanup(func() { service.SetDefaultIdempotencyCoordinator(previous) })

	repo := &adminDesktopUpdateRepoStub{}
	handler := NewDesktopUpdateHandler(service.NewDesktopUpdateService(repo))
	router := gin.New()
	router.POST("/updates", handler.Create)
	router.POST("/updates/:release_id/publish", handler.Publish)
	router.POST("/updates/:release_id/withdraw", handler.Withdraw)

	tests := []struct {
		name string
		path string
		body []byte
	}{
		{name: "create", path: "/updates", body: adminDesktopUpdateDraftBody()},
		{name: "publish", path: "/updates/upd_one/publish"},
		{name: "withdraw", path: "/updates/upd_one/withdraw", body: []byte(`{"reason":"Rollback"}`)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, test.path, bytes.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.Contains(t, recorder.Body.String(), "IDEMPOTENCY_KEY_REQUIRED")
		})
	}
	require.Zero(t, repo.createCalls)
	require.Zero(t, repo.publishCalls)
	require.Zero(t, repo.withdrawCalls)
}
