//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestDesktopNativeUnifiedModelPoliciesRoundTripAndOrganizationIsolation(t *testing.T) {
	ctx := context.Background()
	one := newDesktopRepositoryFixture(t, "modelsone", 10)
	two := newDesktopRepositoryFixture(t, "modelstwo", 10)
	models := []service.DesktopModel{
		{Model: "visible-enabled", Visible: true, Enabled: true, Remark: "日常开发"},
		{Model: "visible-disabled", Visible: true, Remark: "维护中"},
		{Model: "hidden-enabled", Enabled: true, Remark: "内部配置"},
		{Model: "hidden-disabled"},
	}
	_, err := one.repo.UpdateTargetConfig(ctx, one.organization.PublicID, &service.DesktopTargetConfig{SchemaVersion: 2, Models: models})
	require.NoError(t, err)
	stored, err := one.repo.GetOrganization(ctx, one.organization.PublicID)
	require.NoError(t, err)
	require.Equal(t, 2, stored.TargetConfig.SchemaVersion)
	require.Equal(t, models, stored.TargetConfig.Models)
	other, err := two.repo.GetOrganization(ctx, two.organization.PublicID)
	require.NoError(t, err)
	require.Nil(t, other.TargetConfig)
	_, err = one.repo.ScopedToGatewayUser(two.user.ID).UpdateTargetConfig(ctx, one.organization.PublicID, &service.DesktopTargetConfig{SchemaVersion: 2, Models: []service.DesktopModel{}})
	require.ErrorIs(t, err, service.ErrDesktopOrganizationNotFound)
	stored, err = one.repo.GetOrganization(ctx, one.organization.PublicID)
	require.NoError(t, err)
	require.Equal(t, models, stored.TargetConfig.Models)
	_, err = one.repo.UpdateTargetConfig(ctx, one.organization.PublicID, &service.DesktopTargetConfig{SchemaVersion: 2, Models: []service.DesktopModel{}})
	require.NoError(t, err)
	stored, err = one.repo.GetOrganization(ctx, one.organization.PublicID)
	require.NoError(t, err)
	require.Empty(t, stored.TargetConfig.Models)
}
