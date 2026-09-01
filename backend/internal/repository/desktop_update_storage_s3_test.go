//go:build unit

package repository

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestS3DesktopUpdateArtifactStorageUploadsNonSeekableStream(t *testing.T) {
	const payload = "streamed desktop updater payload"
	var received []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/desktop-releases/desktop-updates/1.2.3/darwin-aarch64/artifact_one.app.tar.gz", r.URL.Path)
		require.Equal(t, int64(len(payload)), r.ContentLength)
		require.Equal(t, "application/gzip", r.Header.Get("Content-Type"))
		require.Equal(t, "UNSIGNED-PAYLOAD", r.Header.Get("X-Amz-Content-Sha256"))
		require.Contains(t, r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=test-ak/")

		var err error
		received, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	storage, err := NewS3DesktopUpdateArtifactStorage(context.Background(), &config.DesktopUpdateStorageConfig{
		Endpoint: server.URL, Region: "cn-hangzhou", Bucket: "desktop-releases",
		AccessKeyID: "test-ak", SecretAccessKey: "test-sk", ForcePathStyle: true,
		PublicBaseURL: "https://downloads.example.com",
	})
	require.NoError(t, err)

	key := "desktop-updates/1.2.3/darwin-aarch64/artifact_one.app.tar.gz"
	reader := io.TeeReader(strings.NewReader(payload), io.Discard)
	artifactURL, err := storage.Upload(context.Background(), key, "application/gzip", reader, int64(len(payload)))
	require.NoError(t, err)
	require.Equal(t, payload, string(received))
	require.Equal(t, "https://downloads.example.com/"+key, artifactURL)
}
