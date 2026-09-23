package config

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBIConfigRequiresIndependentSecretsAndRealEnvironment(t *testing.T) {
	valid := Config{BI: BIConfig{Enabled: true, AppID: "wx_test", AppSecret: "test-app-secret",
		JWTSecret:        base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901")),
		IdentitySecret:   base64.StdEncoding.EncodeToString([]byte("11234567890123456789012345678901")),
		PrivacyNoticeURL: "https://example.com/privacy", PrivacyNoticeVersion: "2026-09-23", MinClientVersion: "0.1.0"}}
	require.NoError(t, valid.validateBI())
	for _, tc := range []struct {
		name string
		edit func(*Config)
	}{
		{"missing app", func(c *Config) { c.BI.AppID = "" }},
		{"short secret", func(c *Config) { c.BI.JWTSecret = "dGVzdA==" }},
		{"same BI secrets", func(c *Config) { c.BI.IdentitySecret = c.BI.JWTSecret }},
		{"same Desktop secret", func(c *Config) { c.Desktop.JWTSecret = c.BI.JWTSecret }},
		{"same raw web secret", func(c *Config) { c.JWT.Secret = "01234567890123456789012345678901" }},
		{"insecure privacy", func(c *Config) { c.BI.PrivacyNoticeURL = "http://example.com/privacy" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid
			tc.edit(&cfg)
			require.Error(t, cfg.validateBI())
			cfg.BI.Enabled = false
			require.NoError(t, cfg.validateBI())
		})
	}
}

func TestBIConfigLoadsEveryEnvironmentSetting(t *testing.T) {
	resetViperWithJWTSecret(t)
	t.Setenv("CONFIG_FILE", "")
	t.Setenv("DATA_DIR", t.TempDir())
	want := BIConfig{Enabled: true, AppID: "wx_test", AppSecret: "test-app-secret",
		JWTSecret:        base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901")),
		IdentitySecret:   base64.StdEncoding.EncodeToString([]byte("11234567890123456789012345678901")),
		PrivacyNoticeURL: "https://example.com/privacy", PrivacyNoticeVersion: "test-v2", MinClientVersion: "0.2.0", ReportRetentionMonths: 12}
	for key, value := range map[string]string{
		"BI_ENABLED": "true", "BI_APPID": want.AppID, "BI_APP_SECRET": want.AppSecret,
		"BI_JWT_SECRET": want.JWTSecret, "BI_IDENTITY_SECRET": want.IdentitySecret,
		"BI_PRIVACY_NOTICE_URL": want.PrivacyNoticeURL, "BI_PRIVACY_NOTICE_VERSION": want.PrivacyNoticeVersion,
		"BI_MIN_CLIENT_VERSION": want.MinClientVersion, "BI_REPORT_RETENTION_MONTHS": "12",
	} {
		t.Setenv(key, value)
	}
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, want, cfg.BI)
}
