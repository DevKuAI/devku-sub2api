package bi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"
)

// WeChatExchanger never exposes session_key, AppSecret or openid to the client.
type WeChatExchanger interface {
	Exchange(context.Context, string) (string, error)
}

type weChatClient struct {
	client *http.Client
	appID  string
	secret string
}

func newWeChatClient(appID, secret string) *weChatClient {
	return &weChatClient{appID: appID, secret: secret, client: &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (w *weChatClient) Exchange(ctx context.Context, code string) (string, error) {
	query := url.Values{"appid": {w.appID}, "secret": {w.secret}, "js_code": {code}, "grant_type": {"authorization_code"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.weixin.qq.com/sns/jscode2session?"+query.Encode(), nil)
	if err != nil {
		return "", apiError(503, "UPSTREAM_UNAVAILABLE", "WeChat authentication is unavailable")
	}
	resp, err := w.client.Do(req)
	if err != nil {
		// Transport errors contain the URL and therefore must not escape this boundary.
		return "", apiError(503, "UPSTREAM_UNAVAILABLE", "WeChat authentication is unavailable")
	}
	defer func() { _ = resp.Body.Close() }()
	var result struct {
		OpenID  string `json:"openid"`
		ErrCode int    `json:"errcode"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&result) != nil {
		return "", apiError(503, "UPSTREAM_UNAVAILABLE", "WeChat authentication is unavailable")
	}
	if result.ErrCode == 40029 || result.ErrCode == 40163 {
		return "", invalid("code", "WeChat code is invalid or already used")
	}
	if result.ErrCode != 0 || len(result.OpenID) == 0 || len(result.OpenID) > 256 {
		return "", apiError(503, "UPSTREAM_UNAVAILABLE", "WeChat authentication is unavailable")
	}
	return result.OpenID, nil
}
