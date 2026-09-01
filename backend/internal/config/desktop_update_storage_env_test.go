//go:build unit

package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDesktopUpdateStorageFromEnv(t *testing.T) {
	resetViperWithJWTSecret(t)
	t.Setenv("DESKTOP_UPDATE_STORAGE_ENDPOINT", "https://oss-cn-hangzhou.aliyuncs.com")
	t.Setenv("DESKTOP_UPDATE_STORAGE_REGION", "cn-hangzhou")
	t.Setenv("DESKTOP_UPDATE_STORAGE_BUCKET", "desktop-releases")
	t.Setenv("DESKTOP_UPDATE_STORAGE_ACCESS_KEY_ID", "ak")
	t.Setenv("DESKTOP_UPDATE_STORAGE_SECRET_ACCESS_KEY", "sk")
	t.Setenv("DESKTOP_UPDATE_STORAGE_PUBLIC_BASE_URL", "https://downloads.example.com")

	cfg, err := Load()
	require.NoError(t, err)
	require.True(t, cfg.DesktopUpdateStorage.IsConfigured())
	require.Equal(t, "https://oss-cn-hangzhou.aliyuncs.com", cfg.DesktopUpdateStorage.Endpoint)
	require.Equal(t, "cn-hangzhou", cfg.DesktopUpdateStorage.Region)
	require.Equal(t, "desktop-releases", cfg.DesktopUpdateStorage.Bucket)
	require.Equal(t, int64(209715200), cfg.DesktopUpdateStorage.MaxUploadBytes)
}
