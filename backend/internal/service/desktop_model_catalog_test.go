package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type desktopCatalogStub struct {
	ids      []string
	err      error
	groupIDs *[]int64
}

func (s desktopCatalogStub) Models(_ context.Context, groupID int64) ([]string, error) {
	if s.groupIDs != nil {
		*s.groupIDs = append(*s.groupIDs, groupID)
	}
	return s.ids, s.err
}

func TestDesktopUnifiedModelsVisibilityAndLegacyTargets(t *testing.T) {
	authorized := activeDesktopAuthorization()
	authorized.Organization.GroupID = 12
	authorized.Organization.TargetConfig.SchemaVersion = 2
	authorized.Organization.TargetConfig.Models = []DesktopModel{
		{Model: "visible-enabled", Visible: true, Enabled: true, Remark: "ready"},
		{Model: "visible-disabled", Visible: true, Remark: "maintenance"},
		{Model: "hidden-enabled", Enabled: true, Remark: "private"},
		{Model: "hidden-disabled", Remark: "private too"},
		{Model: "removed", Visible: true, Enabled: true},
	}
	ids := []string{"visible-enabled", "visible-disabled", "hidden-enabled", "hidden-disabled", "new-model"}
	groups := []int64{}
	svc := (&DesktopService{}).WithModelCatalog(desktopCatalogStub{ids: ids, groupIDs: &groups})
	configuration, tag, err := svc.ModelConfiguration(context.Background(), authorized, []string{"workbuddy"})
	require.NoError(t, err)
	require.Equal(t, []int64{12}, groups)
	require.Len(t, configuration.Models, 2)
	require.Equal(t, "maintenance", configuration.Models[1].Remark)
	require.False(t, configuration.Models[1].Enabled)
	require.Len(t, configuration.Targets, 1)
	require.Equal(t, "visible-enabled", configuration.Targets["workbuddy"].RequestedModel)
	require.Equal(t, DesktopWireAPIChatCompletions, configuration.Targets["workbuddy"].WireAPI)
	admin, err := svc.ResolveOrganizationModels(context.Background(), authorized.Organization, false)
	require.NoError(t, err)
	require.Len(t, admin.TargetConfig.Models, 5)
	require.Equal(t, DesktopModel{Model: "new-model"}, admin.TargetConfig.Models[4])
	require.Equal(t, "visible-enabled", admin.TargetConfig.Targets.ChatGPTCodex.RequestedModel)
	require.Equal(t, "visible-enabled", admin.TargetConfig.Targets.Workbuddy.RequestedModel)
	// A projection must never overwrite stored policy or legacy provider metadata.
	require.Len(t, authorized.Organization.TargetConfig.Models, 5)
	require.Nil(t, authorized.Organization.TargetConfig.Targets.Workbuddy)
	authorized.Organization.TargetConfig.Models[1].Remark = "updated"
	_, changed, err := svc.ModelConfiguration(context.Background(), authorized, []string{"workbuddy"})
	require.NoError(t, err)
	require.NotEqual(t, tag, changed)
}

func TestDesktopUnifiedModelsLegacyUnionAndEmptyConfigurations(t *testing.T) {
	authorized := activeDesktopAuthorization()
	authorized.Organization.TargetConfig.Targets.ChatGPTCodex.RequestedModel = " model-one,model-two,removed,model-one "
	authorized.Organization.TargetConfig.Targets.Workbuddy = &DesktopTarget{Enabled: true, RequestedModel: "model-two,model-three", ProviderID: "work", DisplayName: "Work", WireAPI: DesktopWireAPIChatCompletions}
	svc := (&DesktopService{}).WithModelCatalog(desktopCatalogStub{ids: []string{"model-one", "model-two", "model-three", "new"}})
	configuration, _, err := svc.ModelConfiguration(context.Background(), authorized, nil)
	require.NoError(t, err)
	require.Len(t, configuration.Models, 3)
	require.Equal(t, "model-one,model-two,model-three", configuration.Targets["chatgpt_codex"].RequestedModel)
	require.Equal(t, "work", configuration.Targets["workbuddy"].ProviderID)
	require.Equal(t, 1, authorized.Organization.TargetConfig.SchemaVersion)
	for _, stored := range []*DesktopTargetConfig{nil, {SchemaVersion: 2, Models: []DesktopModel{}}} {
		authorized.Organization.TargetConfig = stored
		configuration, _, err = svc.ModelConfiguration(context.Background(), authorized, nil)
		require.NoError(t, err)
		require.NotNil(t, configuration.Models)
		require.Empty(t, configuration.Models)
		require.NotNil(t, configuration.Targets)
		require.Empty(t, configuration.Targets)
	}
	svc.WithModelCatalog(desktopCatalogStub{err: errors.New("catalog unavailable")})
	_, _, err = svc.ModelConfiguration(context.Background(), authorized, nil)
	require.EqualError(t, err, "catalog unavailable")
	authorized.Member.CurrentAPIKeyDeleted = true
	_, _, err = svc.ModelConfiguration(context.Background(), authorized, nil)
	require.ErrorIs(t, err, ErrDesktopConfigurationMissing)
}

func TestDesktopUnifiedModelValidation(t *testing.T) {
	for _, models := range [][]DesktopModel{{{Model: ""}}, {{Model: "one"}, {Model: "ONE"}}} {
		require.ErrorIs(t, (&DesktopTargetConfig{SchemaVersion: 2, Models: models}).Validate(), ErrDesktopValidation)
	}
	decoded, err := DecodeDesktopTargetConfig([]byte(`{"schema_version":2,"models":[],"targets":{}}`))
	require.NoError(t, err)
	require.Equal(t, 2, decoded.SchemaVersion)
}

type desktopCatalogGroupRepo struct {
	GroupRepository
	group *Group
}

func (r desktopCatalogGroupRepo) GetByID(context.Context, int64) (*Group, error) { return r.group, nil }

type desktopCatalogAccountRepo struct {
	AccountRepository
	accounts map[int64][]Account
	err      error
}

func (r desktopCatalogAccountRepo) ListSchedulableByGroupID(_ context.Context, id int64) ([]Account, error) {
	return r.accounts[id], r.err
}

func TestDesktopCatalogUsesGroupMappingAllowlistAndPropagatesErrors(t *testing.T) {
	group := &Group{ID: 12, Platform: PlatformOpenAI, ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"public-*"}}}
	accounts := desktopCatalogAccountRepo{accounts: map[int64][]Account{
		12: {{ID: 1, Platform: PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"public-one": "upstream-one", "private": "upstream-two"}}}},
		13: {{ID: 2, Platform: PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"other-enterprise": "upstream-one"}}}},
	}}
	catalog := &desktopGroupModelCatalog{groups: desktopCatalogGroupRepo{group: group}, gateway: &GatewayService{accountRepo: accounts}}
	models, err := catalog.Models(context.Background(), group.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"public-one"}, models)
	accounts.err = errors.New("catalog database offline")
	catalog.gateway.accountRepo = accounts
	_, err = catalog.Models(context.Background(), group.ID)
	require.EqualError(t, err, "catalog database offline")
	// The public Gateway method retains its pre-existing error fallback.
	require.Nil(t, catalog.gateway.GetAvailableModels(context.Background(), &group.ID, group.Platform))
}

func TestDesktopUnifiedModelSaveValidatesCurrentCatalogBeforePersistence(t *testing.T) {
	org := activeDesktopAuthorization().Organization
	repo := &desktopRepositoryStub{organization: org}
	svc := (&DesktopService{repo: repo}).WithModelCatalog(desktopCatalogStub{ids: []string{"model-one", "new"}})
	for _, catalog := range []desktopCatalogStub{{err: errors.New("offline")}, {ids: []string{"other"}}} {
		svc.WithModelCatalog(catalog)
		_, err := svc.UpdateTargetConfig(context.Background(), org.PublicID, &DesktopTargetConfig{SchemaVersion: 2, Models: []DesktopModel{{Model: "model-one", Visible: true, Enabled: true}}})
		require.Error(t, err)
		require.Zero(t, repo.targetConfigUpdates)
	}
	svc.WithModelCatalog(desktopCatalogStub{ids: []string{"model-one", "new"}})
	updated, err := svc.UpdateTargetConfig(context.Background(), org.PublicID, &DesktopTargetConfig{SchemaVersion: 2, Models: []DesktopModel{{Model: "model-one", Visible: true, Enabled: false, Remark: " maintenance "}}})
	require.NoError(t, err)
	require.Equal(t, 1, repo.targetConfigUpdates)
	require.Equal(t, 2, repo.organization.TargetConfig.SchemaVersion)
	require.Equal(t, "maintenance", updated.TargetConfig.Models[0].Remark)
	require.False(t, updated.TargetConfig.Models[1].Visible)
	require.Empty(t, updated.TargetConfig.Targets)
	require.Equal(t, "provider", repo.organization.TargetConfig.Targets.ChatGPTCodex.ProviderID)
}
