package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBIPrivacyNoticePublicationAndVersion(t *testing.T) {
	ctx := context.Background()
	repo := &panelRateLimitSettingRepo{values: map[string]string{SettingKeyFrontendURL: "https://example.com/", SettingKeyLoginAgreementDocuments: "untouched"}}
	svc := NewSettingService(repo, &config.Config{})
	notice, err := svc.GetBIPrivacyNotice(ctx)
	require.NoError(t, err)
	require.False(t, notice.Published())
	require.Empty(t, notice.URL)

	notice, err = svc.UpdateBIPrivacyNotice(ctx, " Privacy ", " v1 ", " # Scope\n\nTest content ", " https://example.com/ ")
	require.NoError(t, err)
	require.True(t, notice.Published())
	require.Equal(t, "Privacy", notice.Title)
	require.Equal(t, "v1", notice.Version)
	require.Equal(t, "https://example.com", notice.SiteURL)
	require.Equal(t, "https://example.com/legal/bi-privacy", notice.URL)
	require.NotEmpty(t, notice.UpdatedAt)
	// A fresh service observes the saved record without relying on memory state.
	stored, err := NewSettingService(repo, &config.Config{}).GetBIPrivacyNotice(ctx)
	require.NoError(t, err)
	require.Equal(t, notice, stored)
	_, err = svc.UpdateBIPrivacyNotice(ctx, "Privacy", "v1", "Changed", "https://example.com")
	require.ErrorContains(t, err, "Change the privacy notice version")
	stored, err = svc.GetBIPrivacyNotice(ctx)
	require.NoError(t, err)
	require.Equal(t, notice, stored)
	updated, err := svc.UpdateBIPrivacyNotice(ctx, "Privacy", "v2", "Changed", "https://example.com")
	require.NoError(t, err)
	require.Equal(t, "v2", updated.Version)
	require.Equal(t, "Changed", updated.ContentMD)
	require.Equal(t, "untouched", repo.values[SettingKeyLoginAgreementDocuments])
	unchanged, err := svc.UpdateBIPrivacyNotice(ctx, "Privacy", "v2", "Changed", "https://example.com")
	require.NoError(t, err)
	require.Equal(t, updated, unchanged)
}

func TestBIPrivacyNoticeValidation(t *testing.T) {
	for _, tc := range []struct{ title, version, content string }{
		{"", "v1", "content"}, {"Privacy", "", "content"}, {"Privacy", "v1", " \n "},
		{strings.Repeat("隐", 81), "v1", "content"}, {"Privacy", strings.Repeat("v", 65), "content"},
		{"Privacy", "v1\nv2", "content"}, {"Privacy", "v1", strings.Repeat("a", 200*1024+1)},
	} {
		repo := &panelRateLimitSettingRepo{}
		_, err := NewSettingService(repo, &config.Config{}).UpdateBIPrivacyNotice(context.Background(), tc.title, tc.version, tc.content, "https://example.com")
		require.Error(t, err)
		require.Empty(t, repo.values)
	}
}

func TestBIPrivacyNoticeUsesOnlyConfiguredHTTPSOrigin(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct{ base, want string }{
		{" https://example.com/ ", "https://example.com/legal/bi-privacy"},
		{"https://example.com:8443/", "https://example.com:8443/legal/bi-privacy"},
		{"", ""}, {"http://example.com", ""}, {"//example.com", ""}, {"https://user:pass@example.com", ""},
		{"https://example.com/prefix", ""}, {"https://example.com?token=secret", ""}, {"https://example.com?", ""},
		{"https://example.com/#fragment", ""}, {"https://example.com/#", ""}, {"https://example.com:bad", ""},
		{"https://" + strings.Repeat("a", 2041), ""},
	} {
		t.Run(tc.base, func(t *testing.T) {
			raw, err := json.Marshal(BIPrivacyNotice{SiteURL: tc.base, Title: "Privacy", Version: "v1", ContentMD: "content", URL: "https://stale.example.com/privacy"})
			require.NoError(t, err)
			repo := &panelRateLimitSettingRepo{values: map[string]string{SettingKeyBIPrivacyNotice: string(raw), SettingKeyFrontendURL: "https://web.example.com"}}
			cfg := &config.Config{Server: config.ServerConfig{FrontendURL: "https://fallback.example.com"}}
			svc := NewSettingService(repo, cfg)
			notice, err := svc.GetBIPrivacyNotice(ctx)
			require.NoError(t, err)
			require.Equal(t, tc.want, notice.URL)
			_, err = svc.UpdateBIPrivacyNotice(ctx, "Privacy", "v2", "content", tc.base)
			if tc.want == "" {
				require.ErrorContains(t, err, "HTTPS origin")
				require.Equal(t, string(raw), repo.values[SettingKeyBIPrivacyNotice])
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestBIPrivacySiteChangePreservesDocumentVersionAndSharedSettings(t *testing.T) {
	ctx := context.Background()
	repo := &panelRateLimitSettingRepo{values: map[string]string{SettingKeyFrontendURL: "https://web.example.com"}}
	cfg := &config.Config{Server: config.ServerConfig{FrontendURL: "https://fallback.example.com"}}
	svc := NewSettingService(repo, cfg)
	notice, err := svc.GetBIPrivacyNotice(ctx)
	require.NoError(t, err)
	require.Empty(t, notice.SiteURL)
	require.Empty(t, notice.URL)
	previous, err := svc.UpdateBIPrivacyNotice(ctx, "Privacy", "v1", "content", "https://first.example.com")
	require.NoError(t, err)
	moved, err := svc.UpdateBIPrivacyNotice(ctx, "Privacy", "v1", "content", "https://second.example.com/")
	require.NoError(t, err)
	require.Equal(t, previous.Version, moved.Version)
	require.Equal(t, previous.UpdatedAt, moved.UpdatedAt)
	require.Equal(t, "https://second.example.com/legal/bi-privacy", moved.URL)
	stored, err := svc.GetBIPrivacyNotice(ctx)
	require.NoError(t, err)
	require.Equal(t, moved, stored)
	require.Equal(t, "https://web.example.com", repo.values[SettingKeyFrontendURL])
	require.Equal(t, "https://fallback.example.com", cfg.Server.FrontendURL)
}

type failingBIPrivacyRepo struct {
	SettingRepository
	readErr, writeErr error
}

func (r failingBIPrivacyRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, r.readErr
}

func (r failingBIPrivacyRepo) Set(context.Context, string, string) error { return r.writeErr }

func TestBIPrivacyNoticeDoesNotHideStorageFailures(t *testing.T) {
	for _, repo := range []SettingRepository{
		failingBIPrivacyRepo{readErr: errors.New("read failed")},
		failingBIPrivacyRepo{writeErr: errors.New("write failed")},
		&panelRateLimitSettingRepo{values: map[string]string{SettingKeyBIPrivacyNotice: "broken JSON"}},
	} {
		_, err := NewSettingService(repo, &config.Config{}).UpdateBIPrivacyNotice(context.Background(), "Privacy", "v1", "content", "https://example.com")
		require.Error(t, err)
	}
}
