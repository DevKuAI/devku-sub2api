package routes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type rolloutIdentityRepo struct {
	service.DesktopRepository
	member *service.DesktopAuthorizedMember
}

func (r *rolloutIdentityRepo) GetAuthorizedMember(context.Context, string) (*service.DesktopAuthorizedMember, error) {
	return r.member, nil
}

type rolloutSessionStore struct {
	service.DesktopSessionStore
	session *service.DesktopSession
}

func (s *rolloutSessionStore) Get(context.Context, string) (*service.DesktopSession, error) {
	return s.session, nil
}

type rolloutCheckRepo struct {
	service.DesktopUpdateRepository
	releases []service.DesktopUpdateRelease
}

func (r *rolloutCheckRepo) ListPublished(context.Context) ([]service.DesktopUpdateRelease, error) {
	return r.releases, nil
}

func TestDesktopUpdateAudienceRequiresVerifiedSessionAndActiveEnterprise(t *testing.T) {
	gin.SetMode(gin.TestMode)
	installation := "6b8123f4-e74e-4a2a-b1ad-b5d53b915b52"
	identity := &rolloutIdentityRepo{member: &service.DesktopAuthorizedMember{
		Member:       &service.DesktopMember{PublicID: "mem_1", Status: "active", AuthVersion: 1},
		Organization: &service.DesktopOrganization{PublicID: "org_1", GatewayUserID: 7, Status: "active", AuthVersion: 1},
		GatewayUser:  &service.User{ID: 7, Status: "active"},
	}}
	store := &rolloutSessionStore{session: &service.DesktopSession{MemberPublicID: "mem_1", OrganizationPublicID: "org_1", InstallationID: installation, MemberVersion: 1, OrganizationVersion: 1}}
	desktop := service.NewDesktopService(identity, nil, nil, nil, nil, nil, &config.Config{}, store, nil, nil)
	now := time.Now().UTC()
	artifact := service.DesktopUpdateArtifact{URL: "https://downloads.example.com/update.nsis.zip", Signature: "sig", ObjectKey: "updates/file", FileName: "update.nsis.zip", Size: 3, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	updates := service.NewDesktopUpdateService(&rolloutCheckRepo{releases: []service.DesktopUpdateRelease{
		{Version: "1.0.1", Status: "published", RolloutMode: "full", PublishedAt: &now, Artifacts: service.DesktopUpdateArtifacts{service.DesktopUpdateWindowsX64: artifact}},
		{Version: "1.0.2", Status: "published", RolloutMode: "gray", GatewayUserIDs: []int64{7}, PublishedAt: &now, Artifacts: service.DesktopUpdateArtifacts{service.DesktopUpdateWindowsX64: artifact}},
	}})
	router := gin.New()
	router.GET("/update/:target/:arch/:current_version", middleware.DesktopOptionalSessionAuth(desktop), handler.NewDesktopUpdateHandler(updates).Check)
	token := "dks_" + base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	request := func(credential, installationID, want string) {
		req := httptest.NewRequest(http.MethodGet, "/update/windows/x86_64/1.0.0?gateway_user_id=7", nil)
		req.Header.Set("Authorization", "Bearer "+credential)
		req.Header.Set("X-Installation-ID", installationID)
		req.Header.Set("X-Gateway-User-ID", "7")
		out := httptest.NewRecorder()
		router.ServeHTTP(out, req)
		require.Equal(t, http.StatusOK, out.Code, out.Body.String())
		require.Equal(t, "no-store", out.Header().Get("Cache-Control"))
		var result service.DesktopUpdateCheckResult
		require.NoError(t, json.Unmarshal(out.Body.Bytes(), &result))
		require.Equal(t, want, result.Version)
	}
	request("", installation, "1.0.1")
	request("forged", installation, "1.0.1")
	request(token, installation, "1.0.2")
	identity.member.Member.PublicID = "mem_2"
	store.session.MemberPublicID = "mem_2"
	request(token, installation, "1.0.2")
	request(token, "34df1c53-5579-474e-919e-cf417404cdf5", "1.0.1")
	identity.member.Organization.GatewayUserID = 8
	request(token, installation, "1.0.1")
	identity.member.Organization.GatewayUserID = 7
	identity.member.Organization.Status = "disabled"
	request(token, installation, "1.0.1")
	identity.member.Organization.Status = "active"
	identity.member.GatewayUser.Status = "disabled"
	request(token, installation, "1.0.1")
	identity.member.GatewayUser.Status = "active"
	identity.member.Member.AuthVersion = 2
	request(token, installation, "1.0.1")
}
