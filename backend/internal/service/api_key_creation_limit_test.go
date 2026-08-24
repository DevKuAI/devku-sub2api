//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type creationLimitAPIKeyRepoStub struct {
	quotaBaseAPIKeyRepoStub
	createCalls int
	count       int64
}

func (s *creationLimitAPIKeyRepoStub) Create(_ context.Context, key *APIKey) error {
	s.createCalls++
	key.ID = int64(s.createCalls)
	return nil
}

func (s *creationLimitAPIKeyRepoStub) CountByUserID(context.Context, int64) (int64, error) {
	return s.count, nil
}

type atomicCreationLimitAPIKeyRepoStub struct {
	creationLimitAPIKeyRepoStub
	atomicCalls int
	atomicErr   error
}

func (s *atomicCreationLimitAPIKeyRepoStub) CreateWithinUserLimit(_ context.Context, key *APIKey) error {
	s.atomicCalls++
	if s.atomicErr != nil {
		return s.atomicErr
	}
	key.ID = int64(s.atomicCalls)
	return nil
}

func TestAPIKeyServiceCreateUnlimitedUsesLegacyCreate(t *testing.T) {
	repo := &creationLimitAPIKeyRepoStub{}
	svc := &APIKeyService{
		apiKeyRepo: repo,
		userRepo:   &mockUserRepo{getByIDUser: &User{ID: 7}},
		cfg:        &config.Config{},
	}

	key, err := svc.Create(context.Background(), 7, CreateAPIKeyRequest{Name: "default"})

	require.NoError(t, err)
	require.NotNil(t, key)
	require.Equal(t, 1, repo.createCalls)
}

func TestAPIKeyServiceCreateLimitedUserFailsClosedWithoutAtomicRepository(t *testing.T) {
	repo := &creationLimitAPIKeyRepoStub{}
	svc := &APIKeyService{
		apiKeyRepo: repo,
		userRepo:   &mockUserRepo{getByIDUser: &User{ID: 7, APIKeyLimit: 2}},
		cfg:        &config.Config{},
	}

	_, err := svc.Create(context.Background(), 7, CreateAPIKeyRequest{Name: "limited"})

	require.ErrorIs(t, err, ErrAPIKeyLimitEnforcementUnavailable)
	require.Zero(t, repo.createCalls)
}

func TestAPIKeyServiceCreateUsesAtomicLimitRepository(t *testing.T) {
	repo := &atomicCreationLimitAPIKeyRepoStub{}
	svc := &APIKeyService{
		apiKeyRepo: repo,
		userRepo:   &mockUserRepo{getByIDUser: &User{ID: 7, APIKeyLimit: 2}},
		cfg:        &config.Config{},
	}

	key, err := svc.Create(context.Background(), 7, CreateAPIKeyRequest{Name: "limited"})

	require.NoError(t, err)
	require.NotNil(t, key)
	require.Equal(t, 1, repo.atomicCalls)
	require.Zero(t, repo.createCalls)
}

func TestAPIKeyServiceCreateReturnsLimitReachedFromAtomicRepository(t *testing.T) {
	repo := &atomicCreationLimitAPIKeyRepoStub{atomicErr: ErrAPIKeyLimitReached}
	svc := &APIKeyService{
		apiKeyRepo: repo,
		userRepo:   &mockUserRepo{getByIDUser: &User{ID: 7, APIKeyLimit: 1}},
		cfg:        &config.Config{},
	}

	_, err := svc.Create(context.Background(), 7, CreateAPIKeyRequest{Name: "limited"})

	require.ErrorIs(t, err, ErrAPIKeyLimitReached)
	require.Equal(t, 1, repo.atomicCalls)
}

func TestAPIKeyServiceGetCreationQuota(t *testing.T) {
	tests := []struct {
		name string
		user *User
		used int64
		want APIKeyCreationQuota
	}{
		{
			name: "unlimited",
			user: &User{ID: 7, APIKeyLimit: 0},
			used: 12,
			want: APIKeyCreationQuota{Limit: 0, Used: 12, Remaining: 0, Unlimited: true},
		},
		{
			name: "remaining slots",
			user: &User{ID: 7, APIKeyLimit: 5},
			used: 3,
			want: APIKeyCreationQuota{Limit: 5, Used: 3, Remaining: 2, Unlimited: false},
		},
		{
			name: "limit lowered below current count",
			user: &User{ID: 7, APIKeyLimit: 2},
			used: 3,
			want: APIKeyCreationQuota{Limit: 2, Used: 3, Remaining: 0, Unlimited: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &creationLimitAPIKeyRepoStub{count: tt.used}
			svc := &APIKeyService{
				apiKeyRepo: repo,
				userRepo:   &mockUserRepo{getByIDUser: tt.user},
			}

			got, err := svc.GetCreationQuota(context.Background(), tt.user.ID)

			require.NoError(t, err)
			require.Equal(t, tt.want, *got)
		})
	}
}

func TestAdminServiceRejectsNegativeAPIKeyLimit(t *testing.T) {
	svc := &adminServiceImpl{userRepo: &userRepoStub{nextID: 1}}

	_, err := svc.CreateUser(context.Background(), &CreateUserInput{
		Email:       "negative-limit@test.com",
		Password:    "password",
		APIKeyLimit: -1,
	})

	require.ErrorIs(t, err, ErrAPIKeyLimitInvalid)
}

func TestAdminServiceUpdatesAPIKeyLimit(t *testing.T) {
	repo := &userRepoStub{user: &User{ID: 7, Email: "limited@test.com", APIKeyLimit: 2}}
	svc := &adminServiceImpl{userRepo: repo}
	limit := 5

	updated, err := svc.UpdateUser(context.Background(), 7, &UpdateUserInput{APIKeyLimit: &limit})

	require.NoError(t, err)
	require.Equal(t, 5, updated.APIKeyLimit)
	require.Len(t, repo.updated, 1)
	require.Equal(t, 5, repo.updated[0].APIKeyLimit)
}

func TestAdminServiceRejectsNegativeAPIKeyLimitUpdate(t *testing.T) {
	repo := &userRepoStub{user: &User{ID: 7, Email: "limited@test.com", APIKeyLimit: 2}}
	svc := &adminServiceImpl{userRepo: repo}
	limit := -1

	_, err := svc.UpdateUser(context.Background(), 7, &UpdateUserInput{APIKeyLimit: &limit})

	require.ErrorIs(t, err, ErrAPIKeyLimitInvalid)
	require.Empty(t, repo.updated)
}
