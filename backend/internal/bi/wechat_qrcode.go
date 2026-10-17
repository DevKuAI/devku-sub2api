package bi

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	_ "image/jpeg" // WeChat may return JPEG or PNG codes.
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"time"
)

const maxWeChatImageBytes = 2 << 20

func weChatQRCodeUnavailable() *Error {
	return apiError(503, "UPSTREAM_UNAVAILABLE", "WeChat QR code generation is unavailable")
}

// stableAccessToken serializes refreshes per client. Refreshes use normal mode:
// forcing a refresh would invalidate tokens held by other server instances.
func (w *weChatClient) stableAccessToken(ctx context.Context, rejected string) (string, error) {
	w.tokenMu.Lock()
	defer w.tokenMu.Unlock()
	if w.accessToken != "" && w.accessToken != rejected && time.Now().Before(w.tokenExpiresAt) {
		return w.accessToken, nil
	}
	if w.appID == "" || w.secret == "" {
		return "", weChatQRCodeUnavailable()
	}
	// Do not reuse an invalid token if this refresh fails.
	w.accessToken, w.tokenExpiresAt = "", time.Time{}
	body, err := json.Marshal(struct {
		GrantType    string `json:"grant_type"`
		AppID        string `json:"appid"`
		Secret       string `json:"secret"`
		ForceRefresh bool   `json:"force_refresh"`
	}{"client_credential", w.appID, w.secret, false})
	if err != nil {
		return "", weChatQRCodeUnavailable()
	}
	data, err := w.weChatPost(ctx, "https://api.weixin.qq.com/cgi-bin/stable_token", body, 64<<10)
	if err != nil {
		return "", err
	}
	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
	}
	if json.Unmarshal(data, &result) != nil || result.ErrCode != 0 || result.AccessToken == "" || len(result.AccessToken) > 4096 || result.ExpiresIn <= 0 || result.ExpiresIn > 7200 {
		return "", weChatQRCodeUnavailable()
	}
	w.accessToken = result.AccessToken
	w.tokenExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn-min(60, result.ExpiresIn/10)) * time.Second)
	return w.accessToken, nil
}

func (w *weChatClient) weChatPost(ctx context.Context, endpoint string, body []byte, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, weChatQRCodeUnavailable()
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.client.Do(req)
	if err != nil {
		// Neither upstream bodies nor transport errors may escape: they can contain credentials.
		return nil, weChatQRCodeUnavailable()
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, weChatQRCodeUnavailable()
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, weChatQRCodeUnavailable()
	}
	return data, nil
}

func (w *weChatClient) UnlimitedQRCode(ctx context.Context, page, scene, envVersion string) ([]byte, string, error) {
	body, err := json.Marshal(struct {
		Page       string `json:"page"`
		Scene      string `json:"scene"`
		EnvVersion string `json:"env_version"`
		CheckPath  bool   `json:"check_path"`
		Width      int    `json:"width"`
	}{page, scene, envVersion, envVersion == "release", 430})
	if err != nil {
		return nil, "", weChatQRCodeUnavailable()
	}
	token, err := w.stableAccessToken(ctx, "")
	if err != nil {
		return nil, "", err
	}
	for attempt := 0; attempt < 2; attempt++ {
		data, err := w.weChatPost(ctx, "https://api.weixin.qq.com/wxa/getwxacodeunlimit?"+url.Values{"access_token": {token}}.Encode(), body, maxWeChatImageBytes)
		if err != nil {
			return nil, "", err
		}
		var failure struct {
			ErrCode int `json:"errcode"`
		}
		if json.Unmarshal(data, &failure) == nil {
			if attempt == 0 && (failure.ErrCode == 40001 || failure.ErrCode == 40014 || failure.ErrCode == 42001) {
				token, err = w.stableAccessToken(ctx, token)
				if err != nil {
					return nil, "", err
				}
				continue
			}
			return nil, "", weChatQRCodeUnavailable()
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 1280 || cfg.Height > 1280 || (format != "png" && format != "jpeg") {
			return nil, "", weChatQRCodeUnavailable()
		}
		if _, _, err = image.Decode(bytes.NewReader(data)); err != nil {
			return nil, "", weChatQRCodeUnavailable()
		}
		return data, "image/" + format, nil
	}
	return nil, "", weChatQRCodeUnavailable()
}
