package repository

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestDesktopResourceStorageReusesUpdateConfiguration(t *testing.T) {
	const payload = "zip bytes"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/private-resources/existing/resources/file.zip", r.URL.Path)
		require.Contains(t, r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=test-ak/")
		require.NotContains(t, r.Header.Get("Authorization"), "Bearer")
		if r.Method == http.MethodPut {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.Equal(t, payload, string(body))
			require.Equal(t, "application/zip", r.Header.Get("Content-Type"))
			w.WriteHeader(200)
			return
		}
		require.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Length", "9")
		_, _ = io.WriteString(w, payload)
	}))
	defer server.Close()
	cfg := &config.Config{DesktopUpdateStorage: config.DesktopUpdateStorageConfig{Endpoint: server.URL, Region: "auto", Bucket: "releases", ResourceBucket: "private-resources", AccessKeyID: "test-ak", SecretAccessKey: "test-sk", ForcePathStyle: true}}
	storage := NewDesktopResourceStorage(cfg)
	err := storage.Upload(context.Background(), "existing/resources/file.zip", strings.NewReader(payload), int64(len(payload)))
	require.NoError(t, err)
	body, size, err := storage.Open(context.Background(), "existing/resources/file.zip")
	require.NoError(t, err)
	defer func() { _ = body.Close() }()
	require.EqualValues(t, len(payload), size)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, payload, string(data))
}

func TestDesktopResourceStorageMissingConfiguration(t *testing.T) {
	err := NewDesktopResourceStorage(nil).Upload(context.Background(), "key", strings.NewReader("a"), 1)
	require.Error(t, err)
}

func TestDesktopResourcePresignsPrivateBucketWithBoundedExpiry(t *testing.T) {
	cfg := &config.Config{DesktopUpdateStorage: config.DesktopUpdateStorageConfig{
		Endpoint: "https://s3.example.com", Region: "auto", Bucket: "public-updates", ResourceBucket: "private-resources",
		AccessKeyID: "test-ak", SecretAccessKey: "test-sk", ForcePathStyle: true,
		PublicBaseURL: "https://public-downloads.example.com",
	}}
	storage := NewDesktopResourceStorage(cfg)
	link, err := storage.Presign(context.Background(), "resources/package.zip", 5*time.Minute)
	require.NoError(t, err)
	u, err := url.Parse(link)
	require.NoError(t, err)
	require.Equal(t, "s3.example.com", u.Host)
	require.Equal(t, "/private-resources/resources/package.zip", u.Path)
	require.Equal(t, "300", u.Query().Get("X-Amz-Expires"))
	require.NotEmpty(t, u.Query().Get("X-Amz-Signature"))
	require.Equal(t, `attachment; filename="resource.zip"`, u.Query().Get("response-content-disposition"))
	require.Equal(t, "private, no-store", u.Query().Get("response-cache-control"))
	require.Equal(t, "application/zip", u.Query().Get("response-content-type"))
	require.NotContains(t, link, "public-updates")
	require.NotContains(t, link, cfg.DesktopUpdateStorage.PublicBaseURL)
	cfg.DesktopUpdateStorage.ResourceBucket = ""
	_, err = NewDesktopResourceStorage(cfg).Presign(context.Background(), "resources/package.zip", 5*time.Minute)
	require.Error(t, err, "must not fall back to the public updater bucket")
}
