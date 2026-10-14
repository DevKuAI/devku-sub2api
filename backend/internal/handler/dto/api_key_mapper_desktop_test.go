package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyFromServiceMasksDesktopManagedCredential(t *testing.T) {
	source := &service.APIKey{
		ID:          7,
		Key:         "sk-desktop-secret",
		Name:        "desktop:org_test:mem_test",
		DisplayName: "Alice",
		ManagedBy:   "desktop",
	}

	result := APIKeyFromService(source)

	require.Equal(t, "***", result.Key)
	require.Equal(t, "desktop:org_test:mem_test", result.Name)
	require.Equal(t, "Alice", result.DisplayName)
	require.Equal(t, "desktop", result.ManagedBy)
}

func TestAPIKeyFromServiceForOwnerExposesDesktopManagedCredential(t *testing.T) {
	source := &service.APIKey{
		ID:          7,
		Key:         "sk-desktop-secret",
		Name:        "desktop:org_test:mem_test",
		DisplayName: "Alice",
		ManagedBy:   "desktop",
		Group: &service.Group{
			Name: "原始分组",
			I18n: service.GroupI18n{"en": {Name: "Desktop group"}},
		},
	}

	result := APIKeyFromServiceForOwner(source).Localized("en")

	require.Equal(t, "sk-desktop-secret", result.Key)
	require.Equal(t, "desktop:org_test:mem_test", result.Name)
	require.Equal(t, "Alice", result.DisplayName)
	require.Equal(t, "desktop", result.ManagedBy)
	require.Equal(t, "Desktop group", result.Group.Name)
	require.Nil(t, result.Group.I18n)
	require.Equal(t, "原始分组", source.Group.Name)
	require.Contains(t, source.Group.I18n, "en")
}

func TestUsageLogMappersMaskDesktopManagedCredential(t *testing.T) {
	source := &service.UsageLog{
		APIKey: &service.APIKey{ID: 7, Key: "sk-desktop-secret", ManagedBy: "desktop"},
	}

	userResult := UsageLogFromService(source)
	adminResult := UsageLogFromServiceAdmin(source)

	require.NotNil(t, userResult.APIKey)
	require.Equal(t, "***", userResult.APIKey.Key)
	require.Equal(t, "desktop", userResult.APIKey.ManagedBy)
	require.NotNil(t, adminResult.APIKey)
	require.Equal(t, "***", adminResult.APIKey.Key)
	require.Equal(t, "desktop", adminResult.APIKey.ManagedBy)
}
