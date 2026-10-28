package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

// DesktopModelCatalog resolves the bound group's public model IDs, not its
// unfiltered administrative allowlist candidates.
type DesktopModelCatalog interface {
	Models(context.Context, int64) ([]string, error)
}

type desktopGroupModelCatalog struct {
	groups             GroupRepository
	gateway            *GatewayService
	openai             *OpenAIGatewayService
	maxAccountSwitches int
}

func (r *desktopGroupModelCatalog) Models(ctx context.Context, groupID int64) ([]string, error) {
	group, err := r.groups.GetByID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if group.Platform == PlatformOpenAI && group.CodexModelsManifestConfig.Enabled {
		response, _, err := r.openai.FetchPinnedOpenAIModelsList(ctx, group, r.maxAccountSwitches, "")
		if err != nil {
			return nil, err
		}
		var catalog struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body, &catalog); err != nil {
			return nil, err
		}
		models := make([]string, 0, len(catalog.Data))
		for _, model := range catalog.Data {
			models = append(models, model.ID)
		}
		return models, nil
	}
	var models []string
	if group.Platform == PlatformComposite {
		models, err = r.compositeModels(ctx, group.ID)
	} else {
		models, err = r.gateway.getAvailableModels(ctx, &group.ID, group.Platform)
	}
	if err != nil {
		return nil, err
	}
	if len(models) == 0 && (group.Platform != PlatformOpenAI || models == nil) {
		models = DefaultModelIDsForPlatform(group.Platform)
	} else if group.Platform == PlatformAnthropic && group.ModelAllowlistEnabled() {
		models = append(models, DefaultModelIDsForPlatform(group.Platform)...)
	}
	return group.ModelAllowlist.FilterForListing(models), nil
}

func (r *desktopGroupModelCatalog) compositeModels(ctx context.Context, groupID int64) ([]string, error) {
	accounts, err := r.gateway.accountRepo.ListSchedulableByGroupID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	platforms := make(map[string]bool)
	for _, account := range accounts {
		platforms[account.Platform] = true
	}
	models := make([]string, 0)
	for _, platform := range domain.CompositePrecedencePlatformIDs() {
		ids, err := r.gateway.getAvailableModels(ctx, &groupID, platform)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 && (platform != PlatformOpenAI || ids == nil) && platforms[platform] && !IsMultiProtocolAPIKeyProvider(platform) {
			ids = DefaultModelIDsForPlatform(platform)
		}
		models = append(models, ids...)
	}
	routes, err := r.gateway.GetCompositeRouteModels(ctx, &groupID, "", true)
	if err != nil {
		return nil, err
	}
	return append(models, routes...), nil
}

func (s *DesktopService) WithModelCatalog(catalog DesktopModelCatalog) *DesktopService {
	s.modelCatalog = catalog
	return s
}

// ResolveOrganizationModels creates a response projection without mutating the
// persisted legacy configuration or exposing hidden policies to the enterprise.
func (s *DesktopService) ResolveOrganizationModels(ctx context.Context, organization *DesktopOrganization, enterprise bool) (*DesktopOrganization, error) {
	configuration, err := s.resolveModelConfiguration(ctx, organization, enterprise)
	if err != nil {
		return nil, err
	}
	result := *organization
	result.TargetConfig, result.TargetConfigAssigned = configuration, true
	return &result, nil
}

func (s *DesktopService) resolveModelConfiguration(ctx context.Context, organization *DesktopOrganization, enterprise bool) (*DesktopTargetConfig, error) {
	if s.modelCatalog == nil {
		return nil, fmt.Errorf("desktop model catalog is not configured")
	}
	ids, err := s.modelCatalog.Models(ctx, organization.GroupID)
	if err != nil {
		return nil, err
	}
	return resolveDesktopModels(organization, ids, enterprise), nil
}

func resolveDesktopModels(organization *DesktopOrganization, ids []string, enterprise bool) *DesktopTargetConfig {
	stored := organization.TargetConfig
	policies := make(map[string]DesktopModel)
	if stored != nil && stored.SchemaVersion == 2 {
		for _, model := range stored.Models {
			policies[strings.ToLower(model.Model)] = model
		}
	} else if stored != nil {
		for _, target := range []*DesktopTarget{stored.Targets.ChatGPTCodex, stored.Targets.Workbuddy} {
			if target == nil || !target.Enabled {
				continue
			}
			for _, id := range strings.Split(target.RequestedModel, ",") {
				id = strings.TrimSpace(id)
				policies[strings.ToLower(id)] = DesktopModel{Model: id, Visible: true, Enabled: true}
			}
		}
	}
	result := &DesktopTargetConfig{SchemaVersion: 2, Models: make([]DesktopModel, 0)}
	seen := make(map[string]bool)
	usable := make([]string, 0)
	for _, id := range ids {
		id = strings.TrimSpace(id)
		key := strings.ToLower(id)
		if key == "" || seen[key] || strings.Contains(id, "*") {
			continue
		}
		seen[key] = true
		model := policies[key]
		model.Model = id
		model.Visible = model.Visible && model.Enabled
		if model.Visible && model.Enabled {
			usable = append(usable, id)
		}
		if !enterprise || model.Visible {
			result.Models = append(result.Models, model)
		}
	}
	legacy := DesktopTargets{}
	if stored != nil {
		legacy = stored.Targets
	}
	result.Targets.ChatGPTCodex = desktopLegacyTarget(organization, legacy.ChatGPTCodex, usable, DesktopWireAPIResponses)
	result.Targets.Workbuddy = desktopLegacyTarget(organization, legacy.Workbuddy, usable, DesktopWireAPIChatCompletions)
	return result
}

func desktopLegacyTarget(organization *DesktopOrganization, previous *DesktopTarget, models []string, wireAPI string) *DesktopTarget {
	result := DesktopTarget{Enabled: true, ProviderID: organization.Code, DisplayName: organization.Name}
	if previous != nil {
		result = *previous
	}
	if strings.TrimSpace(result.ProviderID) == "" {
		result.ProviderID = organization.Code
	}
	if strings.TrimSpace(result.DisplayName) == "" {
		result.DisplayName = organization.Name
	}
	result.ProviderID = strings.TrimSpace(result.ProviderID)
	result.DisplayName = strings.TrimSpace(result.DisplayName)
	result.RequestedModel, result.WireAPI = strings.Join(models, ","), wireAPI
	return &result
}
