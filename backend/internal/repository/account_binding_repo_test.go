package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newAccountBindingRepo(t *testing.T) (*accountRepository, *dbent.Client, *sql.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	driver := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(driver)))
	t.Cleanup(func() { _ = client.Close() })
	return newAccountRepositoryWithSQL(client, db, nil), client, db
}

func createBindingUser(t *testing.T, ctx context.Context, client *dbent.Client, email, username string) *dbent.User {
	t.Helper()
	user, err := client.User.Create().
		SetEmail(email).
		SetPasswordHash("test-password-hash").
		SetUsername(username).
		SetRole(service.RoleUser).
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)
	return user
}

func createBindingAccount(t *testing.T, ctx context.Context, client *dbent.Client, name string) *dbent.Account {
	t.Helper()
	account, err := client.Account.Create().
		SetName(name).
		SetPlatform(service.PlatformOpenAI).
		SetType(service.AccountTypeAPIKey).
		SetStatus(service.StatusActive).
		SetCredentials(map[string]any{"api_key": "sk-private"}).
		Save(ctx)
	require.NoError(t, err)
	return account
}

func TestAccountRepositoryBindingSupportsManyAccountsAndRemoval(t *testing.T) {
	repo, client, _ := newAccountBindingRepo(t)
	ctx := context.Background()
	alice := createBindingUser(t, ctx, client, "alice@example.com", "Alice")
	bob := createBindingUser(t, ctx, client, "bob@example.com", "Bob")
	first := createBindingAccount(t, ctx, client, "First")
	second := createBindingAccount(t, ctx, client, "Second")

	for _, accountID := range []int64{first.ID, second.ID} {
		bound, err := repo.SetBoundUser(ctx, accountID, &alice.ID, nil)
		require.NoError(t, err)
		require.Equal(t, alice.ID, *bound.BoundUserID)
		require.Equal(t, "Alice", bound.BoundUser.Username)
	}

	aliceAccounts, err := repo.ListByBoundUserID(ctx, alice.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"First", "Second"}, []string{aliceAccounts[0].Name, aliceAccounts[1].Name})

	rebound, err := repo.SetBoundUser(ctx, first.ID, &bob.ID, &alice.ID)
	require.NoError(t, err)
	require.Equal(t, bob.ID, *rebound.BoundUserID)
	require.Equal(t, "Bob", rebound.BoundUser.Username)

	aliceAccounts, err = repo.ListByBoundUserID(ctx, alice.ID)
	require.NoError(t, err)
	require.Len(t, aliceAccounts, 1)
	require.Equal(t, second.ID, aliceAccounts[0].ID)

	removed, err := repo.SetBoundUser(ctx, first.ID, nil, &bob.ID)
	require.NoError(t, err)
	require.Nil(t, removed.BoundUserID)
	require.Nil(t, removed.BoundUser)

	bobAccounts, err := repo.ListByBoundUserID(ctx, bob.ID)
	require.NoError(t, err)
	require.Empty(t, bobAccounts)
}

func TestAccountRepositoryBindingClearsOnHardUserDelete(t *testing.T) {
	repo, client, _ := newAccountBindingRepo(t)
	ctx := context.Background()
	user := createBindingUser(t, ctx, client, "deleted@example.com", "Deleted User")
	account := createBindingAccount(t, ctx, client, "Bound Account")

	_, err := repo.SetBoundUser(ctx, account.ID, &user.ID, nil)
	require.NoError(t, err)
	require.NoError(t, client.User.DeleteOneID(user.ID).Exec(mixins.SkipSoftDelete(ctx)))

	stored, err := client.Account.Get(ctx, account.ID)
	require.NoError(t, err)
	require.Nil(t, stored.BoundUserID)
}

func TestAccountRepositoryBindingRejectsSoftDeletedAccount(t *testing.T) {
	repo, client, _ := newAccountBindingRepo(t)
	ctx := context.Background()
	user := createBindingUser(t, ctx, client, "owner@example.com", "Owner")
	account := createBindingAccount(t, ctx, client, "Deleted Account")
	require.NoError(t, client.Account.DeleteOneID(account.ID).Exec(ctx))

	_, err := repo.SetBoundUser(ctx, account.ID, &user.ID, nil)
	require.ErrorIs(t, err, service.ErrAccountNotFound)

	stored, err := client.Account.Get(mixins.SkipSoftDelete(ctx), account.ID)
	require.NoError(t, err)
	require.Nil(t, stored.BoundUserID)
}

func TestUserRepositorySoftDeleteClearsAccountBindings(t *testing.T) {
	accountRepo, client, db := newAccountBindingRepo(t)
	ctx := context.Background()
	user := createBindingUser(t, ctx, client, "deleted@example.com", "Deleted User")
	account := createBindingAccount(t, ctx, client, "Bound Account")
	_, err := accountRepo.SetBoundUser(ctx, account.ID, &user.ID, nil)
	require.NoError(t, err)

	userRepo := newUserRepositoryWithSQL(client, db)
	require.NoError(t, userRepo.Delete(ctx, user.ID))

	stored, err := client.Account.Get(ctx, account.ID)
	require.NoError(t, err)
	require.Nil(t, stored.BoundUserID)
}

func TestAccountRepositoryBindingRejectsStaleExpectedUser(t *testing.T) {
	repo, client, _ := newAccountBindingRepo(t)
	ctx := context.Background()
	alice := createBindingUser(t, ctx, client, "alice@example.com", "Alice")
	bob := createBindingUser(t, ctx, client, "bob@example.com", "Bob")
	account := createBindingAccount(t, ctx, client, "Bound Account")

	_, err := repo.SetBoundUser(ctx, account.ID, &alice.ID, nil)
	require.NoError(t, err)

	_, err = repo.SetBoundUser(ctx, account.ID, &bob.ID, nil)
	require.ErrorIs(t, err, service.ErrAccountBindingConflict)

	stored, err := client.Account.Get(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, alice.ID, *stored.BoundUserID)
}
