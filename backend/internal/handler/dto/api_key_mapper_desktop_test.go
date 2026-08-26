package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyFromServiceMasksDesktopManagedCredential(t *testing.T) {
	source := &service.APIKey{ID: 7, Key: "sk-desktop-secret", ManagedBy: "desktop"}

	result := APIKeyFromService(source)

	require.Equal(t, "***", result.Key)
	require.Equal(t, "desktop", result.ManagedBy)
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
