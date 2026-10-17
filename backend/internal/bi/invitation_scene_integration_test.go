//go:build integration

package bi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func invitationRequest(t *testing.T, router http.Handler, session, operation string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/bi/v1/invitations/"+operation, strings.NewReader(string(raw)))
	if session != "" {
		req.Header.Set("Authorization", "Bearer "+session)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestIndependentInvitationSceneHTTPAndLegacyCompatibility(t *testing.T) {
	f := independentFixture(t)
	ctx := context.Background()
	invite, err := f.s.CreateInvitation(ctx, f.org, Capabilities, "", f.owner, "scene", "张经理")
	require.NoError(t, err)
	require.Len(t, invite.Scene, 32)
	require.Len(t, invite.Token, 47)
	var storedScene, storedToken string
	require.NoError(t, f.db.QueryRow(`SELECT scene_hash,token_hash FROM bi_invitations WHERE id=$1`, invite.ID).Scan(&storedScene, &storedToken))
	require.Equal(t, tokenHash(invite.Scene), storedScene)
	require.Equal(t, tokenHash(invite.Token), storedToken)
	user := f.login(t, "scene-user")
	other := f.login(t, "other")
	router := nativeRouter(f)
	unauth := invitationRequest(t, router, "", "preview", gin.H{"scene": invite.Scene})
	require.Equal(t, 401, unauth.Code)
	for _, body := range []gin.H{{}, {"scene": "bad"}, {"scene": invite.Scene, "token": invite.Token}, {"scene": invite.Scene, "token": nil}, {"scene": nil}, {"token": nil}, {"scene": invite.Scene, "unexpected": true}} {
		w := invitationRequest(t, router, user.AccessToken, "preview", body)
		require.Equal(t, 400, w.Code, w.Body.String())
	}
	for _, body := range []gin.H{{"scene": invite.Scene}, {"scene": invite.Scene, "confirm_join": false}} {
		w := invitationRequest(t, router, user.AccessToken, "accept", body)
		require.Equal(t, 400, w.Code)
	}
	w := invitationRequest(t, router, user.AccessToken, "preview", gin.H{"scene": invite.Scene})
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "张经理")
	require.NotContains(t, w.Body.String(), invite.Scene)
	var status string
	require.NoError(t, f.db.QueryRow(`SELECT status FROM bi_invitations WHERE id=$1`, invite.ID).Scan(&status))
	require.Equal(t, "pending", status, "preview and unconfirmed requests must not consume the invitation")
	w = invitationRequest(t, router, user.AccessToken, "accept", gin.H{"scene": invite.Scene, "confirm_join": true})
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"organization_id":"`+f.org+`"`)
	// Both credentials address one invitation; replay is scoped to the same user.
	w = invitationRequest(t, router, user.AccessToken, "accept", gin.H{"token": invite.Token, "confirm_join": true})
	require.Equal(t, 200, w.Code, w.Body.String())
	w = invitationRequest(t, router, other.AccessToken, "accept", gin.H{"token": invite.Token, "confirm_join": true})
	require.Equal(t, 404, w.Code)
	w = invitationRequest(t, router, other.AccessToken, "preview", gin.H{"scene": invite.Scene})
	require.Equal(t, 404, w.Code)
	// Existing rows have no scene; their original token still works.
	legacy, err := f.s.CreateInvitation(ctx, f.org, []string{"analytics:read"}, "", f.owner, "legacy")
	require.NoError(t, err)
	_, err = f.db.Exec(`UPDATE bi_invitations SET scene_hash=NULL WHERE id=$1`, legacy.ID)
	require.NoError(t, err)
	w = invitationRequest(t, router, other.AccessToken, "accept", gin.H{"token": legacy.Token, "confirm_join": true})
	require.Equal(t, 200, w.Code, w.Body.String())
	w = invitationRequest(t, router, other.AccessToken, "preview", gin.H{"scene": legacy.Scene})
	require.Equal(t, 404, w.Code)
	_, err = f.db.Exec(`UPDATE bi_access SET status='revoked' WHERE bi_user_id=$1`, user.User.ID)
	require.NoError(t, err)
	w = invitationRequest(t, router, user.AccessToken, "accept", gin.H{"scene": invite.Scene, "confirm_join": true})
	require.Equal(t, 404, w.Code, "revoked access remains hidden on replay")
	for _, state := range []string{"expired", "revoked"} {
		unused, createErr := f.s.CreateInvitation(ctx, f.org, []string{"analytics:read"}, "", f.owner, "invalid")
		require.NoError(t, createErr)
		if state == "expired" {
			_, err = f.db.Exec(`UPDATE bi_invitations SET expires_at=NOW()-INTERVAL '1 second' WHERE id=$1`, unused.ID)
		} else {
			_, err = f.db.Exec(`UPDATE bi_invitations SET status='revoked' WHERE id=$1`, unused.ID)
		}
		require.NoError(t, err)
		w = invitationRequest(t, router, other.AccessToken, "preview", gin.H{"scene": unused.Scene})
		require.Equal(t, 404, w.Code, state)
		w = invitationRequest(t, router, other.AccessToken, "accept", gin.H{"scene": unused.Scene, "confirm_join": true})
		require.Equal(t, 404, w.Code, state)
	}
}

func TestIndependentInvitationSceneAndTokenConcurrentClaim(t *testing.T) {
	f := independentFixture(t)
	invite, err := f.s.CreateInvitation(context.Background(), f.org, []string{"analytics:read"}, "", f.owner, "race")
	require.NoError(t, err)
	users := []*Session{f.login(t, "first"), f.login(t, "second")}
	credentials := []InvitationCredential{{Token: &invite.Token}, {Scene: &invite.Scene}}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := range users {
		wg.Go(func() {
			_, err := f.s.AcceptInvitationCredential(context.Background(), Principal{UserID: users[i].User.ID}, credentials[i], "race")
			results <- err
		})
	}
	wg.Wait()
	close(results)
	wins := 0
	for err := range results {
		if err == nil {
			wins++
		} else {
			require.ErrorIs(t, err, ErrNotFound)
		}
	}
	require.Equal(t, 1, wins)
	var count int
	require.NoError(t, f.db.QueryRow(`SELECT COUNT(*) FROM bi_access`).Scan(&count))
	require.Equal(t, 1, count)
}

type invitationQRStub struct {
	identityWX
	generate func(context.Context, string, string, string) ([]byte, string, error)
}

func (s invitationQRStub) UnlimitedQRCode(ctx context.Context, page, scene, env string) ([]byte, string, error) {
	return s.generate(ctx, page, scene, env)
}

func TestIndependentInvitationQRCodeScopeRetryAndRevocation(t *testing.T) {
	f := independentFixture(t)
	ctx := context.Background()
	invite, err := f.s.CreateInvitation(ctx, f.org, []string{"analytics:read"}, "", f.owner, "qr")
	require.NoError(t, err)
	picture := testQRImage(t)
	calls := 0
	f.s.wechat = invitationQRStub{generate: func(_ context.Context, page, scene, env string) ([]byte, string, error) {
		calls++
		require.Equal(t, invitationPage, page)
		require.Equal(t, invite.Scene, scene)
		require.Equal(t, "release", env)
		if calls == 1 {
			return nil, "", weChatQRCodeUnavailable()
		}
		return picture, "image/png", nil
	}}
	_, err = f.s.GenerateInvitationQRCode(ctx, "other-org", invite.ID, invite.Scene, "", f.owner)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = f.s.GenerateInvitationQRCode(ctx, f.org, "other-invitation", invite.Scene, "", f.owner)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = f.s.GenerateInvitationQRCode(ctx, f.org, invite.ID, strings.Repeat("x", 32), "", f.owner)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = f.s.GenerateInvitationQRCode(ctx, f.org, invite.ID, invite.Scene, "", f.owner+100)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = f.s.GenerateInvitationQRCode(ctx, f.org, invite.ID, invite.Scene, "invalid", f.owner)
	require.Error(t, err)
	require.Zero(t, calls)
	_, err = f.s.GenerateInvitationQRCode(ctx, f.org, invite.ID, invite.Scene, "", f.owner)
	require.EqualError(t, err, weChatQRCodeUnavailable().Error())
	// A retry returns the image in the standard admin envelope, without a new invite.
	r := gin.New()
	r.POST("/invitations/:invitation_id/qrcode", Boundary, func(c *gin.Context) {
		c.Set("bi.managed_org", f.org)
		c.Set("bi.actor", f.owner)
	}, f.h.InvitationQRCode)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/invitations/"+invite.ID+"/qrcode", strings.NewReader(`{"scene":"`+invite.Scene+`"}`)))
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
	var envelope struct {
		Code int              `json:"code"`
		Data InvitationQRCode `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Zero(t, envelope.Code)
	require.Equal(t, base64.StdEncoding.EncodeToString(picture), envelope.Data.ImageBase64)
	require.Equal(t, "image/png", envelope.Data.MIMEType)
	require.Equal(t, invitationPage, envelope.Data.Page)
	require.WithinDuration(t, invite.ExpiresAt, envelope.Data.ExpiresAt, 1000)
	var count int
	require.NoError(t, f.db.QueryRow(`SELECT COUNT(*) FROM bi_invitations`).Scan(&count))
	require.Equal(t, 1, count)
	// Permission changes while WeChat responds must be rechecked before returning the image.
	f.s.wechat = invitationQRStub{generate: func(context.Context, string, string, string) ([]byte, string, error) {
		_, e := f.db.Exec(`UPDATE bi_invitations SET status='revoked' WHERE id=$1`, invite.ID)
		require.NoError(t, e)
		return picture, "image/png", nil
	}}
	result, err := f.s.GenerateInvitationQRCode(ctx, f.org, invite.ID, invite.Scene, "", f.owner)
	require.ErrorIs(t, err, ErrNotFound)
	require.Empty(t, result.ImageBase64)
}
