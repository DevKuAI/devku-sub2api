package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type desktopUpdateCheckRepoStub struct {
	releases []service.DesktopUpdateRelease
	err      error
}

func (*desktopUpdateCheckRepoStub) Create(context.Context, *service.DesktopUpdateRelease) error {
	panic("unexpected Create call")
}
func (*desktopUpdateCheckRepoStub) List(context.Context, pagination.PaginationParams, service.DesktopUpdateListFilters) ([]service.DesktopUpdateRelease, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}
func (*desktopUpdateCheckRepoStub) Get(context.Context, string) (*service.DesktopUpdateRelease, error) {
	panic("unexpected Get call")
}
func (*desktopUpdateCheckRepoStub) UpdateDraft(context.Context, string, service.DesktopUpdateDraftInput) (*service.DesktopUpdateRelease, error) {
	panic("unexpected UpdateDraft call")
}
func (*desktopUpdateCheckRepoStub) Publish(context.Context, string, int64, time.Time) (*service.DesktopUpdateRelease, error) {
	panic("unexpected Publish call")
}
func (*desktopUpdateCheckRepoStub) Withdraw(context.Context, string, int64, string, time.Time) (*service.DesktopUpdateRelease, error) {
	panic("unexpected Withdraw call")
}
func (r *desktopUpdateCheckRepoStub) ListPublished(context.Context) ([]service.DesktopUpdateRelease, error) {
	return r.releases, r.err
}

func desktopUpdateHandlerArtifacts() service.DesktopUpdateArtifacts {
	return service.DesktopUpdateArtifacts{
		service.DesktopUpdateDarwinARM64: desktopUpdateHandlerArtifact("app-arm.dmg", "arm-sig"),
		service.DesktopUpdateDarwinX64:   desktopUpdateHandlerArtifact("app-x64.app", "x64-sig"),
		service.DesktopUpdateWindowsX64:  desktopUpdateHandlerArtifact("app-x64.msi", "windows-sig"),
	}
}

func desktopUpdateHandlerArtifact(fileName, signature string) service.DesktopUpdateArtifact {
	key := "desktop-updates/1.2.3/" + fileName
	return service.DesktopUpdateArtifact{
		URL: "https://example.com/" + key, Signature: signature, ObjectKey: key,
		FileName: fileName, Size: 1234, SHA256: strings.Repeat("a", 64),
	}
}

func desktopUpdateCheckRouter(repo service.DesktopUpdateRepository) *gin.Engine {
	router := gin.New()
	router.GET("/api/desktop/v1/update/:target/:arch/:current_version", NewDesktopUpdateHandler(service.NewDesktopUpdateService(repo)).Check)
	return router
}

func TestDesktopUpdateCheckHandlerResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	publishedAt := time.Date(2026, time.September, 1, 8, 0, 0, 0, time.UTC)
	release := service.DesktopUpdateRelease{
		Version: "1.2.3", Notes: "Release notes", Status: service.DesktopUpdateStatusPublished,
		PublishedAt: &publishedAt, Artifacts: desktopUpdateHandlerArtifacts(),
	}

	tests := []struct {
		name       string
		path       string
		repo       *desktopUpdateCheckRepoStub
		wantStatus int
		wantCode   string
	}{
		{name: "update available", path: "/api/desktop/v1/update/darwin/aarch64/1.0.0", repo: &desktopUpdateCheckRepoStub{releases: []service.DesktopUpdateRelease{release}}, wantStatus: http.StatusOK},
		{name: "no update", path: "/api/desktop/v1/update/darwin/aarch64/1.2.3", repo: &desktopUpdateCheckRepoStub{releases: []service.DesktopUpdateRelease{release}}, wantStatus: http.StatusNoContent},
		{name: "invalid version", path: "/api/desktop/v1/update/darwin/aarch64/v1.0.0", repo: &desktopUpdateCheckRepoStub{}, wantStatus: http.StatusBadRequest, wantCode: "DESKTOP_UPDATE_VERSION_INVALID"},
		{name: "unsupported platform", path: "/api/desktop/v1/update/linux/x86_64/1.0.0", repo: &desktopUpdateCheckRepoStub{}, wantStatus: http.StatusBadRequest, wantCode: "DESKTOP_UPDATE_PLATFORM_UNSUPPORTED"},
		{name: "database failure", path: "/api/desktop/v1/update/windows/x86_64/1.0.0", repo: &desktopUpdateCheckRepoStub{err: errors.New("database unavailable")}, wantStatus: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			desktopUpdateCheckRouter(test.repo).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
			require.Equal(t, test.wantStatus, recorder.Code)
			require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
			if test.wantCode != "" {
				var envelope struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
				require.Equal(t, test.wantCode, envelope.Error.Code)
			}
		})
	}
}
