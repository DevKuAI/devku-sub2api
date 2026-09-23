package config

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/viper"
)

// BIConfig keeps mini-program credentials separate from web and Desktop identities.
type BIConfig struct {
	ReportRetentionMonths int    `mapstructure:"report_retention_months"`
	Enabled               bool   `mapstructure:"enabled"`
	AppID                 string `mapstructure:"appid"`
	AppSecret             string `mapstructure:"app_secret"`
	JWTSecret             string `mapstructure:"jwt_secret"`
	IdentitySecret        string `mapstructure:"identity_secret"`
	PrivacyNoticeURL      string `mapstructure:"privacy_notice_url"`
	PrivacyNoticeVersion  string `mapstructure:"privacy_notice_version"`
	MinClientVersion      string `mapstructure:"min_client_version"`
}

func setBIDefaults() {
	viper.SetDefault("bi.report_retention_months", 24)
	viper.SetDefault("bi.enabled", false)
	for _, key := range []string{"appid", "app_secret", "jwt_secret", "identity_secret", "privacy_notice_url", "privacy_notice_version"} {
		viper.SetDefault("bi."+key, "")
	}
	viper.SetDefault("bi.min_client_version", "0.1.0")
}

func (c *Config) validateBI() error {
	if !c.BI.Enabled {
		return nil
	}
	if c.BI.ReportRetentionMonths < 0 || c.BI.ReportRetentionMonths > 120 {
		return fmt.Errorf("bi.report_retention_months must be between 1 and 120 (zero uses 24)")
	}
	for name, value := range map[string]string{
		"appid": c.BI.AppID, "app_secret": c.BI.AppSecret,
		"privacy_notice_version": c.BI.PrivacyNoticeVersion, "min_client_version": c.BI.MinClientVersion,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("bi.%s is required when BI is enabled", name)
		}
	}
	var secrets [][]byte
	desktopSecret, _ := base64.StdEncoding.Strict().DecodeString(c.Desktop.JWTSecret)
	for name, value := range map[string]string{"jwt_secret": c.BI.JWTSecret, "identity_secret": c.BI.IdentitySecret} {
		decoded, err := base64.StdEncoding.Strict().DecodeString(value)
		if err != nil || len(decoded) < 32 {
			return fmt.Errorf("bi.%s must be standard base64 encoding at least 32 bytes", name)
		}
		if bytes.Equal(decoded, desktopSecret) || value == c.JWT.Secret || string(decoded) == c.JWT.Secret {
			return fmt.Errorf("bi.%s must be independent of web and Desktop secrets", name)
		}
		secrets = append(secrets, decoded)
	}
	if bytes.Equal(secrets[0], secrets[1]) {
		return fmt.Errorf("bi.jwt_secret and bi.identity_secret must be independent")
	}
	u, err := url.Parse(c.BI.PrivacyNoticeURL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return fmt.Errorf("bi.privacy_notice_url must be an absolute HTTPS URL")
	}
	return nil
}
