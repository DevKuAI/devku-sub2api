package bi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type qrTransport func(*http.Request) (*http.Response, error)

func (f qrTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func qrResponse(data []byte) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), Header: make(http.Header)}
}

func testQRImage(t *testing.T) []byte {
	t.Helper()
	var data bytes.Buffer
	require.NoError(t, png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 32, 32))))
	return data.Bytes()
}

func TestWeChatQRCodeCachesTokenAcrossConcurrentCalls(t *testing.T) {
	picture := testQRImage(t)
	w := newWeChatClient("test-app", "test-secret")
	var tokenCalls, imageCalls atomic.Int32
	w.client.Transport = qrTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" || r.URL.Host != "api.weixin.qq.com" {
			return nil, errors.New("unexpected request")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return nil, err
		}
		if r.URL.Path == "/cgi-bin/stable_token" {
			if body["appid"] != "test-app" || body["secret"] != "test-secret" || body["force_refresh"] != false || body["grant_type"] != "client_credential" {
				return nil, errors.New("incorrect stable token request")
			}
			tokenCalls.Add(1)
			return qrResponse([]byte(`{"access_token":"cached-token","expires_in":7200}`)), nil
		}
		if r.URL.Path != "/wxa/getwxacodeunlimit" || r.URL.Query().Get("access_token") != "cached-token" || body["page"] != invitationPage || body["scene"] != strings.Repeat("a", 32) || body["check_path"] != true || body["env_version"] != "release" {
			return nil, errors.New("incorrect QR request")
		}
		imageCalls.Add(1)
		return qrResponse(picture), nil
	})
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			data, mime, err := w.UnlimitedQRCode(context.Background(), invitationPage, strings.Repeat("a", 32), "release")
			if err != nil || mime != "image/png" || !bytes.Equal(data, picture) {
				t.Errorf("QR request failed: %v, %s", err, mime)
			}
		})
	}
	wg.Wait()
	require.EqualValues(t, 1, tokenCalls.Load())
	require.EqualValues(t, 10, imageCalls.Load())
	w.tokenExpiresAt = time.Now().Add(-time.Second)
	_, _, err := w.UnlimitedQRCode(context.Background(), invitationPage, strings.Repeat("a", 32), "release")
	require.NoError(t, err)
	require.EqualValues(t, 2, tokenCalls.Load())
}

func TestWeChatQRCodeRefreshesRejectedTokenOnce(t *testing.T) {
	for _, code := range []int{40001, 40014, 42001} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			w := newWeChatClient("app", "secret")
			w.accessToken, w.tokenExpiresAt = "stale", time.Now().Add(time.Hour)
			calls, refreshes := 0, 0
			picture := testQRImage(t)
			w.client.Transport = qrTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/cgi-bin/stable_token" {
					refreshes++
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					require.Equal(t, false, payload["force_refresh"])
					return qrResponse([]byte(`{"access_token":"fresh","expires_in":7200}`)), nil
				}
				calls++
				if r.URL.Query().Get("access_token") == "stale" {
					body, err := json.Marshal(map[string]int{"errcode": code})
					require.NoError(t, err)
					return qrResponse(body), nil
				}
				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.Equal(t, false, body["check_path"])
				return qrResponse(picture), nil
			})
			_, _, err := w.UnlimitedQRCode(context.Background(), invitationPage, strings.Repeat("a", 32), "trial")
			require.NoError(t, err)
			require.Equal(t, 2, calls)
			require.Equal(t, 1, refreshes)
		})
	}
}

func TestWeChatQRCodeRejectsFailuresWithoutLeakingCredentials(t *testing.T) {
	for _, body := range [][]byte{[]byte(`{"errcode":41030,"errmsg":"secret token scene"}`), []byte(`{"errcode":0}`), []byte("<html>secret</html>"), bytes.Repeat([]byte("x"), maxWeChatImageBytes+1), testQRImage(t)[:30]} {
		w := newWeChatClient("app", "secret")
		w.accessToken, w.tokenExpiresAt = "token", time.Now().Add(time.Hour)
		w.client.Transport = qrTransport(func(*http.Request) (*http.Response, error) { return qrResponse(body), nil })
		data, mime, err := w.UnlimitedQRCode(context.Background(), invitationPage, strings.Repeat("a", 32), "release")
		require.Empty(t, data)
		require.Empty(t, mime)
		require.EqualError(t, err, weChatQRCodeUnavailable().Error())
	}
	for _, failure := range []string{"transport", "status", "token", "expired_again"} {
		t.Run(failure, func(t *testing.T) {
			w := newWeChatClient("app", "secret")
			calls := 0
			w.client.Transport = qrTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if failure == "transport" {
					return nil, errors.New("secret in URL https://api.weixin.qq.com/?access_token=token")
				}
				if failure == "status" {
					resp := qrResponse([]byte("secret"))
					resp.StatusCode = 500
					return resp, nil
				}
				if failure == "token" {
					return qrResponse([]byte(`{"access_token":"secret","expires_in":0}`)), nil
				}
				if r.URL.Path == "/cgi-bin/stable_token" {
					return qrResponse([]byte(`{"access_token":"token","expires_in":7200}`)), nil
				}
				return qrResponse([]byte(`{"errcode":42001,"errmsg":"secret"}`)), nil
			})
			_, _, err := w.UnlimitedQRCode(context.Background(), invitationPage, strings.Repeat("a", 32), "release")
			require.EqualError(t, err, weChatQRCodeUnavailable().Error())
			require.LessOrEqual(t, calls, 4, "retry must be bounded")
		})
	}
}

func TestInvitationCredentialValidation(t *testing.T) {
	scene, err := newInvitationScene()
	require.NoError(t, err)
	require.Regexp(t, invitationScenePattern, scene)
	other, err := newInvitationScene()
	require.NoError(t, err)
	require.NotEqual(t, scene, other)
	token := randomToken("bii_")
	column, hash, err := (InvitationCredential{Scene: &scene}).lookup()
	require.NoError(t, err)
	require.Equal(t, "i.scene_hash", column)
	require.Equal(t, tokenHash(scene), hash)
	column, hash, err = (InvitationCredential{Token: &token}).lookup()
	require.NoError(t, err)
	require.Equal(t, "i.token_hash", column)
	require.Equal(t, tokenHash(token), hash)
	for _, c := range []InvitationCredential{{}, {Token: &token, Scene: &scene}, {Scene: &token}} {
		_, _, err = c.lookup()
		require.Error(t, err)
	}
	for _, invalidScene := range []string{"", strings.Repeat("a", 31), strings.Repeat("a", 33), strings.Repeat("%", 32), strings.Repeat("中", 32)} {
		_, _, err = (InvitationCredential{Scene: &invalidScene}).lookup()
		require.Error(t, err)
	}
}
