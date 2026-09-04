package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCaptureUsageRequestBodyStoresOnlyLatestTaggedUserInput(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled:      "true",
		SettingKeyContentModerationConfig: runtimeCacheTestConfig(t),
	}}
	svc := runtimeCacheTestService(repo, time.Hour)
	wrappedPrompt := "This conversation is powered by SUBS\n" +
		strings.Repeat("system context that must not be stored\n", 600) +
		"<user_query>1</user_query>"
	body, err := json.Marshal(map[string]any{
		"messages": []map[string]any{
			{"role": "system", "content": "system instruction"},
			{"role": "user", "content": "older input"},
			{"role": "assistant", "content": "older output"},
			{"role": "user", "content": wrappedPrompt},
		},
	})
	require.NoError(t, err)

	captured := svc.CaptureUsageRequestBody(context.Background(), ContentModerationProtocolOpenAIChat, body, "application/json")
	require.NotNil(t, captured)
	require.Equal(t, `{"prompt":"1"}`, *captured)
	require.NotContains(t, *captured, "system context")
	require.NotContains(t, *captured, "older input")
}

func TestCaptureUsageRequestBodyFollowsRiskControlSwitch(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled:      "true",
		SettingKeyContentModerationConfig: runtimeCacheTestConfig(t),
	}}
	svc := runtimeCacheTestService(repo, time.Hour)
	body := []byte(`{"messages":[{"role":"user","content":"hello"}],"api_key":"secret"}`)

	captured := svc.CaptureUsageRequestBody(context.Background(), ContentModerationProtocolOpenAIChat, body, "application/json")
	require.NotNil(t, captured)
	require.JSONEq(t, `{"prompt":"hello"}`, *captured)

	require.NoError(t, repo.Set(context.Background(), SettingKeyRiskControlEnabled, "false"))
	require.Nil(t, svc.CaptureUsageRequestBody(context.Background(), ContentModerationProtocolOpenAIChat, body, "application/json"))
}

func TestCaptureUsageRequestBodyExtractsCommonUserInputShapes(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled:      "true",
		SettingKeyContentModerationConfig: runtimeCacheTestConfig(t),
	}}
	svc := runtimeCacheTestService(repo, time.Hour)
	tests := []struct {
		name     string
		protocol string
		body     string
		want     string
	}{
		{
			name:     "chat latest user message",
			protocol: ContentModerationProtocolOpenAIChat,
			body:     `{"messages":[{"role":"system","content":"policy"},{"role":"user","content":"old"},{"role":"assistant","content":"answer"},{"role":"user","content":[{"type":"text","text":"latest first"},{"type":"image_url","image_url":{"url":"data:image/png;base64,secret"}},{"type":"text","text":"latest second"}]}]}`,
			want:     "latest first\n\nlatest second",
		},
		{
			name:     "responses input",
			protocol: ContentModerationProtocolOpenAIResponses,
			body:     `{"instructions":"policy","input":[{"role":"assistant","content":"answer"},{"role":"user","content":[{"type":"input_text","text":"response input"}]}]}`,
			want:     "response input",
		},
		{
			name:     "gemini contents",
			protocol: ContentModerationProtocolGemini,
			body:     `{"systemInstruction":{"parts":[{"text":"policy"}]},"contents":[{"role":"model","parts":[{"text":"answer"}]},{"role":"user","parts":[{"text":"gemini input"}]}]}`,
			want:     "gemini input",
		},
		{
			name:     "websocket response input",
			protocol: ContentModerationProtocolOpenAIResponses,
			body:     `{"type":"response.create","response":{"input":"turn input"}}`,
			want:     "turn input",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			captured := svc.CaptureUsageRequestBody(context.Background(), tt.protocol, []byte(tt.body), "application/json")
			require.NotNil(t, captured)
			var result map[string]string
			require.NoError(t, json.Unmarshal([]byte(*captured), &result))
			require.Equal(t, tt.want, result["prompt"])
			require.NotContains(t, *captured, "policy")
			require.NotContains(t, *captured, "answer")
		})
	}
}

func TestCaptureUsageRequestBodyRedactsSecretsInExtractedPrompt(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled:      "true",
		SettingKeyContentModerationConfig: runtimeCacheTestConfig(t),
	}}
	svc := runtimeCacheTestService(repo, time.Hour)
	body := []byte(`{"messages":[{"role":"user","content":"Authorization: Bearer abcdefghijklmnop"}]}`)

	captured := svc.CaptureUsageRequestBody(context.Background(), ContentModerationProtocolOpenAIChat, body, "application/json")
	require.NotNil(t, captured)
	require.Contains(t, *captured, "Authorization: ***")
	require.NotContains(t, *captured, "abcdefghijklmnop")
}

func TestCaptureUsageRequestBodyKeepsOriginalCaptureLimit(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled:      "true",
		SettingKeyContentModerationConfig: runtimeCacheTestConfig(t),
	}}
	svc := runtimeCacheTestService(repo, time.Hour)
	body := []byte(`{"messages":[{"role":"user","content":"` +
		strings.Repeat("a", AuditRequestBodyCaptureLimit) +
		`"}]}`)

	captured := svc.CaptureUsageRequestBody(context.Background(), ContentModerationProtocolOpenAIChat, body, "application/json")
	require.NotNil(t, captured)
	require.Equal(t, "<body omitted: exceeds 262144 bytes>", *captured)
}

func TestCaptureUsageRequestBodySkipsKnownProtocolWithoutCurrentUserText(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled:      "true",
		SettingKeyContentModerationConfig: runtimeCacheTestConfig(t),
	}}
	svc := runtimeCacheTestService(repo, time.Hour)
	body := []byte(`{"messages":[{"role":"user","content":"old input"},{"role":"assistant","content":"tool call"},{"role":"tool","content":"tool output"}]}`)

	require.Nil(t, svc.CaptureUsageRequestBody(
		context.Background(),
		ContentModerationProtocolOpenAIChat,
		body,
		"application/json",
	))
}

func TestCaptureUsageRequestBodyFailsClosedWhenSettingsUnavailable(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{
		values:      map[string]string{},
		getValueErr: context.DeadlineExceeded,
	}
	svc := runtimeCacheTestService(repo, time.Hour)

	require.Nil(t, svc.CaptureUsageRequestBody(
		context.Background(),
		ContentModerationProtocolOpenAIResponses,
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

	require.NotNil(t, svc.CaptureUsageRequestBody(context.Background(), ContentModerationProtocolOpenAIResponses, body, "application/json"))
	repo.failValue(context.DeadlineExceeded)
	require.Nil(t, svc.CaptureUsageRequestBody(context.Background(), ContentModerationProtocolOpenAIResponses, body, "application/json"))
}

func TestCaptureUsageRequestBodySkipsEmptyBody(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled:      "true",
		SettingKeyContentModerationConfig: runtimeCacheTestConfig(t),
	}}
	svc := runtimeCacheTestService(repo, time.Hour)

	require.Nil(t, svc.CaptureUsageRequestBody(context.Background(), ContentModerationProtocolOpenAIResponses, nil, "application/json"))
}

func TestProvideContentModerationServiceInvalidatesSnapshotAfterSettingsUpdate(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled:      "true",
		SettingKeyContentModerationConfig: runtimeCacheTestConfig(t),
	}}
	settingService := NewSettingService(repo, nil)
	moderationService := ProvideContentModerationService(repo, nil, nil, nil, nil, nil, nil, nil, settingService)
	body := []byte(`{"input":"record while enabled"}`)

	require.NotNil(t, moderationService.CaptureUsageRequestBody(context.Background(), ContentModerationProtocolOpenAIResponses, body, "application/json"))
	require.NoError(t, repo.Set(context.Background(), SettingKeyRiskControlEnabled, "false"))
	settingService.notifyRuntimeSettingsListeners()
	require.Nil(t, moderationService.CaptureUsageRequestBody(context.Background(), ContentModerationProtocolOpenAIResponses, body, "application/json"))
}

func TestUsageRequestBodyContextUsesIndependentSnapshots(t *testing.T) {
	first := "first"
	parent := WithUsageRequestBodySnapshot(context.Background(), &first)
	child := WithUsageRequestBodySnapshot(context.Background(), UsageRequestBodyFromContext(parent))

	WithUsageRequestBodySnapshot(parent, nil)
	require.Nil(t, UsageRequestBodyFromContext(parent))
	require.Equal(t, "first", *UsageRequestBodyFromContext(child))
}
