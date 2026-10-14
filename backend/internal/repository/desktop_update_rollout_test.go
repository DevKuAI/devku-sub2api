package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDesktopUpdateGrayPromotionPreservesArtifacts(t *testing.T) {
	repo := newDesktopUpdateRepositoryForTest(t)
	ctx := context.Background()
	now := time.Now().UTC()
	createDesktopUpdateDraft(t, repo, "upd_gray", "1.1.0")
	_, err := repo.Publish(ctx, "upd_gray", 4, now, service.DesktopUpdatePublishInput{RolloutMode: "gray", GatewayUserIDs: []int64{100}})
	require.ErrorIs(t, err, service.ErrDesktopUpdateInvalidFields)
	gray, err := repo.Publish(ctx, "upd_gray", 4, now, service.DesktopUpdatePublishInput{RolloutMode: "gray", GatewayUserIDs: []int64{7, 9, 7}})
	require.NoError(t, err)
	require.Equal(t, []int64{7, 9}, gray.GatewayUserIDs)
	full, err := service.NewDesktopUpdateService(repo).Promote(ctx, "upd_gray", 5)
	require.NoError(t, err)
	require.Equal(t, "full", full.RolloutMode)
	require.Empty(t, full.GatewayUserIDs)
	require.Equal(t, gray.Artifacts, full.Artifacts)
	require.Equal(t, gray.Version, full.Version)
	require.Equal(t, gray.PublishedAt, full.PublishedAt)
	_, err = service.NewDesktopUpdateService(repo).Promote(ctx, "upd_gray", 5)
	require.ErrorIs(t, err, service.ErrDesktopUpdateInvalidState)
	createDesktopUpdateDraft(t, repo, "upd_nextgray", "1.2.0")
	_, err = repo.Publish(ctx, "upd_nextgray", 4, now, service.DesktopUpdatePublishInput{RolloutMode: "gray", GatewayUserIDs: []int64{7}})
	require.NoError(t, err)
	createDesktopUpdateDraft(t, repo, "upd_newer", "1.3.0")
	_, err = repo.Publish(ctx, "upd_newer", 4, now)
	require.NoError(t, err)
	_, err = service.NewDesktopUpdateService(repo).Promote(ctx, "upd_nextgray", 5)
	require.ErrorIs(t, err, service.ErrDesktopUpdateNotNewer)
}

type rolloutStorage struct{ bodies [][]byte }

func (s *rolloutStorage) Upload(_ context.Context, key, _ string, body io.Reader, _ int64) (string, error) {
	data, err := io.ReadAll(body)
	s.bodies = append(s.bodies, data)
	return "https://downloads.example.com/" + key, err
}
func TestDesktopUpdateAutomationImportIsAuthenticatedRetryableAndDraftOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newDesktopUpdateRepositoryForTest(t)
	storage := &rolloutStorage{}
	cfg := &config.Config{DesktopUpdateUploadToken: strings.Repeat("t", 32), DesktopUpdateStorage: config.DesktopUpdateStorageConfig{Endpoint: "https://s3.example.com", Bucket: "updates", AccessKeyID: "test", SecretAccessKey: "test", PublicBaseURL: "https://downloads.example.com", Prefix: "desktop-updates"}}
	svc := service.ProvideDesktopUpdateService(repo, func(context.Context, *config.DesktopUpdateStorageConfig) (service.DesktopUpdateArtifactStorage, error) {
		return storage, nil
	}, cfg)
	router := gin.New()
	router.PUT("/import/:version", middleware.DesktopUpdateUploadAuth(cfg), middleware.StrictBodyLimit(1024*1024), adminhandler.NewDesktopUpdateHandler(svc).Import)
	request := func(token, fileName, signature string) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		file, err := writer.CreateFormFile(service.DesktopUpdateWindowsX64, fileName)
		require.NoError(t, err)
		_, err = file.Write([]byte("original signed updater bytes"))
		require.NoError(t, err)
		require.NoError(t, writer.WriteField(service.DesktopUpdateWindowsX64+".signature", signature))
		require.NoError(t, writer.Close())
		req := httptest.NewRequest(http.MethodPut, "/import/1.2.3", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		return recorder
	}
	require.Equal(t, http.StatusUnauthorized, request("invalid", "Devku.Desktop_1.2.3_x64-setup.nsis.zip", "sig").Code)
	require.Empty(t, storage.bodies)
	require.Equal(t, http.StatusBadRequest, request(cfg.DesktopUpdateUploadToken, "Devku.Desktop_1.2.2_x64-setup.nsis.zip", "sig").Code)
	require.Equal(t, http.StatusBadRequest, request(cfg.DesktopUpdateUploadToken, "Devku.Desktop_1.2.3_x64-setup.exe", "sig").Code)
	require.Equal(t, http.StatusBadRequest, request(cfg.DesktopUpdateUploadToken, "Devku.Desktop_1.2.3_x64-setup.nsis.zip", "").Code)
	var previous string
	for range 2 {
		result := request(cfg.DesktopUpdateUploadToken, "Devku.Desktop_1.2.3_x64-setup.nsis.zip", "sig")
		require.Equal(t, http.StatusOK, result.Code, result.Body.String())
		var envelope struct {
			Data struct {
				PublicID  string                         `json:"public_id"`
				Status    string                         `json:"status"`
				Artifacts service.DesktopUpdateArtifacts `json:"artifacts"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(result.Body.Bytes(), &envelope))
		require.Equal(t, "draft", envelope.Data.Status)
		if previous != "" {
			require.Equal(t, previous, envelope.Data.PublicID)
		}
		previous = envelope.Data.PublicID
		require.Len(t, envelope.Data.Artifacts, 3)
		require.Equal(t, "sig", envelope.Data.Artifacts[service.DesktopUpdateWindowsX64].Signature)
	}
	require.Len(t, storage.bodies, 2)
	require.Equal(t, storage.bodies[0], storage.bodies[1])
	_, err := svc.Publish(context.Background(), previous, 5)
	require.NoError(t, err)
	result := request(cfg.DesktopUpdateUploadToken, "Devku.Desktop_1.2.3_x64-setup.nsis.zip", "sig")
	require.Equal(t, http.StatusConflict, result.Code)
	require.Len(t, storage.bodies, 2)
	_, err = svc.Withdraw(context.Background(), previous, 5, "rollback")
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, request(cfg.DesktopUpdateUploadToken, "Devku.Desktop_1.2.3_x64-setup.nsis.zip", "sig").Code)
}
