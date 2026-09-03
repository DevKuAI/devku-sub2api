package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type desktopRepositoryStub struct {
	authorized                     *DesktopAuthorizedMember
	organization                   *DesktopOrganization
	members                        []DesktopMember
	listMemberFilters              []DesktopMemberListFilters
	scopedUserIDs                  []int64
	updateOrganizationInputs       []DesktopUpdateOrganizationInput
	keyIDs                         []int64
	hasUserOrganization            bool
	hasGroupOrganization           bool
	userOrganizationGroupID        int64
	userOrganizationGroupExclusive bool
	userOrganizationGroupPresent   bool
}

func (s *desktopRepositoryStub) ScopedToGatewayUser(userID int64) DesktopRepository {
	s.scopedUserIDs = append(s.scopedUserIDs, userID)
	return s
}

func (s *desktopRepositoryStub) CreateOrganization(context.Context, DesktopCreateOrganizationInput) (*DesktopOrganization, error) {
	return nil, nil
}
func (s *desktopRepositoryStub) ListOrganizations(context.Context, pagination.PaginationParams, DesktopOrganizationListFilters) ([]DesktopOrganization, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (s *desktopRepositoryStub) GetOrganization(context.Context, string) (*DesktopOrganization, error) {
	return s.organization, nil
}
func (s *desktopRepositoryStub) GetOrganizationForGatewayUser(context.Context, int64) (*DesktopOrganization, error) {
	if s.organization == nil {
		return nil, ErrDesktopOrganizationNotFound
	}
	return s.organization, nil
}
func (s *desktopRepositoryStub) UpdateOrganization(_ context.Context, _ string, input DesktopUpdateOrganizationInput) (*DesktopOrganization, []string, error) {
	s.updateOrganizationInputs = append(s.updateOrganizationInputs, input)
	return s.organization, nil, nil
}
func (s *desktopRepositoryStub) UpdateTargetConfig(context.Context, string, *DesktopTargetConfig) (*DesktopOrganization, error) {
	return nil, nil
}
func (s *desktopRepositoryStub) CreateMember(context.Context, string, DesktopCreateMemberInput) (*DesktopMember, error) {
	return nil, nil
}
func (s *desktopRepositoryStub) ListMembers(_ context.Context, _ string, params pagination.PaginationParams, filters DesktopMemberListFilters) ([]DesktopMember, *pagination.PaginationResult, error) {
	s.listMemberFilters = append(s.listMemberFilters, filters)
	members := append([]DesktopMember(nil), s.members...)
	return members, &pagination.PaginationResult{Total: int64(len(members)), Page: params.Page, PageSize: params.PageSize, Pages: 1}, nil
}
func (s *desktopRepositoryStub) GetMember(context.Context, string, string) (*DesktopMember, error) {
	return nil, nil
}
func (s *desktopRepositoryStub) UpdateMember(context.Context, string, string, DesktopUpdateMemberInput) (*DesktopMember, []string, error) {
	return nil, nil, nil
}
func (s *desktopRepositoryStub) DeleteMember(context.Context, string, string) ([]string, error) {
	return nil, nil
}
func (s *desktopRepositoryStub) RotateMemberAPIKey(context.Context, string, string, *APIKey) (*DesktopMember, []string, error) {
	return nil, nil, nil
}
func (s *desktopRepositoryStub) FindActiveOrganizationByCode(context.Context, string) (*DesktopOrganization, error) {
	return nil, nil
}
func (s *desktopRepositoryStub) FindMemberByPhone(context.Context, int64, string) (*DesktopMember, error) {
	return nil, nil
}
func (s *desktopRepositoryStub) GetAuthorizedMember(context.Context, string) (*DesktopAuthorizedMember, error) {
	return s.authorized, nil
}
func (s *desktopRepositoryStub) ListMemberAPIKeyIDs(context.Context, int64) ([]int64, error) {
	return append([]int64(nil), s.keyIDs...), nil
}
func (s *desktopRepositoryStub) IsManagedAPIKey(context.Context, int64) (bool, error) {
	return false, nil
}
func (s *desktopRepositoryStub) HasOrganizationForUser(context.Context, int64) (bool, error) {
	return s.hasUserOrganization, nil
}
func (s *desktopRepositoryStub) HasOrganizationForGroup(context.Context, int64) (bool, error) {
	return s.hasGroupOrganization, nil
}
func (s *desktopRepositoryStub) ListAvailableGatewayUsers(context.Context, pagination.PaginationParams, string) ([]User, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (s *desktopRepositoryStub) DesktopOrganizationGroupForUser(context.Context, int64) (int64, bool, bool, error) {
	return s.userOrganizationGroupID, s.userOrganizationGroupExclusive, s.userOrganizationGroupPresent, nil
}

type desktopUsageRepositoryStub struct {
	calls       [][]int64
	next        []*usagestats.UsageStats
	memberCalls []desktopMemberUsageCall
	memberUsage map[int64]*DesktopMemberUsage
}

type desktopMemberUsageCall struct {
	memberIDs       []int64
	todayStart      time.Time
	last30DaysStart time.Time
	endTime         time.Time
}

func (s *desktopUsageRepositoryStub) GetAPIKeysStatsAggregated(_ context.Context, ids []int64, _, _ time.Time) (*usagestats.UsageStats, error) {
	s.calls = append(s.calls, append([]int64(nil), ids...))
	result := s.next[0]
	s.next = s.next[1:]
	return result, nil
}

func (s *desktopUsageRepositoryStub) GetDesktopMembersUsage(_ context.Context, ids []int64, todayStart, last30DaysStart, endTime time.Time) (map[int64]*DesktopMemberUsage, error) {
	s.memberCalls = append(s.memberCalls, desktopMemberUsageCall{
		memberIDs: append([]int64(nil), ids...), todayStart: todayStart,
		last30DaysStart: last30DaysStart, endTime: endTime,
	})
	return s.memberUsage, nil
}

func activeDesktopAuthorization() *DesktopAuthorizedMember {
	keyID := int64(42)
	return &DesktopAuthorizedMember{
		Member: &DesktopMember{
			ID: 7, PublicID: "mem_one", Name: "Member", Phone: "+8613800000000", Status: DesktopStatusActive,
			AuthVersion: 2, CurrentAPIKeyID: &keyID, CurrentAPIKey: "sk-model-one", CurrentAPIKeyStatus: StatusAPIKeyActive,
		},
		Organization: &DesktopOrganization{
			PublicID: "org_one", Code: "desktop", Name: "Desktop", Status: DesktopStatusActive, AuthVersion: 3,
			TargetConfig: &DesktopTargetConfig{SchemaVersion: 1, Targets: DesktopTargets{
				ChatGPTCodex: &DesktopTarget{Enabled: true, ProviderID: "provider", DisplayName: "Model", RequestedModel: "model-one", WireAPI: "responses"},
			}},
		},
		GatewayUser: &User{ID: 9, Status: StatusActive},
	}
}

func TestDesktopTargetConfigValidatesWireAPIByTarget(t *testing.T) {
	newConfig := func(chatWireAPI, workbuddyWireAPI string, workbuddyEnabled bool) *DesktopTargetConfig {
		return &DesktopTargetConfig{SchemaVersion: 1, Targets: DesktopTargets{
			ChatGPTCodex: &DesktopTarget{
				Enabled: true, ProviderID: "provider", DisplayName: "Codex", RequestedModel: "model-one", WireAPI: chatWireAPI,
			},
			Workbuddy: &DesktopTarget{
				Enabled: workbuddyEnabled, ProviderID: "provider", DisplayName: "Workbuddy", RequestedModel: "model-two", WireAPI: workbuddyWireAPI,
			},
		}}
	}

	require.NoError(t, newConfig(DesktopWireAPIResponses, DesktopWireAPIChatCompletions, false).Validate())
	require.NoError(t, newConfig(DesktopWireAPIResponses, DesktopWireAPIChatCompletions, true).Validate())
	require.ErrorIs(t, newConfig(DesktopWireAPIChatCompletions, DesktopWireAPIChatCompletions, false).Validate(), ErrDesktopValidation)
	require.ErrorIs(t, newConfig(DesktopWireAPIResponses, DesktopWireAPIResponses, false).Validate(), ErrDesktopValidation)
}

func TestDesktopModelConfigurationETagIgnoresRawKeyAndUpdatedAt(t *testing.T) {
	authorized := activeDesktopAuthorization()
	svc := &DesktopService{config: config.DesktopConfig{PublicGatewayBaseURL: "https://gateway.example.com/v1"}}

	first, firstVersion, err := svc.ModelConfiguration(authorized, nil)
	require.NoError(t, err)
	require.Equal(t, "sk-model-one", first.ModelToken)

	authorized.Member.CurrentAPIKey = "sk-model-two"
	authorized.Member.UpdatedAt = time.Now().Add(24 * time.Hour)
	authorized.Organization.UpdatedAt = time.Now().Add(48 * time.Hour)
	second, secondVersion, err := svc.ModelConfiguration(authorized, nil)
	require.NoError(t, err)
	require.Equal(t, "sk-model-two", second.ModelToken)
	require.Equal(t, firstVersion, secondVersion)

	nextKeyID := *authorized.Member.CurrentAPIKeyID + 1
	authorized.Member.CurrentAPIKeyID = &nextKeyID
	_, rotatedVersion, err := svc.ModelConfiguration(authorized, nil)
	require.NoError(t, err)
	require.NotEqual(t, firstVersion, rotatedVersion)
}

func TestDesktopListMembersReturnsFullPhoneAndNormalizesExactSearch(t *testing.T) {
	repo := &desktopRepositoryStub{members: []DesktopMember{{
		PublicID: "mem_one", Name: "Member", Phone: "+8613800000000", Status: DesktopStatusActive,
	}}}
	svc := &DesktopService{repo: repo}

	for _, search := range []string{"13800000000", "+8613800000000"} {
		members, _, listErr := svc.ListMembers(context.Background(), "org_one", pagination.PaginationParams{Page: 1, PageSize: 20}, DesktopMemberListFilters{Search: search})
		require.NoError(t, listErr)
		require.Len(t, members, 1)
		require.Equal(t, "+8613800000000", members[0].Phone)
	}

	require.Len(t, repo.listMemberFilters, 2)
	require.Empty(t, repo.listMemberFilters[0].Search)
	require.Equal(t, "+8613800000000", repo.listMemberFilters[0].Phone)
	require.Equal(t, repo.listMemberFilters[0].Phone, repo.listMemberFilters[1].Phone)
}

func TestDesktopListMembersWithUsageAggregatesCurrentPage(t *testing.T) {
	repo := &desktopRepositoryStub{members: []DesktopMember{
		{ID: 7, PublicID: "mem_one", Name: "One", Phone: "+8613800000000", Status: DesktopStatusActive},
		{ID: 8, PublicID: "mem_two", Name: "Two", Phone: "+8613800000001", Status: DesktopStatusActive},
	}}
	memberUsage := &DesktopMemberUsage{
		TodayTokens: 100, Last30DaysTokens: 900, TotalTokens: 1200,
		TodayActualCost: 0.1, Last30DaysActualCost: 0.9, TotalActualCost: 1.2,
	}
	usage := &desktopUsageRepositoryStub{memberUsage: map[int64]*DesktopMemberUsage{7: memberUsage}}
	now := time.Date(2026, 9, 3, 6, 30, 0, 0, time.UTC)
	svc := &DesktopService{repo: repo, usage: usage, now: func() time.Time { return now }}

	members, _, err := svc.ListMembersWithUsage(
		context.Background(),
		"org_one",
		pagination.PaginationParams{Page: 1, PageSize: 20},
		DesktopMemberListFilters{},
	)

	require.NoError(t, err)
	require.Len(t, usage.memberCalls, 1)
	require.Equal(t, []int64{7, 8}, usage.memberCalls[0].memberIDs)
	require.Equal(t, now.AddDate(0, 0, -30), usage.memberCalls[0].last30DaysStart)
	require.Equal(t, now, usage.memberCalls[0].endTime)
	require.Equal(t, memberUsage, members[0].Usage)
	require.NotNil(t, members[1].Usage)
	require.Zero(t, members[1].Usage.TotalTokens)
}

func TestDesktopManagedOrganizationUsesGatewayUserScope(t *testing.T) {
	organization := &DesktopOrganization{PublicID: "org_one", GatewayUserID: 9}
	repo := &desktopRepositoryStub{organization: organization}
	svc := &DesktopService{repo: repo}
	gatewayUserID := int64(11)
	groupID := int64(12)
	name := "Managed"
	status := DesktopStatusActive

	updated, err := svc.UpdateManagedOrganization(context.Background(), 9, DesktopUpdateOrganizationInput{
		Name: &name, Status: &status, GatewayUserID: &gatewayUserID, GroupID: &groupID,
	})

	require.NoError(t, err)
	require.Same(t, organization, updated)
	require.Equal(t, []int64{9}, repo.scopedUserIDs)
	require.Len(t, repo.updateOrganizationInputs, 1)
	require.Nil(t, repo.updateOrganizationInputs[0].GatewayUserID)
	require.Nil(t, repo.updateOrganizationInputs[0].GroupID)
}

func TestDesktopManagedOrganizationRejectsMissingIdentity(t *testing.T) {
	svc := &DesktopService{repo: &desktopRepositoryStub{}}
	_, err := svc.GetManagedOrganization(context.Background(), 0)
	require.ErrorIs(t, err, ErrDesktopUnauthenticated)
}

func TestDesktopUsageSummaryAggregatesAllHistoricalKeys(t *testing.T) {
	repo := &desktopRepositoryStub{keyIDs: []int64{11, 12, 13}}
	usage := &desktopUsageRepositoryStub{next: []*usagestats.UsageStats{{TotalTokens: 100}, {TotalTokens: 900}}}
	svc := &DesktopService{repo: repo, usage: usage, now: func() time.Time {
		return time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	}}

	result, err := svc.UsageSummary(context.Background(), activeDesktopAuthorization(), "Asia/Shanghai")
	require.NoError(t, err)
	require.Equal(t, int64(100), result.Today.Used)
	require.Equal(t, int64(900), result.Month.Used)
	require.Nil(t, result.Today.Limit)
	require.Nil(t, result.Month.Remaining)
	require.Equal(t, [][]int64{{11, 12, 13}, {11, 12, 13}}, usage.calls)
}

func TestDesktopAuthorizeRevalidatesCarrierAndAuthVersions(t *testing.T) {
	cfg := desktopSecurityConfig()
	manager, err := NewDesktopTokenManager(cfg)
	require.NoError(t, err)
	authorized := activeDesktopAuthorization()
	token, err := manager.Issue(
		&DesktopMemberOrganizationClaims{PublicID: authorized.Member.PublicID, AuthVersion: authorized.Member.AuthVersion},
		&DesktopMemberOrganizationClaims{PublicID: authorized.Organization.PublicID, AuthVersion: authorized.Organization.AuthVersion},
		"family_one", time.Now(),
	)
	require.NoError(t, err)
	svc := &DesktopService{repo: &desktopRepositoryStub{authorized: authorized}, tokens: manager}

	_, _, err = svc.Authorize(context.Background(), token)
	require.NoError(t, err)

	authorized.GatewayUser.Status = StatusDisabled
	_, _, err = svc.Authorize(context.Background(), token)
	require.ErrorIs(t, err, ErrDesktopMembershipRevoked)

	authorized.GatewayUser.Status = StatusActive
	authorized.Organization.AuthVersion++
	_, _, err = svc.Authorize(context.Background(), token)
	require.ErrorIs(t, err, ErrDesktopMembershipRevoked)
}

func TestDesktopDependencyGuards(t *testing.T) {
	t.Run("gateway user deletion", func(t *testing.T) {
		svc := &DesktopService{repo: &desktopRepositoryStub{hasUserOrganization: true}}
		require.ErrorIs(t, svc.EnsureGatewayUserCanBeDeleted(context.Background(), 9), ErrDesktopDependency)
	})

	t.Run("group deletion", func(t *testing.T) {
		svc := &DesktopService{repo: &desktopRepositoryStub{hasGroupOrganization: true}}
		require.ErrorIs(t, svc.EnsureGroupCanBeDeleted(context.Background(), 7), ErrDesktopDependency)
	})

	tests := []struct {
		name                 string
		isExclusive          bool
		restrictPublicGroups bool
		allowedGroups        []int64
		wantErr              bool
	}{
		{name: "unrestricted public group", allowedGroups: []int64{8}},
		{name: "restricted public group removed", restrictPublicGroups: true, allowedGroups: []int64{8}, wantErr: true},
		{name: "exclusive group removed", isExclusive: true, allowedGroups: []int64{8}, wantErr: true},
		{name: "restricted group retained", isExclusive: true, restrictPublicGroups: true, allowedGroups: []int64{7, 8}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &DesktopService{repo: &desktopRepositoryStub{
				userOrganizationGroupID: 7, userOrganizationGroupExclusive: tt.isExclusive, userOrganizationGroupPresent: true,
			}}
			err := svc.EnsureGatewayUserGroupAccess(context.Background(), 9, tt.restrictPublicGroups, tt.allowedGroups)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrDesktopDependency)
				return
			}
			require.NoError(t, err)
		})
	}
}
