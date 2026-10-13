package repository

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAccountingStorageUsesResourceBucketAndAccountingFolder(t *testing.T) {
	var objectPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		objectPath = r.URL.Path
		require.Equal(t, "PUT", r.Method)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, "proof", string(body))
		w.WriteHeader(200)
	}))
	defer server.Close()
	cfg := &config.Config{DesktopUpdateStorage: config.DesktopUpdateStorageConfig{Endpoint: server.URL, Region: "test", Bucket: "public-updater", ResourceBucket: "private-resources", AccessKeyID: "test-key", SecretAccessKey: "test-secret", ForcePathStyle: true}}
	storage := NewAccountingStorage(cfg)
	require.True(t, storage.Ready())
	require.NoError(t, storage.Upload(context.Background(), "accounting/org/file.pdf", strings.NewReader("proof"), 5, "application/pdf"))
	require.Equal(t, "/private-resources/accounting/org/file.pdf", objectPath)
	url, err := storage.Presign(context.Background(), "accounting/org/file.pdf", "invoice.pdf", "application/pdf", 5*time.Minute)
	require.NoError(t, err)
	require.Contains(t, url, "/private-resources/")
	require.Contains(t, url, "X-Amz-Expires=300")
	require.Contains(t, url, "response-cache-control=private%2C%20no-store")
	require.NotContains(t, url, "test-secret")
	cfg.DesktopUpdateStorage.ResourceBucket = ""
	require.False(t, NewAccountingStorage(cfg).Ready())
}
