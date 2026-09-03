//go:build integration

package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestDesktopRepositoryRejectsRestrictedPublicGroupCarrier(t *testing.T) {
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	group := mustCreateGroup(t, integrationEntClient, &service.Group{
		Name: fmt.Sprintf("desktop-restricted-group-%d", suffix), RateMultiplier: 1,
	})
	user := mustCreateUser(t, integrationEntClient, &service.User{
		Email: fmt.Sprintf("desktop-restricted-%d@example.com", suffix), APIKeyLimit: 10,
	})
	_, err := integrationEntClient.User.UpdateOneID(user.ID).SetRestrictPublicGroups(true).Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM desktop_organizations WHERE gateway_user_id = $1", user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM groups WHERE id = $1", group.ID)
	})

	repo := NewDesktopRepository(integrationEntClient, NewAPIKeyRepository(integrationEntClient, integrationDB))
	_, err = repo.CreateOrganization(ctx, service.DesktopCreateOrganizationInput{
		PublicID:      fmt.Sprintf("org_restricted_%d", suffix),
		Code:          fmt.Sprintf("r%x", suffix%100000),
		Name:          "Restricted",
		GatewayUserID: user.ID,
		GroupID:       group.ID,
	})
	require.ErrorIs(t, err, service.ErrGroupNotAllowed)
}

func TestDesktopRepositoryLoadsAssignedExclusiveGroup(t *testing.T) {
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	group := mustCreateGroup(t, integrationEntClient, &service.Group{
		Name: fmt.Sprintf("desktop-exclusive-group-%d", suffix), RateMultiplier: 1, IsExclusive: true,
	})
	user := mustCreateUser(t, integrationEntClient, &service.User{
		Email: fmt.Sprintf("desktop-exclusive-%d@example.com", suffix), APIKeyLimit: 10, AllowedGroups: []int64{group.ID},
	})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM desktop_organizations WHERE gateway_user_id = $1", user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM user_allowed_groups WHERE user_id = $1", user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM groups WHERE id = $1", group.ID)
	})

	repo := NewDesktopRepository(integrationEntClient, NewAPIKeyRepository(integrationEntClient, integrationDB))
	_, err := repo.CreateOrganization(ctx, service.DesktopCreateOrganizationInput{
		PublicID:      fmt.Sprintf("org_exclusive_%d", suffix),
		Code:          fmt.Sprintf("e%x", suffix%100000),
		Name:          "Exclusive",
		GatewayUserID: user.ID,
		GroupID:       group.ID,
	})
	require.NoError(t, err)

	reader, ok := repo.(interface {
		DesktopOrganizationGroupForUser(context.Context, int64) (int64, bool, bool, error)
	})
	require.True(t, ok)
	groupID, isExclusive, assigned, err := reader.DesktopOrganizationGroupForUser(ctx, user.ID)
	require.NoError(t, err)
	require.True(t, assigned)
	require.True(t, isExclusive)
	require.Equal(t, group.ID, groupID)
}

func TestDesktopRepositoryGatewayUserScopeRejectsCrossOrganizationAccess(t *testing.T) {
	ctx := context.Background()
	owner := newDesktopRepositoryFixture(t, "owner-scope", 10)
	other := newDesktopRepositoryFixture(t, "other-scope", 10)
	otherMember := other.createMember(t, "other-scope", 1)
	scoped := owner.repo.ScopedToGatewayUser(owner.user.ID)

	organization, err := scoped.GetOrganizationForGatewayUser(ctx, owner.user.ID)
	require.NoError(t, err)
	require.Equal(t, owner.organization.PublicID, organization.PublicID)

	_, err = scoped.GetOrganization(ctx, other.organization.PublicID)
	require.ErrorIs(t, err, service.ErrDesktopOrganizationNotFound)
	_, _, err = scoped.ListMembers(ctx, other.organization.PublicID, pagination.PaginationParams{Page: 1, PageSize: 20}, service.DesktopMemberListFilters{})
	require.ErrorIs(t, err, service.ErrDesktopOrganizationNotFound)
	_, _, err = scoped.UpdateOrganization(ctx, other.organization.PublicID, service.DesktopUpdateOrganizationInput{})
	require.ErrorIs(t, err, service.ErrDesktopOrganizationNotFound)
	_, err = scoped.UpdateTargetConfig(ctx, other.organization.PublicID, &service.DesktopTargetConfig{
		SchemaVersion: 1,
		Targets: service.DesktopTargets{ChatGPTCodex: &service.DesktopTarget{
			Enabled: true, ProviderID: "provider", DisplayName: "Codex", RequestedModel: "model", WireAPI: service.DesktopWireAPIResponses,
		}},
	})
	require.ErrorIs(t, err, service.ErrDesktopOrganizationNotFound)
	_, err = scoped.CreateMember(ctx, other.organization.PublicID, service.DesktopCreateMemberInput{
		Member: &service.DesktopMember{PublicID: "mem_cross_scope", Name: "Cross", NameNormalized: "Cross", Phone: "+8613800000011"},
		APIKey: &service.APIKey{Key: "sk-cross-scope", Name: "Cross", Status: service.StatusAPIKeyActive},
	})
	require.ErrorIs(t, err, service.ErrDesktopOrganizationNotFound)
	_, _, err = scoped.UpdateMember(ctx, other.organization.PublicID, otherMember.PublicID, service.DesktopUpdateMemberInput{})
	require.ErrorIs(t, err, service.ErrDesktopOrganizationNotFound)
	_, err = scoped.DeleteMember(ctx, other.organization.PublicID, otherMember.PublicID)
	require.ErrorIs(t, err, service.ErrDesktopOrganizationNotFound)
	_, _, err = scoped.RotateMemberAPIKey(ctx, other.organization.PublicID, otherMember.PublicID, &service.APIKey{})
	require.ErrorIs(t, err, service.ErrDesktopOrganizationNotFound)

	stillPresent, err := other.repo.GetMember(ctx, other.organization.PublicID, otherMember.PublicID)
	require.NoError(t, err)
	require.Equal(t, otherMember.PublicID, stillPresent.PublicID)
}

func TestDesktopRepositoryReassignsOrganizationGroupWithExistingMembers(t *testing.T) {
	ctx := context.Background()
	fixture := newDesktopRepositoryFixture(t, "group-reassign", 10)
	member := fixture.createMember(t, "group-reassign", 1)
	require.NotNil(t, member.CurrentAPIKeyID)

	replacementUser := mustCreateUser(t, integrationEntClient, &service.User{
		Email: fmt.Sprintf("desktop-group-reassign-user-%d@example.com", fixture.suffix), APIKeyLimit: 10,
	})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", replacementUser.ID)
	})
	_, _, err := fixture.repo.UpdateOrganization(ctx, fixture.organization.PublicID, service.DesktopUpdateOrganizationInput{
		GatewayUserID: &replacementUser.ID,
	})
	require.ErrorIs(t, err, service.ErrDesktopProvisioningLocked)

	targetGroup := mustCreateGroup(t, integrationEntClient, &service.Group{
		Name: fmt.Sprintf("desktop-group-reassign-target-%d", fixture.suffix), RateMultiplier: 1, IsExclusive: true,
	})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "UPDATE desktop_organizations SET group_id = $1 WHERE id = $2", fixture.group.ID, fixture.organization.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "UPDATE api_keys SET group_id = $1 WHERE user_id = $2 AND group_id = $3", fixture.group.ID, fixture.user.ID, targetGroup.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM user_allowed_groups WHERE user_id = $1 AND group_id = $2", fixture.user.ID, targetGroup.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM groups WHERE id = $1", targetGroup.ID)
	})

	updated, invalidated, err := fixture.repo.UpdateOrganization(ctx, fixture.organization.PublicID, service.DesktopUpdateOrganizationInput{
		GroupID: &targetGroup.ID,
	})
	require.NoError(t, err)
	require.Equal(t, targetGroup.ID, updated.GroupID)
	require.Equal(t, fixture.user.ID, updated.GatewayUserID)
	require.Equal(t, []string{member.CurrentAPIKey}, invalidated)

	var keyGroupID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT group_id FROM api_keys WHERE id = $1", *member.CurrentAPIKeyID).Scan(&keyGroupID))
	require.Equal(t, targetGroup.ID, keyGroupID)

	var grantCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM user_allowed_groups WHERE user_id = $1 AND group_id = $2", fixture.user.ID, targetGroup.ID).Scan(&grantCount))
	require.Equal(t, 1, grantCount)
}

func TestDesktopRepositoryConcurrentGatewayUserAssignmentAllowsOneOrganization(t *testing.T) {
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	group := mustCreateGroup(t, integrationEntClient, &service.Group{
		Name: fmt.Sprintf("desktop-group-%d", suffix), RateMultiplier: 1,
	})
	user := mustCreateUser(t, integrationEntClient, &service.User{
		Email: fmt.Sprintf("desktop-carrier-%d@example.com", suffix), APIKeyLimit: 10,
	})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM desktop_organizations WHERE gateway_user_id = $1", user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM groups WHERE id = $1", group.ID)
	})
	repo := NewDesktopRepository(integrationEntClient, NewAPIKeyRepository(integrationEntClient, integrationDB))

	inputs := []service.DesktopCreateOrganizationInput{
		{PublicID: fmt.Sprintf("org_concurrent_a_%d", suffix), Code: fmt.Sprintf("a%x", suffix%100000), Name: "Desktop A", GatewayUserID: user.ID, GroupID: group.ID},
		{PublicID: fmt.Sprintf("org_concurrent_b_%d", suffix), Code: fmt.Sprintf("b%x", suffix%100000), Name: "Desktop B", GatewayUserID: user.ID, GroupID: group.ID},
	}
	errorsByAttempt := make(chan error, len(inputs))
	var start sync.WaitGroup
	start.Add(1)
	for _, input := range inputs {
		go func(candidate service.DesktopCreateOrganizationInput) {
			start.Wait()
			_, err := repo.CreateOrganization(ctx, candidate)
			errorsByAttempt <- err
		}(input)
	}
	start.Done()

	succeeded := 0
	conflicted := 0
	for range inputs {
		err := <-errorsByAttempt
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, service.ErrDesktopGatewayUserAssigned):
			conflicted++
		default:
			require.NoError(t, err)
		}
	}
	require.Equal(t, 1, succeeded)
	require.Equal(t, 1, conflicted)
}

func TestDesktopRepositoryCreateMemberRollsBackWhenAPIKeyCreationFails(t *testing.T) {
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	group := mustCreateGroup(t, integrationEntClient, &service.Group{
		Name: fmt.Sprintf("desktop-rollback-group-%d", suffix), RateMultiplier: 1,
	})
	user := mustCreateUser(t, integrationEntClient, &service.User{
		Email: fmt.Sprintf("desktop-rollback-%d@example.com", suffix), APIKeyLimit: 10,
	})
	apiKeys := NewAPIKeyRepository(integrationEntClient, integrationDB)
	repo := NewDesktopRepository(integrationEntClient, apiKeys)
	organization, err := repo.CreateOrganization(ctx, service.DesktopCreateOrganizationInput{
		PublicID: fmt.Sprintf("org_rollback_%d", suffix), Code: fmt.Sprintf("r%x", suffix%100000), Name: "Rollback", GatewayUserID: user.ID, GroupID: group.ID,
	})
	require.NoError(t, err)
	existingKey := &service.APIKey{UserID: user.ID, Key: fmt.Sprintf("sk-desktop-duplicate-%d", suffix), Name: "Existing", Status: service.StatusAPIKeyActive}
	require.NoError(t, apiKeys.Create(ctx, existingKey))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM desktop_member_api_keys WHERE member_id IN (SELECT id FROM desktop_members WHERE organization_id = $1)", organization.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM desktop_members WHERE organization_id = $1", organization.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM desktop_organizations WHERE id = $1", organization.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM api_keys WHERE user_id = $1", user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM groups WHERE id = $1", group.ID)
	})

	_, err = repo.CreateMember(ctx, organization.PublicID, service.DesktopCreateMemberInput{
		Member: &service.DesktopMember{
			PublicID: fmt.Sprintf("mem_rollback_%d", suffix), Name: "Member", NameNormalized: "Member",
			Phone: desktopTestPhone(suffix), Status: service.DesktopStatusActive,
		},
		APIKey: &service.APIKey{UserID: user.ID, Key: existingKey.Key, Name: "Duplicate", Status: service.StatusAPIKeyActive},
	})
	require.Error(t, err)

	var memberCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM desktop_members WHERE organization_id = $1", organization.ID).Scan(&memberCount))
	require.Zero(t, memberCount)
	var keyCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM api_keys WHERE user_id = $1 AND deleted_at IS NULL", user.ID).Scan(&keyCount))
	require.Equal(t, 1, keyCount)
}

func TestDesktopRepositoryListAvailableGatewayUsersFiltersAssignedAndDisabledUsers(t *testing.T) {
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	group := mustCreateGroup(t, integrationEntClient, &service.Group{
		Name: fmt.Sprintf("desktop-available-group-%d", suffix), RateMultiplier: 1,
	})
	freeUser := mustCreateUser(t, integrationEntClient, &service.User{
		Email: fmt.Sprintf("desktop-available-free-%d@example.com", suffix), APIKeyLimit: 10,
	})
	assignedUser := mustCreateUser(t, integrationEntClient, &service.User{
		Email: fmt.Sprintf("desktop-available-assigned-%d@example.com", suffix), APIKeyLimit: 10,
	})
	disabledUser := mustCreateUser(t, integrationEntClient, &service.User{
		Email: fmt.Sprintf("desktop-available-disabled-%d@example.com", suffix), Status: service.StatusDisabled, APIKeyLimit: 10,
	})
	repo := NewDesktopRepository(integrationEntClient, NewAPIKeyRepository(integrationEntClient, integrationDB))
	organization, err := repo.CreateOrganization(ctx, service.DesktopCreateOrganizationInput{
		PublicID: fmt.Sprintf("org_available_%d", suffix), Code: fmt.Sprintf("u%x", suffix%100000),
		Name: "Assigned", GatewayUserID: assignedUser.ID, GroupID: group.ID,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM desktop_organizations WHERE id = $1", organization.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id IN ($1, $2, $3)", freeUser.ID, assignedUser.ID, disabledUser.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM groups WHERE id = $1", group.ID)
	})

	users, result, err := repo.ListAvailableGatewayUsers(ctx, pagination.PaginationParams{Page: 1, PageSize: 20}, "desktop-available-")
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Total)
	require.Len(t, users, 1)
	require.Equal(t, freeUser.ID, users[0].ID)
}

func TestDesktopRepositoryOrganizationRestoreOnlyReactivatesOrganizationSuspendedKeys(t *testing.T) {
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	group := mustCreateGroup(t, integrationEntClient, &service.Group{
		Name: fmt.Sprintf("desktop-restore-group-%d", suffix), RateMultiplier: 1,
	})
	user := mustCreateUser(t, integrationEntClient, &service.User{
		Email: fmt.Sprintf("desktop-restore-%d@example.com", suffix), APIKeyLimit: 10,
	})
	repo := NewDesktopRepository(integrationEntClient, NewAPIKeyRepository(integrationEntClient, integrationDB))
	organization, err := repo.CreateOrganization(ctx, service.DesktopCreateOrganizationInput{
		PublicID: fmt.Sprintf("org_restore_%d", suffix), Code: fmt.Sprintf("s%x", suffix%100000),
		Name: "Restore", GatewayUserID: user.ID, GroupID: group.ID,
	})
	require.NoError(t, err)

	createMember := func(label string, offset int64) *service.DesktopMember {
		member, createErr := repo.CreateMember(ctx, organization.PublicID, service.DesktopCreateMemberInput{
			Member: &service.DesktopMember{
				PublicID: fmt.Sprintf("mem_%s_%d", label, suffix), Name: label, NameNormalized: label,
				Phone:  desktopTestPhone(suffix + offset),
				Status: service.DesktopStatusActive,
			},
			APIKey: &service.APIKey{
				Key: fmt.Sprintf("sk-desktop-%s-%d", label, suffix), Name: "Desktop " + label,
				Status: service.StatusAPIKeyActive,
			},
		})
		require.NoError(t, createErr)
		return member
	}
	activeMember := createMember("active", 1)
	disabledMember := createMember("disabled", 2)
	disabled := service.DesktopStatusDisabled
	_, _, err = repo.UpdateMember(ctx, organization.PublicID, disabledMember.PublicID, service.DesktopUpdateMemberInput{
		Status: &disabled, RevokeCredential: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM desktop_member_api_keys WHERE member_id IN (SELECT id FROM desktop_members WHERE organization_id = $1)", organization.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM desktop_members WHERE organization_id = $1", organization.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM desktop_organizations WHERE id = $1", organization.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM api_keys WHERE user_id = $1", user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM groups WHERE id = $1", group.ID)
	})

	loadState := func(memberID int64) (string, bool, string) {
		var memberStatus, keyStatus string
		var suspended bool
		err := integrationDB.QueryRowContext(ctx, `
			SELECT m.status, m.api_key_suspended_by_organization, k.status
			FROM desktop_members m
			JOIN desktop_member_api_keys a ON a.member_id = m.id AND a.retired_at IS NULL
			JOIN api_keys k ON k.id = a.api_key_id
			WHERE m.id = $1`, memberID).Scan(&memberStatus, &suspended, &keyStatus)
		require.NoError(t, err)
		return memberStatus, suspended, keyStatus
	}

	_, _, err = repo.UpdateOrganization(ctx, organization.PublicID, service.DesktopUpdateOrganizationInput{Status: &disabled})
	require.NoError(t, err)
	memberStatus, suspended, keyStatus := loadState(activeMember.ID)
	require.Equal(t, service.DesktopStatusActive, memberStatus)
	require.True(t, suspended)
	require.Equal(t, service.StatusAPIKeyDisabled, keyStatus)
	memberStatus, suspended, keyStatus = loadState(disabledMember.ID)
	require.Equal(t, service.DesktopStatusDisabled, memberStatus)
	require.False(t, suspended)
	require.Equal(t, service.StatusAPIKeyDisabled, keyStatus)

	active := service.DesktopStatusActive
	_, _, err = repo.UpdateOrganization(ctx, organization.PublicID, service.DesktopUpdateOrganizationInput{Status: &active})
	require.NoError(t, err)
	memberStatus, suspended, keyStatus = loadState(activeMember.ID)
	require.Equal(t, service.DesktopStatusActive, memberStatus)
	require.False(t, suspended)
	require.Equal(t, service.StatusAPIKeyActive, keyStatus)
	memberStatus, suspended, keyStatus = loadState(disabledMember.ID)
	require.Equal(t, service.DesktopStatusDisabled, memberStatus)
	require.False(t, suspended)
	require.Equal(t, service.StatusAPIKeyDisabled, keyStatus)
}

func TestDesktopRepositoryConcurrentDuplicatePhoneAllowsOneMember(t *testing.T) {
	fixture := newDesktopRepositoryFixture(t, "phone", 10)
	phone := desktopTestPhone(fixture.suffix)
	inputs := []service.DesktopCreateMemberInput{
		{
			Member: &service.DesktopMember{
				PublicID: fmt.Sprintf("mem_phone_a_%d", fixture.suffix), Name: "Member A", NameNormalized: "Member A",
				Phone: phone, Status: service.DesktopStatusActive,
			},
			APIKey: &service.APIKey{Key: fmt.Sprintf("sk-desktop-phone-a-%d", fixture.suffix), Name: "Desktop A", Status: service.StatusAPIKeyActive},
		},
		{
			Member: &service.DesktopMember{
				PublicID: fmt.Sprintf("mem_phone_b_%d", fixture.suffix), Name: "Member B", NameNormalized: "Member B",
				Phone: phone, Status: service.DesktopStatusActive,
			},
			APIKey: &service.APIKey{Key: fmt.Sprintf("sk-desktop-phone-b-%d", fixture.suffix), Name: "Desktop B", Status: service.StatusAPIKeyActive},
		},
	}

	errorsByAttempt := make(chan error, len(inputs))
	var start sync.WaitGroup
	start.Add(1)
	for _, input := range inputs {
		go func(candidate service.DesktopCreateMemberInput) {
			start.Wait()
			_, err := fixture.repo.CreateMember(context.Background(), fixture.organization.PublicID, candidate)
			errorsByAttempt <- err
		}(input)
	}
	start.Done()

	succeeded := 0
	conflicted := 0
	for range inputs {
		err := <-errorsByAttempt
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, service.ErrDesktopValidation):
			conflicted++
		default:
			require.NoError(t, err)
		}
	}
	require.Equal(t, 1, succeeded)
	require.Equal(t, 1, conflicted)

	var memberCount, keyCount, assignmentCount int
	require.NoError(t, integrationDB.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM desktop_members WHERE organization_id = $1 AND deleted_at IS NULL", fixture.organization.ID).Scan(&memberCount))
	require.NoError(t, integrationDB.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM api_keys WHERE user_id = $1 AND deleted_at IS NULL", fixture.user.ID).Scan(&keyCount))
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), `
		SELECT COUNT(*)
		FROM desktop_member_api_keys a
		JOIN desktop_members m ON m.id = a.member_id
		WHERE m.organization_id = $1 AND a.retired_at IS NULL`, fixture.organization.ID).Scan(&assignmentCount))
	require.Equal(t, 1, memberCount)
	require.Equal(t, 1, keyCount)
	require.Equal(t, 1, assignmentCount)
}

func TestDesktopRepositoryRotateAtAPIKeyLimitPreservesHistoryAndEnqueuesInvalidation(t *testing.T) {
	fixture := newDesktopRepositoryFixture(t, "limit", 1)
	member := fixture.createMember(t, "limit", 1)
	require.NotNil(t, member.CurrentAPIKeyID)
	oldKeyID := *member.CurrentAPIKeyID
	oldKey := member.CurrentAPIKey
	oldHash := sha256.Sum256([]byte(oldKey))
	cacheKey := hex.EncodeToString(oldHash[:])
	_, err := integrationDB.ExecContext(context.Background(), "DELETE FROM auth_cache_invalidation_outbox WHERE cache_key = $1", cacheKey)
	require.NoError(t, err)

	newKey := &service.APIKey{
		Key: fmt.Sprintf("sk-desktop-limit-rotated-%d", fixture.suffix), Name: "Desktop rotated", Status: service.StatusAPIKeyActive,
	}
	rotated, invalidated, err := fixture.repo.RotateMemberAPIKey(context.Background(), fixture.organization.PublicID, member.PublicID, newKey)
	require.NoError(t, err)
	require.Equal(t, []string{oldKey}, invalidated)
	require.NotNil(t, rotated.CurrentAPIKeyID)
	require.Equal(t, newKey.ID, *rotated.CurrentAPIKeyID)

	var oldDeleted bool
	require.NoError(t, integrationDB.QueryRowContext(context.Background(),
		"SELECT deleted_at IS NOT NULL FROM api_keys WHERE id = $1", oldKeyID).Scan(&oldDeleted))
	require.True(t, oldDeleted)
	var historyCount, currentCount int
	var currentKeyID int64
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE retired_at IS NULL),
		       MAX(api_key_id) FILTER (WHERE retired_at IS NULL)
		FROM desktop_member_api_keys WHERE member_id = $1`, member.ID).Scan(&historyCount, &currentCount, &currentKeyID))
	require.Equal(t, 2, historyCount)
	require.Equal(t, 1, currentCount)
	require.Equal(t, newKey.ID, currentKeyID)
	var outboxCount int
	require.NoError(t, integrationDB.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM auth_cache_invalidation_outbox WHERE cache_key = $1", cacheKey).Scan(&outboxCount))
	require.GreaterOrEqual(t, outboxCount, 1)
}

func TestDesktopRepositoryRotateFailureRestoresOldKeyAndAssignment(t *testing.T) {
	fixture := newDesktopRepositoryFixture(t, "rollback", 2)
	member := fixture.createMember(t, "rollback", 1)
	require.NotNil(t, member.CurrentAPIKeyID)
	oldKeyID := *member.CurrentAPIKeyID
	oldKey := member.CurrentAPIKey
	extraKey := &service.APIKey{
		UserID: fixture.user.ID, Key: fmt.Sprintf("sk-desktop-rotation-duplicate-%d", fixture.suffix),
		Name: "Duplicate target", Status: service.StatusAPIKeyActive,
	}
	require.NoError(t, fixture.apiKeys.Create(context.Background(), extraKey))

	_, _, err := fixture.repo.RotateMemberAPIKey(context.Background(), fixture.organization.PublicID, member.PublicID, &service.APIKey{
		Key: extraKey.Key, Name: "Conflicting rotation", Status: service.StatusAPIKeyActive,
	})
	require.Error(t, err)

	var storedKey, status string
	var oldActive bool
	require.NoError(t, integrationDB.QueryRowContext(context.Background(),
		"SELECT key, status, deleted_at IS NULL FROM api_keys WHERE id = $1", oldKeyID).Scan(&storedKey, &status, &oldActive))
	require.Equal(t, oldKey, storedKey)
	require.Equal(t, service.StatusAPIKeyActive, status)
	require.True(t, oldActive)
	var historyCount, currentCount int
	var currentKeyID int64
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE retired_at IS NULL),
		       MAX(api_key_id) FILTER (WHERE retired_at IS NULL)
		FROM desktop_member_api_keys WHERE member_id = $1`, member.ID).Scan(&historyCount, &currentCount, &currentKeyID))
	require.Equal(t, 1, historyCount)
	require.Equal(t, 1, currentCount)
	require.Equal(t, oldKeyID, currentKeyID)
}

func TestDesktopGatewayUserStatusChangesBumpOrganizationAuthVersion(t *testing.T) {
	fixture := newDesktopRepositoryFixture(t, "version", 1)
	initial := fixture.organization.AuthVersion

	_, err := integrationDB.ExecContext(context.Background(), "UPDATE users SET status = 'disabled' WHERE id = $1", fixture.user.ID)
	require.NoError(t, err)
	var disabledVersion int64
	require.NoError(t, integrationDB.QueryRowContext(context.Background(),
		"SELECT auth_version FROM desktop_organizations WHERE id = $1", fixture.organization.ID).Scan(&disabledVersion))
	require.Equal(t, initial+1, disabledVersion)

	_, err = integrationDB.ExecContext(context.Background(), "UPDATE users SET balance = balance + 1 WHERE id = $1", fixture.user.ID)
	require.NoError(t, err)
	var unchangedVersion int64
	require.NoError(t, integrationDB.QueryRowContext(context.Background(),
		"SELECT auth_version FROM desktop_organizations WHERE id = $1", fixture.organization.ID).Scan(&unchangedVersion))
	require.Equal(t, disabledVersion, unchangedVersion)

	_, err = integrationDB.ExecContext(context.Background(), "UPDATE users SET status = 'active' WHERE id = $1", fixture.user.ID)
	require.NoError(t, err)
	var activeVersion int64
	require.NoError(t, integrationDB.QueryRowContext(context.Background(),
		"SELECT auth_version FROM desktop_organizations WHERE id = $1", fixture.organization.ID).Scan(&activeVersion))
	require.Equal(t, initial+2, activeVersion)
}

type desktopRepositoryFixture struct {
	repo         service.DesktopRepository
	apiKeys      service.APIKeyRepository
	organization *service.DesktopOrganization
	user         *service.User
	group        *service.Group
	suffix       int64
}

func newDesktopRepositoryFixture(t *testing.T, label string, apiKeyLimit int) *desktopRepositoryFixture {
	t.Helper()
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	group := mustCreateGroup(t, integrationEntClient, &service.Group{
		Name: fmt.Sprintf("desktop-%s-group-%d", label, suffix), RateMultiplier: 1,
	})
	user := mustCreateUser(t, integrationEntClient, &service.User{
		Email: fmt.Sprintf("desktop-%s-%d@example.com", label, suffix), APIKeyLimit: apiKeyLimit,
	})
	apiKeys := NewAPIKeyRepository(integrationEntClient, integrationDB)
	repo := NewDesktopRepository(integrationEntClient, apiKeys)
	organization, err := repo.CreateOrganization(ctx, service.DesktopCreateOrganizationInput{
		PublicID: fmt.Sprintf("org_%s_%d", label, suffix), Code: fmt.Sprintf("%c%x", label[0], suffix%100000),
		Name: "Desktop " + label, GatewayUserID: user.ID, GroupID: group.ID,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM desktop_member_api_keys WHERE member_id IN (SELECT id FROM desktop_members WHERE organization_id = $1)", organization.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM desktop_members WHERE organization_id = $1", organization.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM desktop_organizations WHERE id = $1", organization.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM api_keys WHERE user_id = $1", user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM groups WHERE id = $1", group.ID)
	})
	return &desktopRepositoryFixture{repo: repo, apiKeys: apiKeys, organization: organization, user: user, group: group, suffix: suffix}
}

func (f *desktopRepositoryFixture) createMember(t *testing.T, label string, offset int64) *service.DesktopMember {
	t.Helper()
	member, err := f.repo.CreateMember(context.Background(), f.organization.PublicID, service.DesktopCreateMemberInput{
		Member: &service.DesktopMember{
			PublicID: fmt.Sprintf("mem_%s_%d", label, f.suffix), Name: "Member " + label, NameNormalized: "Member " + label,
			Phone: desktopTestPhone(f.suffix + offset), Status: service.DesktopStatusActive,
		},
		APIKey: &service.APIKey{
			Key: fmt.Sprintf("sk-desktop-%s-%d", label, f.suffix), Name: "Desktop " + label, Status: service.StatusAPIKeyActive,
		},
	})
	require.NoError(t, err)
	return member
}

func desktopTestPhone(seed int64) string {
	value := seed % 1_000_000_000
	if value < 0 {
		value = -value
	}
	return fmt.Sprintf("+8613%09d", value)
}
