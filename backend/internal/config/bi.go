package config

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"golang.org/x/mod/semver"
)

// BIConfig keeps mini-program credentials separate from web and Desktop identities.
type BIConfig struct {
	Enabled          bool   `mapstructure:"enabled"`
	AppID            string `mapstructure:"appid"`
	AppSecret        string `mapstructure:"app_secret"`
	JWTSecret        string `mapstructure:"jwt_secret"`
	IdentitySecret   string `mapstructure:"identity_secret"`
	MinClientVersion string `mapstructure:"min_client_version"`
}

func setBIDefaults() {
	viper.SetDefault("bi.enabled", false)
	for _, key := range []string{"appid", "app_secret", "jwt_secret", "identity_secret"} {
		viper.SetDefault("bi."+key, "")
	}
	viper.SetDefault("bi.min_client_version", "0.2.0")
}

func (c *Config) validateBI() error {
	if !c.BI.Enabled {
		return nil
	}
	for name, value := range map[string]string{
		"appid": c.BI.AppID, "app_secret": c.BI.AppSecret, "min_client_version": c.BI.MinClientVersion,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("bi.%s is required when BI is enabled", name)
		}
	}
	if !semver.IsValid("v" + c.BI.MinClientVersion) {
		return fmt.Errorf("bi.min_client_version must be a semantic version")
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
	return nil
}
