//go:build integration

package bi

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"fmt"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type identityWX struct{}

func (identityWX) Exchange(_ context.Context, code string) (string, error) {
	return strings.Split(code, ":")[0], nil
}

type biFixture struct {
	db      *sql.DB
	s       *Service
	h       *Handler
	org     string
	owner   int64
	desktop *service.DesktopService
}

func independentFixture(t *testing.T) *biFixture {
	t.Helper()
	ctx := context.Background()
	container, e := tcpostgres.Run(ctx, "postgres:18.1-alpine3.23", tcpostgres.WithDatabase("bi_test"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("postgres"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, e)
	t.Cleanup(func() { _ = container.Terminate(ctx) })
	dsn, e := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, e)
	db, e := sql.Open("postgres", dsn)
	require.NoError(t, e)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, repository.ApplyMigrations(ctx, db))
	var owner, group, org int64
	require.NoError(t, db.QueryRow(`INSERT INTO users(email,password_hash,role) VALUES('owner@local.test','test-hash','user') RETURNING id`).Scan(&owner))
	require.NoError(t, db.QueryRow(`INSERT INTO groups(name) VALUES('native-group') RETURNING id`).Scan(&group))
	require.NoError(t, db.QueryRow(`INSERT INTO desktop_organizations(public_id,code,name,gateway_user_id,group_id,conversation_reporting_enabled,conversation_summary_enabled,analysis_model) VALUES('org_native','native','企业名称', $1,$2,TRUE,TRUE,'test-model') RETURNING id`, owner, group).Scan(&org))
	cfg := &config.Config{BI: config.BIConfig{Enabled: true, AppID: "wx_test", JWTSecret: base64.StdEncoding.EncodeToString([]byte(strings.Repeat("j", 32))), IdentitySecret: base64.StdEncoding.EncodeToString([]byte(strings.Repeat("i", 32)))}, Desktop: config.DesktopConfig{Enabled: true}}
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	keys := repository.NewAPIKeyRepository(client, db)
	desk := service.ProvideDesktopService(repository.NewDesktopRepository(client, keys), repository.NewUsageLogRepository(client, db).(service.DesktopUsageRepository), nil, nil, nil, nil, cfg, nil, repository.NewDesktopConversationRepository(client), nil, repository.NewDesktopReportRepository(db, client))
	s := NewService(db, cfg.BI, desk, identityWX{})
	r := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: r.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	h := &Handler{service: s, desktopEnabled: true, redis: redisClient}
	return &biFixture{db: db, s: s, h: h, org: "org_native", owner: owner, desktop: desk}
}
func (f *biFixture) login(t *testing.T, identity string) *Session {
	t.Helper()
	v, e := f.s.Login(context.Background(), identity+":"+fmt.Sprint(time.Now().UnixNano()), LoginOptions{ClientVersion: "0.2.0"})
	require.NoError(t, e)
	return v
}
func TestIndependentIdentityInvitationsIsolationReplayAndRevocation(t *testing.T) {
	f := independentFixture(t)
	ctx := context.Background()
	one := f.login(t, "one")
	two := f.login(t, "two")
	require.NotEqual(t, one.User.ID, two.User.ID)
	var webUsers int
	require.NoError(t, f.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&webUsers))
	require.Equal(t, 1, webUsers, "mobile logins must not create web users")
	invite, e := f.s.CreateInvitation(ctx, f.org, []string{"analytics:read"}, "recipient", f.owner, "req")
	require.NoError(t, e)
	p := Principal{UserID: one.User.ID}
	a, e := f.s.AcceptInvitation(ctx, p, invite.Token, "req")
	require.NoError(t, e)
	require.Equal(t, []string{"analytics:read"}, a.Capabilities)
	_, e = f.s.AcceptInvitation(ctx, p, invite.Token, "req")
	require.NoError(t, e)
	_, e = f.s.AcceptInvitation(ctx, Principal{UserID: two.User.ID}, invite.Token, "req")
	require.Error(t, e)
	elevated, e := f.s.CreateInvitation(ctx, f.org, Capabilities, "", f.owner, "req")
	require.NoError(t, e)
	a, e = f.s.AcceptInvitation(ctx, p, elevated.Token, "req")
	require.NoError(t, e)
	require.Equal(t, []string{"analytics:read"}, a.Capabilities, "repeat joining does not elevate")
	_, e = f.s.AuthorizeOrganization(ctx, p, f.org, "members:read")
	require.ErrorIs(t, e, ErrForbidden)
	a, e = f.s.UpdateAccess(ctx, f.org, p.UserID, a.Capabilities, "revoked", a.Revision, f.owner, "req")
	require.NoError(t, e)
	_, e = f.s.AcceptInvitation(ctx, p, elevated.Token, "req")
	require.ErrorIs(t, e, ErrForbidden)
	_, e = f.s.UpdateAccess(ctx, f.org, p.UserID, Capabilities, "active", a.Revision-1, f.owner, "req")
	require.Error(t, e)
	_, e = f.s.UpdateAccess(ctx, f.org, p.UserID, Capabilities, "active", a.Revision, f.owner, "req")
	require.NoError(t, e)
	var ownerTwo, orgTwo int64
	require.NoError(t, f.db.QueryRow(`INSERT INTO users(email,password_hash) VALUES('owner-two@local.test','test-hash') RETURNING id`).Scan(&ownerTwo))
	require.NoError(t, f.db.QueryRow(`INSERT INTO desktop_organizations(public_id,code,name,gateway_user_id,group_id) SELECT 'org_second','second','另一家企业',$1,group_id FROM desktop_organizations WHERE public_id=$2 RETURNING id`, ownerTwo, f.org).Scan(&orgTwo))
	second, e := f.s.CreateInvitation(ctx, "org_second", []string{"members:read"}, "second scope", ownerTwo, "req")
	require.NoError(t, e)
	secondAccess, e := f.s.AcceptInvitation(ctx, p, second.Token, "req")
	require.NoError(t, e)
	require.Equal(t, []string{"members:read"}, secondAccess.Capabilities)
	_, e = f.s.AuthorizeOrganization(ctx, p, "org_second", "analytics:read")
	require.ErrorIs(t, e, ErrForbidden)
	organizations, e := f.s.Organizations(ctx, p, 1, 20)
	require.NoError(t, e)
	require.EqualValues(t, 2, organizations.Total)
	revoked, e := f.s.CreateInvitation(ctx, f.org, []string{"analytics:read"}, "revoked", f.owner, "req")
	require.NoError(t, e)
	_, e = f.db.Exec(`UPDATE bi_invitations SET status='revoked' WHERE id=$1`, revoked.ID)
	require.NoError(t, e)
	_, e = f.s.PreviewInvitation(ctx, p, revoked.Token)
	require.Error(t, e)
	expired, e := f.s.CreateInvitation(ctx, f.org, []string{"analytics:read"}, "", f.owner, "req")
	require.NoError(t, e)
	_, e = f.db.Exec(`UPDATE bi_invitations SET expires_at=NOW()-INTERVAL '1 second' WHERE id=$1`, expired.ID)
	require.NoError(t, e)
	_, e = f.s.PreviewInvitation(ctx, p, expired.Token)
	require.Error(t, e)
	race, e := f.s.CreateInvitation(ctx, f.org, []string{"members:read"}, "", f.owner, "req")
	require.NoError(t, e)
	three := f.login(t, "three")
	four := f.login(t, "four")
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, id := range []string{three.User.ID, four.User.ID} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, err := f.s.AcceptInvitation(ctx, Principal{UserID: id}, race.Token, "req")
			results <- err
		}(id)
	}
	wg.Wait()
	close(results)
	wins := 0
	for err := range results {
		if err == nil {
			wins++
		}
	}
	require.Equal(t, 1, wins)
	rotated, e := f.s.Refresh(ctx, one.RefreshToken, "req")
	require.NoError(t, e)
	_, e = f.s.Refresh(ctx, one.RefreshToken, "req")
	require.ErrorIs(t, e, ErrUnauthenticated)
	_, e = f.s.Authorize(ctx, rotated.AccessToken)
	require.ErrorIs(t, e, ErrUnauthenticated, "refresh replay revocation must commit")
	require.NoError(t, f.s.SetUserStatus(ctx, two.User.ID, "disabled", f.owner, "req"))
	_, e = f.s.Authorize(ctx, two.AccessToken)
	require.ErrorIs(t, e, ErrUnauthenticated)
}
func nativeRouter(f *biFixture) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	root := r.Group("/api/bi/v1", Boundary)
	root.POST("/auth/wechat", f.h.Login)
	root.POST("/auth/refresh", f.h.Refresh)
	root.POST("/auth/logout", f.h.Logout)
	mobile := root.Group("", f.h.MobileAuth)
	mobile.GET("/me", f.h.Me)
	mobile.GET("/organizations", f.h.Organizations)
	mobile.POST("/invitations/preview", f.h.PreviewInvitation)
	mobile.POST("/invitations/accept", f.h.AcceptInvitation)
	for path, operation := range map[string]string{"": "organization", "/usage": "usage", "/members": "members", "/conversations": "conversations", "/daily-reports/:date/summary": "daily_summary", "/daily-reports/:date/text": "daily_text", "/period-reports": "periods", "/period-reports/:report_id": "period", "/period-reports/:report_id/sources": "sources"} {
		mobile.GET("/organizations/:organization_id/desktop"+path, f.h.NativeRead(operation))
	}
	return r
}
func TestNativeHTTPAndActualTypeScriptClient(t *testing.T) {
	f := independentFixture(t)
	ctx := context.Background()
	inv, e := f.s.CreateInvitation(ctx, f.org, Capabilities, "TS integration", f.owner, "req")
	require.NoError(t, e)
	_, e = f.db.Exec(`INSERT INTO desktop_daily_report_tasks(organization_id,report_date,status,summary) VALUES((SELECT id FROM desktop_organizations WHERE public_id=$1),'2026-10-01','completed','{"status":"completed","content":"企业日总结正文","generated_at":"2026-10-02T00:00:00Z"}')`, f.org)
	require.NoError(t, e)
	server := httptest.NewServer(nativeRouter(f))
	defer server.Close()
	node := exec.Command("node", "--import", "tsx", "tests/native-http.ts")
	node.Dir = os.Getenv("BI_MOBILE_PROJECT")
	if node.Dir == "" {
		node.Dir, e = filepath.Abs("../../../../../Ts/devku_mobile/miniprogram")
		require.NoError(t, e)
	}
	if _, err := os.Stat(filepath.Join(node.Dir, "tests/native-http.ts")); err != nil {
		t.Skip("set BI_MOBILE_PROJECT to the mobile checkout for actual TypeScript integration")
	}
	node.Env = append(os.Environ(), "BI_TEST_BASE_URL="+server.URL+"/api/bi/v1", "BI_TEST_INVITE="+inv.Token)
	out, e := node.CombinedOutput()
	require.NoError(t, e, string(out))
	require.Contains(t, string(out), "native HTTP flow verified")

}
func TestOperationsMetadataNeverReturnsBusinessBodies(t *testing.T) {
	f := independentFixture(t)
	_, e := f.db.Exec(`INSERT INTO desktop_daily_report_tasks(organization_id,report_date,status,summary,payload) VALUES((SELECT id FROM desktop_organizations WHERE public_id=$1),'2026-10-01','completed','{"content":"FORBIDDEN_SUMMARY","generated_at":"2026-10-02T00:00:00Z"}','{"snapshots":"FORBIDDEN_INPUT"}')`, f.org)
	require.NoError(t, e)
	r := gin.New()
	r.GET("/tasks", Boundary, f.h.OperationsTasks)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/tasks", nil))
	require.Equal(t, 200, w.Code, w.Body.String())
	require.NotContains(t, w.Body.String(), "FORBIDDEN")
	var data map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &data))
	require.Contains(t, w.Body.String(), "available_sources")
	r.GET("/scoped/:organization_id/users", Boundary, func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: f.owner})
	}, f.h.ManagerScope, f.h.ListAccess)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/scoped/another/users", nil))
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"total":0`)
}
