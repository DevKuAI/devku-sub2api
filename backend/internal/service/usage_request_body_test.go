package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCaptureUsageRequestBodyFollowsRiskControlSwitch(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled:      "true",
		SettingKeyContentModerationConfig: runtimeCacheTestConfig(t),
	}}
	svc := runtimeCacheTestService(repo, time.Hour)
	body := []byte(`{"messages":[{"role":"user","content":"hello"}],"api_key":"secret"}`)

	captured := svc.CaptureUsageRequestBody(context.Background(), body, "application/json")
	require.NotNil(t, captured)
	require.JSONEq(t, `{"messages":[{"role":"user","content":"hello"}],"api_key":"***"}`, *captured)

	require.NoError(t, repo.Set(context.Background(), SettingKeyRiskControlEnabled, "false"))
	require.Nil(t, svc.CaptureUsageRequestBody(context.Background(), body, "application/json"))
}

func TestCaptureUsageRequestBodyFailsClosedWhenSettingsUnavailable(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{
		values:      map[string]string{},
		getValueErr: context.DeadlineExceeded,
	}
	svc := runtimeCacheTestService(repo, time.Hour)

	require.Nil(t, svc.CaptureUsageRequestBody(
		context.Background(),
		[]byte(`{"input":"do not retain"}`),
		"application/json",
	))
}

func TestCaptureUsageRequestBodyDoesNotReuseStaleEnabledSnapshot(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled:      "true",
		SettingKeyContentModerationConfig: runtimeCacheTestConfig(t),
	}}
	svc := runtimeCacheTestService(repo, time.Hour)
	body := []byte(`{"input":"do not retain after settings failure"}`)

	require.NotNil(t, svc.CaptureUsageRequestBody(context.Background(), body, "application/json"))
	repo.failValue(context.DeadlineExceeded)
	require.Nil(t, svc.CaptureUsageRequestBody(context.Background(), body, "application/json"))
}

func TestCaptureUsageRequestBodySkipsEmptyBody(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled:      "true",
		SettingKeyContentModerationConfig: runtimeCacheTestConfig(t),
	}}
	svc := runtimeCacheTestService(repo, time.Hour)

	require.Nil(t, svc.CaptureUsageRequestBody(context.Background(), nil, "application/json"))
}

func TestProvideContentModerationServiceInvalidatesSnapshotAfterSettingsUpdate(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled:      "true",
		SettingKeyContentModerationConfig: runtimeCacheTestConfig(t),
	}}
	settingService := NewSettingService(repo, nil)
	moderationService := ProvideContentModerationService(repo, nil, nil, nil, nil, nil, nil, nil, settingService)
	body := []byte(`{"input":"record while enabled"}`)

	require.NotNil(t, moderationService.CaptureUsageRequestBody(context.Background(), body, "application/json"))
	require.NoError(t, repo.Set(context.Background(), SettingKeyRiskControlEnabled, "false"))
	settingService.notifyRuntimeSettingsListeners()
	require.Nil(t, moderationService.CaptureUsageRequestBody(context.Background(), body, "application/json"))
}

func TestUsageRequestBodyContextUsesIndependentSnapshots(t *testing.T) {
	first := "first"
	parent := WithUsageRequestBodySnapshot(context.Background(), &first)
	child := WithUsageRequestBodySnapshot(context.Background(), UsageRequestBodyFromContext(parent))

	WithUsageRequestBodySnapshot(parent, nil)
	require.Nil(t, UsageRequestBodyFromContext(parent))
	require.Equal(t, "first", *UsageRequestBodyFromContext(child))
}
