//go:build integration

package bi

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

var integrationDB *sql.DB

func TestMain(m *testing.M) {
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:18.1-alpine3.23", postgres.WithDatabase("bi_test"),
		postgres.WithUsername("postgres"), postgres.WithPassword("postgres"), postgres.BasicWaitStrategies())
	if err != nil {
		log.Fatal(err)
	}
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err == nil {
		integrationDB, err = sql.Open("postgres", dsn)
	}
	if err == nil {
		err = repository.ApplyMigrations(ctx, integrationDB)
	}
	if err != nil {
		_ = container.Terminate(ctx)
		log.Fatal(err)
	}
	code := m.Run()
	_ = integrationDB.Close()
	_ = container.Terminate(ctx)
	os.Exit(code)
}

type testUsers map[int64]*service.User

func (u testUsers) GetByID(_ context.Context, id int64) (*service.User, error) {
	user := u[id]
	if user == nil {
		return nil, service.ErrUserNotFound
	}
	copy := *user
	return &copy, nil
}

type testWeChat struct{ prefix string }

func (w testWeChat) Exchange(_ context.Context, code string) (string, error) {
	if strings.Contains(code, "wrong-subject") {
		return "other-" + w.prefix, nil
	}
	return w.prefix, nil
}

func authFixture(t *testing.T) (*Service, testUsers, []int64) {
	t.Helper()
	users := testUsers{}
	ids := []int64{}
	for i := 0; i < 2; i++ {
		var id int64
		email := randomToken("user_") + "@example.com"
		require.NoError(t, integrationDB.QueryRow(`INSERT INTO users(email,password_hash) VALUES($1,'test-hash') RETURNING id`, email).Scan(&id))
		users[id] = &service.User{ID: id, Email: email, PasswordHash: "test-hash", Username: "Test manager", Status: service.StatusActive}
		ids = append(ids, id)
	}
	cfg := config.BIConfig{AppID: randomToken("wx_"),
		JWTSecret:      base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", 32))),
		IdentitySecret: base64.StdEncoding.EncodeToString([]byte(strings.Repeat("y", 32)))}
	return NewService(integrationDB, cfg, users, testWeChat{prefix: cfg.AppID}), users, ids
}

func loginChallenge(t *testing.T, s *Service) BindingChallenge {
	t.Helper()
	result, err := s.Login(context.Background(), randomToken("code_"))
	require.NoError(t, err)
	challenge, ok := result.(BindingChallenge)
	require.True(t, ok)
	return challenge
}

func boundSession(t *testing.T, s *Service, userID int64) *Session {
	t.Helper()
	ctx := context.Background()
	challenge := loginChallenge(t, s)
	require.NoError(t, s.ApproveBinding(ctx, userID, challenge.UserCode, "approve"))
	session, err := s.ExchangeBinding(ctx, challenge.BindingTicket, randomToken("code_"))
	require.NoError(t, err)
	return session
}

func TestBindingApprovalRaceAndOneTimeExchange(t *testing.T) {
	s, _, ids := authFixture(t)
	ctx := context.Background()
	challenge := loginChallenge(t, s)
	_, err := s.ExchangeBinding(ctx, challenge.BindingTicket, randomToken("code_"))
	require.ErrorIs(t, err, ErrPendingBinding)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, id := range ids {
		wg.Add(1)
		go func(id int64) { defer wg.Done(); results <- s.ApproveBinding(ctx, id, challenge.UserCode, "race") }(id)
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrBindingConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, conflicts)
	_, err = s.ExchangeBinding(ctx, challenge.BindingTicket, randomToken("wrong-subject_"))
	require.ErrorIs(t, err, ErrBindingConflict)
	session, err := s.ExchangeBinding(ctx, challenge.BindingTicket, randomToken("code_"))
	require.NoError(t, err)
	require.Empty(t, session.User.Capabilities)
	p, err := s.Authorize(ctx, session.AccessToken)
	require.NoError(t, err)
	page, err := s.ListOrganizations(ctx, p, 20, "")
	require.NoError(t, err)
	require.Empty(t, page.Items)
	_, err = s.ExchangeBinding(ctx, challenge.BindingTicket, randomToken("code_"))
	require.ErrorIs(t, err, ErrBindingExpired)
}

func TestWeChatCodeReplayAndUnlinkInvalidatesOutstandingChallenges(t *testing.T) {
	s, _, ids := authFixture(t)
	ctx := context.Background()
	code := randomToken("code_")
	_, err := s.Login(ctx, code)
	require.NoError(t, err)
	_, err = s.Login(ctx, code)
	var replay *Error
	require.ErrorAs(t, err, &replay)
	require.Equal(t, "INVALID_ARGUMENT", replay.Code)
	require.Equal(t, "code", replay.Details[0].Field)
	pending := loginChallenge(t, s)
	session := boundSession(t, s, ids[0])
	p, err := s.Authorize(ctx, session.AccessToken)
	require.NoError(t, err)
	require.ErrorIs(t, s.RevokeBinding(ctx, ids[1], p.BindingID, "wrong-user"), ErrNotFound)
	require.NoError(t, s.RevokeBinding(ctx, ids[0], p.BindingID, "unlink"))
	_, err = s.Authorize(ctx, session.AccessToken)
	require.ErrorIs(t, err, ErrUnauthenticated)
	_, err = s.Refresh(ctx, session.RefreshToken, "refresh")
	require.ErrorIs(t, err, ErrUnauthenticated)
	require.ErrorIs(t, s.ApproveBinding(ctx, ids[0], pending.UserCode, "old-code"), ErrBindingExpired)
	_, err = s.ExchangeBinding(ctx, pending.BindingTicket, randomToken("code_"))
	require.ErrorIs(t, err, ErrBindingExpired)
	// A new explicit login/approval can bind again without reviving any old session.
	newSession := boundSession(t, s, ids[0])
	_, err = s.Authorize(ctx, newSession.AccessToken)
	require.NoError(t, err)
}

func TestRefreshRaceRevokesFamilyAndLogoutIsImmediate(t *testing.T) {
	s, _, ids := authFixture(t)
	ctx := context.Background()
	session := boundSession(t, s, ids[0])
	type result struct {
		session *Session
		err     error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			session, err := s.Refresh(ctx, session.RefreshToken, "refresh-race")
			results <- result{session, err}
		}()
	}
	wg.Wait()
	close(results)
	var rotated *Session
	success, denied := 0, 0
	for result := range results {
		if result.err == nil {
			success++
			rotated = result.session
		} else {
			require.ErrorIs(t, result.err, ErrUnauthenticated)
			denied++
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, denied)
	_, err := s.Authorize(ctx, rotated.AccessToken)
	require.ErrorIs(t, err, ErrUnauthenticated)
	fresh, err := s.Login(ctx, randomToken("code_"))
	require.NoError(t, err)
	current := fresh.(*Session)
	require.NoError(t, s.Logout(ctx, current.RefreshToken, "logout"))
	require.NoError(t, s.Logout(ctx, current.RefreshToken, "logout-again"))
	_, err = s.Authorize(ctx, current.AccessToken)
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestPasswordChangeAndAccountDisableInvalidateMobileSession(t *testing.T) {
	s, users, ids := authFixture(t)
	session := boundSession(t, s, ids[0])
	users[ids[0]].PasswordHash = "new-password-hash"
	_, err := s.Authorize(context.Background(), session.AccessToken)
	require.ErrorIs(t, err, ErrUnauthenticated)
	_, err = s.Refresh(context.Background(), session.RefreshToken, "refresh")
	require.ErrorIs(t, err, ErrUnauthenticated)
	users[ids[0]].PasswordHash = "test-hash"
	users[ids[0]].Status = "disabled"
	_, err = s.Authorize(context.Background(), session.AccessToken)
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestBindingListSnapshotsExpireAndRejectOtherUsers(t *testing.T) {
	s, _, ids := authFixture(t)
	session := boundSession(t, s, ids[0])
	for i := 0; i < 3; i++ {
		_, err := integrationDB.Exec(`INSERT INTO bi_wechat_bindings(id,appid,openid_hash,manager_id) VALUES($1,$2,$3,$4)`,
			randomToken("bib_"), s.config.AppID, tokenHash(fmt.Sprintf("additional-%d", i)), session.User.ID)
		require.NoError(t, err)
	}
	ctx := context.Background()
	page, err := s.ListBindings(ctx, ids[0], 2, "")
	require.NoError(t, err)
	require.True(t, page.HasMore)
	_, err = s.ListBindings(ctx, ids[1], 2, *page.NextCursor)
	require.Error(t, err)
	next, err := s.ListBindings(ctx, ids[0], 2, *page.NextCursor)
	require.NoError(t, err)
	require.False(t, next.HasMore)
	require.Equal(t, page.SnapshotID, next.SnapshotID)
	require.NotEqual(t, page.Items[0].ID, next.Items[0].ID)
	_, err = integrationDB.Exec(`UPDATE bi_list_snapshots SET expires_at=$2 WHERE id=$1`, page.SnapshotID, time.Now().Add(-time.Minute))
	require.NoError(t, err)
	_, err = s.ListBindings(ctx, ids[0], 2, *page.NextCursor)
	require.ErrorContains(t, err, "CONTEXT_EXPIRED")
}

func TestGrantsAreExplicitScopedAndRevokeImmediately(t *testing.T) {
	s, _, ids := authFixture(t)
	ctx := context.Background()
	session := boundSession(t, s, ids[0])
	p, err := s.Authorize(ctx, session.AccessToken)
	require.NoError(t, err)
	var groupID int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO groups(name) VALUES($1) RETURNING id`, randomToken("group_")).Scan(&groupID))
	organizations := []string{}
	for i, userID := range ids {
		publicID := fmt.Sprintf("org_bi_%d", userID)
		_, err := integrationDB.Exec(`INSERT INTO desktop_organizations(public_id,code,name,gateway_user_id,group_id) VALUES($1,$2,'BI test',$3,$4)`,
			publicID, fmt.Sprintf("bi%d", userID), userID, groupID)
		require.NoError(t, err)
		organizations = append(organizations, publicID)
		zero := int64(0)
		capabilities := []string{"analytics:read"}
		if i == 0 {
			capabilities = append(capabilities, "reports:share")
		}
		grant, err := s.SaveGrant(ctx, publicID, GrantInput{UserID: ids[0], Role: "viewer", AllTeams: true, TeamIDs: []string{}, Capabilities: capabilities, ExpectedRevision: &zero}, ids[1], "grant")
		require.NoError(t, err)
		require.EqualValues(t, 1, grant.Revision)
	}
	_, err = s.AuthorizeOrganization(ctx, p, organizations[0], "reports:share")
	require.NoError(t, err)
	_, err = s.AuthorizeOrganization(ctx, p, organizations[1], "reports:share")
	require.ErrorIs(t, err, ErrForbidden)
	page, err := s.ListOrganizations(ctx, p, 1, "")
	require.NoError(t, err)
	require.True(t, page.HasMore)
	err = s.RevokeGrant(ctx, organizations[0], p.ManagerID, 99, ids[1], "wrong-revision")
	require.ErrorContains(t, err, "CONFLICT")
	require.NoError(t, s.RevokeGrant(ctx, organizations[0], p.ManagerID, 1, ids[1], "revoke"))
	_, err = s.AuthorizeOrganization(ctx, p, organizations[0], "analytics:read")
	require.ErrorIs(t, err, ErrNotFound)
	_, err = s.ListOrganizations(ctx, p, 1, *page.NextCursor)
	require.ErrorContains(t, err, "CONTEXT_REVOKED")
	_, err = s.AuthorizeOrganization(ctx, p, organizations[1], "analytics:read")
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE desktop_organizations SET status='disabled' WHERE public_id=$1`, organizations[1])
	require.NoError(t, err)
	_, err = s.AuthorizeOrganization(ctx, p, organizations[1], "analytics:read")
	require.ErrorIs(t, err, ErrNotFound)
}
