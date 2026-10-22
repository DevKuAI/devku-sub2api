package config

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func validDesktopConfig() DesktopConfig {
	secret := base64.StdEncoding.EncodeToString(make([]byte, 32))
	return DesktopConfig{
		Enabled:                     true,
		ConversationMemberPerMinute: 60,
		JWTSecret:                   secret,
		PublicGatewayBaseURL:        "https://gateway.example.com/v1",
		AccessTokenTTLMinutes:       15,
		RefreshFamilyTTLDays:        30,
		LookupIPPerMinute:           10,
		LoginIPPerMinute:            10,
		LoginOrganizationPerMinute:  30,
		LoginPhoneFailureLimit:      5,
		LoginPhoneFreezeMinutes:     15,
	}
}

func TestDesktopConfigDisabledSkipsSecretValidation(t *testing.T) {
	cfg := &Config{Desktop: DesktopConfig{Enabled: false}}
	require.NoError(t, cfg.validateDesktop())
}

func TestDesktopConfigValid(t *testing.T) {
	cfg := &Config{Desktop: validDesktopConfig()}
	require.NoError(t, cfg.validateDesktop())
}

func TestDesktopReportResourceDefaultsAndEnvironment(t *testing.T) {
	resetViperWithJWTSecret(t)
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, 1, cfg.Desktop.ReportWorkers)
	require.Equal(t, 300, cfg.Desktop.ReportScanIntervalSeconds)
	require.Equal(t, 600, cfg.Desktop.ReportCleanupIntervalSeconds)
	t.Setenv("DESKTOP_REPORT_WORKERS", "3")
	t.Setenv("DESKTOP_REPORT_SCAN_INTERVAL_SECONDS", "120")
	t.Setenv("DESKTOP_REPORT_CLEANUP_INTERVAL_SECONDS", "1800")
	cfg, err = Load()
	require.NoError(t, err)
	require.Equal(t, 3, cfg.Desktop.ReportWorkers)
	require.Equal(t, 120, cfg.Desktop.ReportScanIntervalSeconds)
	require.Equal(t, 1800, cfg.Desktop.ReportCleanupIntervalSeconds)
}

func TestDesktopReportResourceIntervalsRejectInvalidValues(t *testing.T) {
	for _, value := range []int{-1, 86401} {
		cfg := &Config{Desktop: validDesktopConfig()}
		cfg.Desktop.ReportScanIntervalSeconds = value
		require.ErrorContains(t, cfg.validateDesktop(), "report_scan_interval_seconds")
		cfg.Desktop.ReportScanIntervalSeconds = 0
		cfg.Desktop.ReportCleanupIntervalSeconds = value
		require.ErrorContains(t, cfg.validateDesktop(), "report_cleanup_interval_seconds")
	}
}

func TestDesktopConfigRejectsInvalidSecretsAndGateway(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*DesktopConfig)
		match  string
	}{
		{name: "jwt not base64", mutate: func(c *DesktopConfig) { c.JWTSecret = "not-base64" }, match: "standard base64"},
		{name: "jwt too short", mutate: func(c *DesktopConfig) { c.JWTSecret = base64.StdEncoding.EncodeToString(make([]byte, 31)) }, match: "at least 32 bytes"},
		{name: "gateway insecure", mutate: func(c *DesktopConfig) { c.PublicGatewayBaseURL = "http://gateway.example.com/v1" }, match: "HTTPS URL"},
		{name: "gateway path", mutate: func(c *DesktopConfig) { c.PublicGatewayBaseURL = "https://gateway.example.com" }, match: "ending exactly in /v1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desktop := validDesktopConfig()
			tt.mutate(&desktop)
			err := (&Config{Desktop: desktop}).validateDesktop()
			require.ErrorContains(t, err, tt.match)
		})
	}
}
