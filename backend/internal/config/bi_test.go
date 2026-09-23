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
