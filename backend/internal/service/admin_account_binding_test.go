package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type accountBindingRepoStub struct {
	AccountRepository
	listUserID    int64
	listResult    []Account
	setAccountID  int64
	setUserID     *int64
	setExpectedID *int64
	setResult     *Account
}

func (s *accountBindingRepoStub) ListByBoundUserID(_ context.Context, userID int64) ([]Account, error) {
	s.listUserID = userID
	return s.listResult, nil
}

func (s *accountBindingRepoStub) SetBoundUser(_ context.Context, accountID int64, userID, expectedUserID *int64) (*Account, error) {
	s.setAccountID = accountID
	s.setUserID = copyAccountBindingUserID(userID)
	s.setExpectedID = copyAccountBindingUserID(expectedUserID)
	return s.setResult, nil
}

func (s *accountBindingRepoStub) PopulateBoundUsers(_ context.Context, _ []Account) error {
	return nil
}

type accountBindingUserRepoStub struct {
	UserRepository
	user     *User
	getByID  int64
	getCalls int
}

func (s *accountBindingUserRepoStub) GetByID(_ context.Context, id int64) (*User, error) {
	s.getByID = id
	s.getCalls++
	return s.user, nil
}

func TestAdminServiceAccountBinding(t *testing.T) {
	t.Run("lists accounts for one user", func(t *testing.T) {
		repo := &accountBindingRepoStub{listResult: []Account{{ID: 1}, {ID: 2}}}
		svc := &adminServiceImpl{accountRepo: repo, accountBindingRepo: repo}

		accounts, err := svc.ListAccountsByBoundUserID(context.Background(), 42)

		require.NoError(t, err)
		require.Len(t, accounts, 2)
		require.Equal(t, int64(42), repo.listUserID)
	})

	t.Run("binds an active user", func(t *testing.T) {
		boundUserID := int64(42)
		repo := &accountBindingRepoStub{setResult: &Account{ID: 7, BoundUserID: &boundUserID}}
		users := &accountBindingUserRepoStub{user: &User{ID: boundUserID, Status: StatusActive}}
		svc := &adminServiceImpl{accountRepo: repo, accountBindingRepo: repo, userRepo: users}

		account, err := svc.BindAccountUser(context.Background(), 7, &boundUserID, nil)

		require.NoError(t, err)
		require.Equal(t, int64(7), account.ID)
		require.Equal(t, boundUserID, users.getByID)
		require.Equal(t, boundUserID, *repo.setUserID)
	})

	t.Run("removes a user without looking one up", func(t *testing.T) {
		repo := &accountBindingRepoStub{setResult: &Account{ID: 7}}
		users := &accountBindingUserRepoStub{}
		svc := &adminServiceImpl{accountRepo: repo, accountBindingRepo: repo, userRepo: users}

		expectedUserID := int64(42)
		account, err := svc.BindAccountUser(context.Background(), 7, nil, &expectedUserID)

		require.NoError(t, err)
		require.Equal(t, int64(7), account.ID)
		require.Zero(t, users.getCalls)
		require.Nil(t, repo.setUserID)
		require.Equal(t, expectedUserID, *repo.setExpectedID)
	})

	t.Run("rejects an inactive user", func(t *testing.T) {
		boundUserID := int64(42)
		repo := &accountBindingRepoStub{}
		users := &accountBindingUserRepoStub{user: &User{ID: boundUserID, Status: StatusDisabled}}
		svc := &adminServiceImpl{accountRepo: repo, accountBindingRepo: repo, userRepo: users}

		_, err := svc.BindAccountUser(context.Background(), 7, &boundUserID, nil)

		require.Error(t, err)
		require.Zero(t, repo.setAccountID)
	})
}

func TestAccountBoundUserIsNotSerialized(t *testing.T) {
	account := Account{
		ID:          7,
		BoundUserID: func() *int64 { value := int64(42); return &value }(),
		BoundUser:   &AccountBoundUser{ID: 42, Username: "Alice", Email: "alice@example.com"},
	}

	payload, err := json.Marshal(account)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "Alice")
	require.NotContains(t, string(payload), "alice@example.com")
}

func copyAccountBindingUserID(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
