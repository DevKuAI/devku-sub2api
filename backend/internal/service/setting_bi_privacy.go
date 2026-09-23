package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const SettingKeyBIPrivacyNotice = "bi_privacy_notice"
const BIPrivacyNoticePath = "/legal/bi-privacy"

// BIPrivacyNotice is independent of the web login agreement and its revision.
type BIPrivacyNotice struct {
	SiteURL   string `json:"site_url"`
	Title     string `json:"title"`
	Version   string `json:"version"`
	ContentMD string `json:"content_md"`
	UpdatedAt string `json:"updated_at"`
	URL       string `json:"url"`
}

func (n BIPrivacyNotice) Published() bool {
	return strings.TrimSpace(n.Title) != "" && strings.TrimSpace(n.Version) != "" && strings.TrimSpace(n.ContentMD) != ""
}

// GetBIPrivacyNotice reads a single document snapshot without a process-local cache.
func (s *SettingService) GetBIPrivacyNotice(ctx context.Context) (*BIPrivacyNotice, error) {
	values, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyBIPrivacyNotice})
	if err != nil {
		return nil, fmt.Errorf("get BI privacy notice: %w", err)
	}
	notice := &BIPrivacyNotice{}
	if raw := values[SettingKeyBIPrivacyNotice]; raw != "" {
		if err := json.Unmarshal([]byte(raw), notice); err != nil {
			return nil, fmt.Errorf("decode BI privacy notice: %w", err)
		}
	}
	// Use only the BI-specific origin, never shared settings or request headers.
	notice.URL = ""
	if origin, err := normalizeBIPrivacySiteURL(notice.SiteURL); err == nil {
		notice.SiteURL = origin
		notice.URL = origin + BIPrivacyNoticePath
	}
	return notice, nil
}

func normalizeBIPrivacySiteURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.ForceQuery || u.RawQuery != "" || strings.Contains(raw, "#") || (u.Path != "" && u.Path != "/") {
		return "", infraerrors.BadRequest("INVALID_BI_PRIVACY_SITE_URL", "BI privacy site URL must be an HTTPS origin of at most 2048 bytes without credentials, path, query, or fragment")
	}
	u.Path, u.RawPath = "", ""
	return u.String(), nil
}

func (s *SettingService) UpdateBIPrivacyNotice(ctx context.Context, title, version, contentMD, siteURL string) (*BIPrivacyNotice, error) {
	origin, err := normalizeBIPrivacySiteURL(siteURL)
	if err != nil {
		return nil, err
	}
	notice := BIPrivacyNotice{SiteURL: origin, Title: strings.TrimSpace(title), Version: strings.TrimSpace(version), ContentMD: strings.TrimSpace(contentMD)}
	if !notice.Published() || utf8.RuneCountInString(notice.Title) > 80 || utf8.RuneCountInString(notice.Version) > 64 || strings.ContainsFunc(notice.Version, unicode.IsControl) || len(notice.ContentMD) > 200*1024 {
		return nil, infraerrors.BadRequest("INVALID_BI_PRIVACY_NOTICE", "Title (1-80 characters), version (1-64 characters without control characters), and Markdown content (1-204800 bytes) are required")
	}
	previous, err := s.GetBIPrivacyNotice(ctx)
	if err != nil {
		return nil, err
	}
	if previous.Published() && previous.Version == notice.Version {
		if previous.Title != notice.Title || previous.ContentMD != notice.ContentMD {
			return nil, infraerrors.BadRequest("BI_PRIVACY_VERSION_REQUIRED", "Change the privacy notice version when updating its title or content")
		}
		if previous.SiteURL == notice.SiteURL {
			return previous, nil
		}
		// Changing only the hosting origin does not change the legal document.
		notice.UpdatedAt = previous.UpdatedAt
	}
	if notice.UpdatedAt == "" {
		notice.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	raw, err := json.Marshal(notice)
	if err != nil {
		return nil, err
	}
	// Store the content and version atomically; URL is always derived on read.
	if err := s.settingRepo.Set(ctx, SettingKeyBIPrivacyNotice, string(raw)); err != nil {
		return nil, fmt.Errorf("save BI privacy notice: %w", err)
	}
	notice.URL = notice.SiteURL + BIPrivacyNoticePath
	return &notice, nil
}
