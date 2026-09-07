//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestDesktopMemberLimitMigrationBackfillsExistingCapacity(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	_, err := tx.ExecContext(ctx, `
		CREATE TEMP TABLE users (id BIGINT PRIMARY KEY, api_key_limit INTEGER, deleted_at TIMESTAMPTZ, updated_at TIMESTAMPTZ) ON COMMIT DROP;
		CREATE TEMP TABLE desktop_organizations (id BIGINT PRIMARY KEY, gateway_user_id BIGINT, deleted_at TIMESTAMPTZ) ON COMMIT DROP;
		CREATE TEMP TABLE desktop_members (id BIGINT, organization_id BIGINT, deleted_at TIMESTAMPTZ) ON COMMIT DROP;
		INSERT INTO users (id, api_key_limit) VALUES (1, 0), (2, 25), (3, 0);
		INSERT INTO desktop_organizations (id, gateway_user_id) VALUES (1, 1), (2, 2), (3, 3);
		INSERT INTO desktop_members (id, organization_id) SELECT n, 3 FROM generate_series(1, 12) AS n;
		INSERT INTO desktop_members VALUES (13, 1, NOW());
	`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("236_desktop_organization_member_limit.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	for id, expected := range map[int]int{1: 10, 2: 25, 3: 12} {
		var memberLimit, apiKeyLimit int
		err := tx.QueryRowContext(ctx, `SELECT o.member_limit, u.api_key_limit FROM desktop_organizations o JOIN users u ON u.id = o.gateway_user_id WHERE o.id = $1`, id).Scan(&memberLimit, &apiKeyLimit)
		require.NoError(t, err)
		require.Equal(t, expected, memberLimit)
		require.Equal(t, expected, apiKeyLimit)
	}
	var defaultLimit int
	err = tx.QueryRowContext(ctx, "INSERT INTO desktop_organizations (id, gateway_user_id) VALUES (4, 1) RETURNING member_limit").Scan(&defaultLimit)
	require.NoError(t, err)
	require.Equal(t, 10, defaultLimit)
	_, err = tx.ExecContext(ctx, "UPDATE desktop_organizations SET member_limit = 0 WHERE id = 4")
	require.Error(t, err)
}

func TestDesktopRepositoryMemberLimitDefaultsAndSyncsOnReassignment(t *testing.T) {
	ctx := context.Background()
	fixture := newDesktopRepositoryFixture(t, "quota-default", 25)
	carrier := mustCreateUser(t, integrationEntClient, &service.User{
		Email: fmt.Sprintf("desktop-quota-carrier-%d@example.com", fixture.suffix),
	})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM desktop_organizations WHERE gateway_user_id = $1", carrier.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM users WHERE id = $1", carrier.ID)
	})
	organization, err := fixture.repo.CreateOrganization(ctx, service.DesktopCreateOrganizationInput{
		PublicID: fmt.Sprintf("org_default_%d", fixture.suffix), Code: fmt.Sprintf("d%x", fixture.suffix%100000),
		Name: "Default quota", GatewayUserID: carrier.ID, GroupID: fixture.group.ID,
	})
	require.NoError(t, err)
	require.Equal(t, 10, organization.MemberLimit)
	stored, err := integrationEntClient.User.Get(ctx, carrier.ID)
	require.NoError(t, err)
	require.Equal(t, 10, stored.APIKeyLimit)

	// Release the initial assignment so the same user can carry a different enterprise.
	_, err = integrationDB.ExecContext(ctx, "UPDATE desktop_organizations SET deleted_at = NOW() WHERE id = $1", organization.ID)
	require.NoError(t, err)
	updated, _, err := fixture.repo.UpdateOrganization(ctx, fixture.organization.PublicID, service.DesktopUpdateOrganizationInput{GatewayUserID: &carrier.ID})
	require.NoError(t, err)
	require.Equal(t, 25, updated.MemberLimit)
	stored, err = integrationEntClient.User.Get(ctx, carrier.ID)
	require.NoError(t, err)
	require.Equal(t, 25, stored.APIKeyLimit)
}

func TestDesktopRepositoryMemberLimitUpdateIsAtomicAndAdminOnly(t *testing.T) {
	ctx := context.Background()
	fixture := newDesktopRepositoryFixture(t, "quota-update", 10)
	fixture.createMember(t, "quota-first", 1)
	fixture.createMember(t, "quota-second", 2)
	limit := 15
	_, _, err := fixture.repo.ScopedToGatewayUser(fixture.user.ID).UpdateOrganization(ctx, fixture.organization.PublicID, service.DesktopUpdateOrganizationInput{MemberLimit: &limit})
	require.ErrorIs(t, err, service.ErrDesktopValidation)

	updated, _, err := fixture.repo.UpdateOrganization(ctx, fixture.organization.PublicID, service.DesktopUpdateOrganizationInput{MemberLimit: &limit})
	require.NoError(t, err)
	require.Equal(t, 15, updated.MemberLimit)
	stored, err := integrationEntClient.User.Get(ctx, fixture.user.ID)
	require.NoError(t, err)
	require.Equal(t, 15, stored.APIKeyLimit)

	limit = 1
	_, _, err = fixture.repo.UpdateOrganization(ctx, fixture.organization.PublicID, service.DesktopUpdateOrganizationInput{MemberLimit: &limit})
	require.ErrorIs(t, err, service.ErrDesktopMemberLimitTooLow)
	updated, err = fixture.repo.GetOrganization(ctx, fixture.organization.PublicID)
	require.NoError(t, err)
	require.Equal(t, 15, updated.MemberLimit)
	stored, err = integrationEntClient.User.Get(ctx, fixture.user.ID)
	require.NoError(t, err)
	require.Equal(t, 15, stored.APIKeyLimit)

	// Force the final organization write to fail after the carrier update.
	limit, emptyName := 20, ""
	_, _, err = fixture.repo.UpdateOrganization(ctx, fixture.organization.PublicID, service.DesktopUpdateOrganizationInput{MemberLimit: &limit, Name: &emptyName})
	require.Error(t, err)
	stored, err = integrationEntClient.User.Get(ctx, fixture.user.ID)
	require.NoError(t, err)
	require.Equal(t, 15, stored.APIKeyLimit)
}

func TestDesktopRepositoryMemberLimitCountsDisabledAndReleasesDeletedMembers(t *testing.T) {
	ctx := context.Background()
	fixture := newDesktopRepositoryFixture(t, "quota-slots", 1)
	member := fixture.createMember(t, "quota-disabled", 1)
	status := service.DesktopStatusDisabled
	_, _, err := fixture.repo.UpdateMember(ctx, fixture.organization.PublicID, member.PublicID, service.DesktopUpdateMemberInput{Status: &status})
	require.NoError(t, err)

	input := service.DesktopCreateMemberInput{
		Member: &service.DesktopMember{PublicID: fmt.Sprintf("mem_slot_%d", fixture.suffix), Name: "New member", NameNormalized: "New member", Phone: desktopTestPhone(fixture.suffix + 2)},
		APIKey: &service.APIKey{Key: fmt.Sprintf("sk-slot-%d", fixture.suffix), Name: "New member", Status: service.StatusAPIKeyActive},
	}
	_, err = fixture.repo.CreateMember(ctx, fixture.organization.PublicID, input)
	require.ErrorIs(t, err, service.ErrDesktopMemberLimitReached)
	_, err = fixture.repo.DeleteMember(ctx, fixture.organization.PublicID, member.PublicID)
	require.NoError(t, err)
	_, err = fixture.repo.CreateMember(ctx, fixture.organization.PublicID, input)
	require.NoError(t, err)
}

func TestDesktopRepositoryConcurrentMemberCreationHonorsLimit(t *testing.T) {
	ctx := context.Background()
	fixture := newDesktopRepositoryFixture(t, "quota-race", 1)
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			<-start
			_, err := fixture.repo.CreateMember(ctx, fixture.organization.PublicID, service.DesktopCreateMemberInput{
				Member: &service.DesktopMember{PublicID: fmt.Sprintf("mem_race_%d_%d", fixture.suffix, index), Name: "Member", NameNormalized: "Member", Phone: desktopTestPhone(fixture.suffix + int64(index))},
				APIKey: &service.APIKey{Key: fmt.Sprintf("sk-race-%d-%d", fixture.suffix, index), Name: "Member", Status: service.StatusAPIKeyActive},
			})
			results <- err
		}(i)
	}
	close(start)
	workers.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else {
			require.ErrorIs(t, err, service.ErrDesktopMemberLimitReached)
		}
	}
	require.Equal(t, 1, successes)
	organization, err := fixture.repo.GetOrganization(ctx, fixture.organization.PublicID)
	require.NoError(t, err)
	require.Equal(t, 1, organization.MemberCount)
	var keyCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM api_keys WHERE user_id = $1 AND deleted_at IS NULL", fixture.user.ID).Scan(&keyCount))
	require.Equal(t, 1, keyCount)
}
