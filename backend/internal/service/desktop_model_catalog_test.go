package service

import (
	"context"
	"encoding/json"
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
	require.Len(t, configuration.Models, 1)
	require.Equal(t, "ready", configuration.Models[0].Remark)
	require.Len(t, configuration.Targets, 1)
	require.Equal(t, "visible-enabled", configuration.Targets["workbuddy"].RequestedModel)
	require.Equal(t, DesktopWireAPIChatCompletions, configuration.Targets["workbuddy"].WireAPI)
	admin, err := svc.ResolveOrganizationModels(context.Background(), authorized.Organization, false)
	require.NoError(t, err)
	require.Len(t, admin.TargetConfig.Models, 5)
	require.False(t, admin.TargetConfig.Models[1].Visible)
	require.False(t, admin.TargetConfig.Models[1].Enabled)
	require.Equal(t, DesktopModel{Model: "new-model"}, admin.TargetConfig.Models[4])
	require.Equal(t, "visible-enabled", admin.TargetConfig.Targets.ChatGPTCodex.RequestedModel)
	require.Equal(t, "visible-enabled", admin.TargetConfig.Targets.Workbuddy.RequestedModel)
	// A projection must never overwrite stored policy or legacy provider metadata.
	require.Len(t, authorized.Organization.TargetConfig.Models, 5)
	require.True(t, authorized.Organization.TargetConfig.Models[1].Visible)
	require.Nil(t, authorized.Organization.TargetConfig.Targets.Workbuddy)
	authorized.Organization.TargetConfig.Models[0].Remark = "updated"
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
		require.Len(t, configuration.Targets, 2)
		require.Empty(t, configuration.Targets["workbuddy"].RequestedModel)
		require.Equal(t, authorized.Organization.Name, configuration.Targets["workbuddy"].DisplayName)
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
	decoded, err = DecodeDesktopTargetConfig([]byte(`{"schema_version":2,"models":[{"model":"disabled","visible":true,"enabled":false,"remark":"maintenance"}],"targets":{}}`))
	require.NoError(t, err)
	require.False(t, decoded.Models[0].Visible)
	require.False(t, decoded.Models[0].Enabled)
	require.Equal(t, "maintenance", decoded.Models[0].Remark)
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
	require.False(t, repo.organization.TargetConfig.Models[0].Visible)
	require.False(t, updated.TargetConfig.Models[0].Visible)
	require.False(t, updated.TargetConfig.Models[1].Visible)
	require.Empty(t, updated.TargetConfig.Targets.ChatGPTCodex.RequestedModel)
	require.Empty(t, updated.TargetConfig.Targets.Workbuddy.RequestedModel)
	require.Equal(t, "provider", repo.organization.TargetConfig.Targets.ChatGPTCodex.ProviderID)
}

func TestDesktopUnifiedModelsSaveIndependentClientSettings(t *testing.T) {
	org := activeDesktopAuthorization().Organization
	repo := &desktopRepositoryStub{organization: org}
	svc := (&DesktopService{repo: repo}).WithModelCatalog(desktopCatalogStub{ids: []string{"model-one"}})
	version := "2.3.4"
	chat := &DesktopTarget{Enabled: true, ProviderID: "chat-provider", DisplayName: "Codex display", WireAPI: DesktopWireAPIResponses, MinimumAppVersion: &version, RestartRequired: true}
	work := &DesktopTarget{Enabled: false, ProviderID: "work-provider", DisplayName: "Workbuddy display", WireAPI: DesktopWireAPIChatCompletions}
	updated, err := svc.UpdateTargetConfig(context.Background(), org.PublicID, &DesktopTargetConfig{
		SchemaVersion: 2, Models: []DesktopModel{{Model: "model-one", Visible: true, Enabled: true}},
		Targets: DesktopTargets{ChatGPTCodex: chat, Workbuddy: work},
	})
	require.NoError(t, err)
	require.Equal(t, "Codex display", repo.organization.TargetConfig.Targets.ChatGPTCodex.DisplayName)
	require.Equal(t, "Workbuddy display", repo.organization.TargetConfig.Targets.Workbuddy.DisplayName)
	require.False(t, updated.TargetConfig.Targets.Workbuddy.Enabled)
	require.Equal(t, "model-one", updated.TargetConfig.Targets.ChatGPTCodex.RequestedModel)
	require.Equal(t, "model-one", updated.TargetConfig.Targets.Workbuddy.RequestedModel)
	require.Equal(t, &version, updated.TargetConfig.Targets.ChatGPTCodex.MinimumAppVersion)
	require.True(t, updated.TargetConfig.Targets.ChatGPTCodex.RestartRequired)
	require.Nil(t, updated.TargetConfig.Targets.Workbuddy.MinimumAppVersion)
	require.False(t, updated.TargetConfig.Targets.Workbuddy.RestartRequired)
	authorized := activeDesktopAuthorization()
	authorized.Organization = repo.organization
	payload, _, err := svc.ModelConfiguration(context.Background(), authorized, nil)
	require.NoError(t, err)
	require.Len(t, payload.Targets, 2)
	require.Equal(t, "Workbuddy display", payload.Targets["workbuddy"].DisplayName)
	require.False(t, payload.Targets["workbuddy"].Enabled)
}

func TestDesktopUnifiedModelsKeepClientFieldsWithoutUsableModels(t *testing.T) {
	authorized := activeDesktopAuthorization()
	version := "1.2.3"
	authorized.Organization.TargetConfig = &DesktopTargetConfig{
		SchemaVersion: 2, Models: []DesktopModel{{Model: "model-one", Visible: true, Enabled: false}},
		Targets: DesktopTargets{Workbuddy: &DesktopTarget{Enabled: false, ProviderID: "provider", DisplayName: "Workbuddy name", WireAPI: DesktopWireAPIChatCompletions, MinimumAppVersion: &version, RestartRequired: true}},
	}
	svc := (&DesktopService{}).WithModelCatalog(desktopCatalogStub{ids: []string{"model-one"}})
	payload, _, err := svc.ModelConfiguration(context.Background(), authorized, nil)
	require.NoError(t, err)
	require.Len(t, payload.Targets, 2)
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	targets, ok := decoded["targets"].(map[string]any)
	require.True(t, ok)
	work, ok := targets["workbuddy"].(map[string]any)
	require.True(t, ok)
	require.Len(t, work, 7)
	require.Equal(t, "Workbuddy name", work["display_name"])
	require.Equal(t, "", work["requested_model"])
	require.Equal(t, false, work["enabled"])
	require.Equal(t, "1.2.3", work["minimum_app_version"])
	require.Equal(t, true, work["restart_required"])
}

func TestDesktopUnifiedModelsKeepOmittedClientSettings(t *testing.T) {
	org := activeDesktopAuthorization().Organization
	repo := &desktopRepositoryStub{organization: org}
	svc := (&DesktopService{repo: repo}).WithModelCatalog(desktopCatalogStub{ids: []string{"model-one"}})
	models := []DesktopModel{{Model: "model-one", Visible: true, Enabled: true}}
	work := &DesktopTarget{Enabled: false, ProviderID: "work-provider", DisplayName: "Workbuddy display", WireAPI: DesktopWireAPIChatCompletions}
	_, err := svc.UpdateTargetConfig(context.Background(), org.PublicID, &DesktopTargetConfig{SchemaVersion: 2, Models: models, Targets: DesktopTargets{Workbuddy: work}})
	require.NoError(t, err)
	require.Equal(t, "Model", repo.organization.TargetConfig.Targets.ChatGPTCodex.DisplayName)
	updated, err := svc.UpdateTargetConfig(context.Background(), org.PublicID, &DesktopTargetConfig{SchemaVersion: 2, Models: models})
	require.NoError(t, err)
	require.Equal(t, "Workbuddy display", updated.TargetConfig.Targets.Workbuddy.DisplayName)
	require.False(t, updated.TargetConfig.Targets.Workbuddy.Enabled)
}

func TestDesktopUnifiedClientSettingsValidation(t *testing.T) {
	for _, target := range []*DesktopTarget{
		{ProviderID: "provider", DisplayName: "", WireAPI: DesktopWireAPIChatCompletions},
		{ProviderID: "", DisplayName: "Workbuddy", WireAPI: DesktopWireAPIChatCompletions},
		{ProviderID: "provider", DisplayName: "Workbuddy", WireAPI: DesktopWireAPIResponses},
	} {
		cfg := &DesktopTargetConfig{SchemaVersion: 2, Models: []DesktopModel{}, Targets: DesktopTargets{Workbuddy: target}}
		require.ErrorIs(t, cfg.Validate(), ErrDesktopValidation)
	}
}

func TestDesktopClientSettingsChangeConfigurationVersion(t *testing.T) {
	for _, change := range []struct {
		name  string
		apply func(*DesktopTarget)
	}{
		{"display_name", func(target *DesktopTarget) { target.DisplayName = "Updated" }},
		{"provider_id", func(target *DesktopTarget) { target.ProviderID = "updated" }},
		{"enabled", func(target *DesktopTarget) { target.Enabled = false }},
		{"minimum_app_version", func(target *DesktopTarget) { version := "2.3.4"; target.MinimumAppVersion = &version }},
		{"restart_required", func(target *DesktopTarget) { target.RestartRequired = true }},
	} {
		t.Run(change.name, func(t *testing.T) {
			authorized := activeDesktopAuthorization()
			svc := (&DesktopService{}).WithModelCatalog(desktopCatalogStub{ids: []string{"model-one"}})
			_, before, err := svc.ModelConfiguration(context.Background(), authorized, nil)
			require.NoError(t, err)
			change.apply(authorized.Organization.TargetConfig.Targets.ChatGPTCodex)
			_, after, err := svc.ModelConfiguration(context.Background(), authorized, nil)
			require.NoError(t, err)
			require.NotEqual(t, before, after)
		})
	}
}
