package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const usageCompactionSummaryPreambleFixture = "Another language model started to solve this problem and produced a summary of its thinking process. " +
	"You also have access to the state of the tools that were used by that language model. " +
	"Use this to build on the work that has already been done and avoid duplicating work. " +
	"Here is the summary produced by the other language model, use the information in this summary to assist with your own analysis:"

func TestCaptureUsageRequestBodyStoresOnlyLatestTaggedUserInput(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled:      "true",
		SettingKeyContentModerationConfig: runtimeCacheTestConfig(t),
	}}
	svc := runtimeCacheTestService(repo, time.Hour)
	wrappedPrompt := "This conversation is powered by SUBS\n" +
		strings.Repeat("system context that must not be stored\n", 10000) +
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
	require.Greater(t, len(body), AuditRequestBodyCaptureLimit)

	captured := svc.CaptureUsageRequestBody(context.Background(), ContentModerationProtocolOpenAIChat, body, "application/json")
	require.NotNil(t, captured)
	require.Equal(t, "1", *captured)
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
	require.Equal(t, "hello", *captured)

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
		{
			name:     "responses flat content input",
			protocol: ContentModerationProtocolOpenAIResponses,
			body:     `{"input":[{"type":"input_text","text":"first block"},{"type":"input_image","image_url":"data:image/png;base64,secret"},{"type":"input_text","text":"second block"}]}`,
			want:     "first block\n\nsecond block",
		},
		{
			name:     "image generation prompt",
			protocol: ContentModerationProtocolOpenAIImages,
			body:     `{"prompt":"draw a lighthouse","instructions":"policy","image":"BASE64SECRET"}`,
			want:     "draw a lighthouse",
		},
		{
			name:     "literal inline wrapper names",
			protocol: ContentModerationProtocolOpenAIChat,
			body:     `{"messages":[{"role":"user","content":"Explain the <system-reminder> wrapper and AGENTS.md instructions."}]}`,
			want:     "Explain the <system-reminder> wrapper and AGENTS.md instructions.",
		},
		{
			name:     "long inline wrapper examples",
			protocol: ContentModerationProtocolOpenAIChat,
			body:     `{"messages":[{"role":"user","content":"Explain these tags: ` + strings.Repeat("<system-reminder> ", 20000) + `"}]}`,
			want:     "Explain these tags: " + strings.Repeat("<system-reminder> ", 20000),
		},
		{
			name:     "plain Chinese prompt",
			protocol: ContentModerationProtocolOpenAIResponses,
			body:     `{"input":"如果我没有写纠错内容呢，"}`,
			want:     "如果我没有写纠错内容呢，",
		},
		{
			name:     "paragraphs and indentation",
			protocol: ContentModerationProtocolOpenAIResponses,
			body:     `{"input":"  First paragraph\n\nSecond paragraph\n\tIndented line\n  "}`,
			want:     "  First paragraph\n\nSecond paragraph\n\tIndented line\n  ",
		},
		{
			name:     "CRLF paragraphs",
			protocol: ContentModerationProtocolOpenAIResponses,
			body:     `{"input":"First paragraph\r\n\r\nSecond paragraph"}`,
			want:     "First paragraph\r\n\r\nSecond paragraph",
		},
		{
			name:     "literal newline escapes",
			protocol: ContentModerationProtocolOpenAIResponses,
			body:     `{"input":"Keep \\n\\n as text.\nUse a real line break here."}`,
			want:     "Keep \\n\\n as text.\nUse a real line break here.",
		},
		{
			name:     "user supplied JSON remains text",
			protocol: ContentModerationProtocolOpenAIResponses,
			body:     `{"input":"{\"prompt\":\"line one\\n\\nline two\"}"}`,
			want:     `{"prompt":"line one\n\nline two"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			captured := svc.CaptureUsageRequestBody(context.Background(), tt.protocol, []byte(tt.body), "application/json")
			require.NotNil(t, captured)
			require.Equal(t, tt.want, *captured)
			require.NotContains(t, *captured, "policy")
			require.NotContains(t, *captured, "answer")
		})
	}
}

func TestCaptureUsageRequestBodyFindsLatestUserInputBeforeToolContinuation(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled: "true",
	}}
	svc := runtimeCacheTestService(repo, time.Hour)
	tests := []struct {
		name     string
		protocol string
		body     string
		want     string
	}{
		{
			name:     "workbuddy chat tool result",
			protocol: ContentModerationProtocolOpenAIChat,
			body:     `{"messages":[{"role":"user","content":"workbuddy request"},{"role":"assistant","tool_calls":[{"id":"call_1"}]},{"role":"tool","tool_call_id":"call_1","content":"tool result"}]}`,
			want:     "workbuddy request",
		},
		{
			name:     "anthropic tool result",
			protocol: ContentModerationProtocolAnthropicMessages,
			body:     `{"messages":[{"role":"user","content":"anthropic request"},{"role":"assistant","content":[{"type":"tool_use","id":"call_1"}]},{"role":"tool","content":[{"type":"tool_result","tool_use_id":"call_1","content":"tool result"}]}]}`,
			want:     "anthropic request",
		},
		{
			name:     "gemini model continuation",
			protocol: ContentModerationProtocolGemini,
			body:     `{"contents":[{"role":"user","parts":[{"text":"gemini request"}]},{"role":"model","parts":[{"text":"model continuation"}]}]}`,
			want:     "gemini request",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			captured := svc.CaptureUsageRequestBody(context.Background(), tt.protocol, []byte(tt.body), "application/json")
			require.NotNil(t, captured)
			require.Equal(t, tt.want, *captured)
		})
	}
}

func TestCaptureUsageRequestBodyRedactsSecretsInExtractedPrompt(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled:      "true",
		SettingKeyContentModerationConfig: runtimeCacheTestConfig(t),
	}}
	svc := runtimeCacheTestService(repo, time.Hour)
	body, err := json.Marshal(map[string]any{"messages": []map[string]string{{
		"role":    "user",
		"content": strings.Repeat("plain user text\n", 20000) + "Authorization: Bearer abcdefghijklmnop",
	}}})
	require.NoError(t, err)
	require.Greater(t, len(body), AuditRequestBodyCaptureLimit)

	captured := svc.CaptureUsageRequestBody(context.Background(), ContentModerationProtocolOpenAIChat, body, "application/json")
	require.NotNil(t, captured)
	require.Contains(t, *captured, "Authorization: ***")
	require.NotContains(t, *captured, "abcdefghijklmnop")
}

func TestCaptureUsageRequestBodyPreservesLongUserInput(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled:      "true",
		SettingKeyContentModerationConfig: runtimeCacheTestConfig(t),
	}}
	svc := runtimeCacheTestService(repo, time.Hour)
	for _, size := range []int{auditRequestBodyMaxBytes, AuditRequestBodyCaptureLimit, 1024 * 1024} {
		prompt := "  first line\n\t" + strings.Repeat("用户输入\n", size/8) + "\n  last line  "
		body, err := json.Marshal(map[string]any{
			"messages": []map[string]any{{"role": "user", "content": prompt}},
		})
		require.NoError(t, err)

		captured := svc.CaptureUsageRequestBody(context.Background(), ContentModerationProtocolOpenAIChat, body, "application/json")
		require.NotNil(t, captured)
		require.Equal(t, prompt, *captured)
	}
}

func TestCaptureUsageRequestBodyExcludesBuiltInPromptBlocks(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled:      "true",
		SettingKeyContentModerationConfig: runtimeCacheTestConfig(t),
	}}
	svc := runtimeCacheTestService(repo, time.Hour)
	const prompt = "Please fix the input parser.\nKeep its formatting."
	const environment = "<environment_context><cwd>/workspace</cwd></environment_context>"
	const instructions = "# AGENTS.md instructions\n<INSTRUCTIONS>built-in policy</INSTRUCTIONS>"
	const summary = usageCompactionSummaryPreambleFixture + "\n\n## Active Task\nAutomatically generated context."
	textBlocks := []map[string]any{
		{"type": "text", "text": instructions},
		{"type": "text", "text": environment},
		{"type": "text", "text": summary},
		{"type": "text", "text": prompt},
		{"type": "text", "text": environment + "\n\n" + summary},
		{"type": "text", "text": "<system-reminder>injected reminder</system-reminder>"},
		{"type": "image", "source": map[string]string{"type": "base64", "media_type": "image/png", "data": strings.Repeat("a", AuditRequestBodyCaptureLimit)}},
	}
	tests := []struct {
		name     string
		protocol string
		body     any
	}{
		{
			name:     "anthropic text blocks",
			protocol: ContentModerationProtocolAnthropicMessages,
			body: map[string]any{
				"system": "built-in system policy",
				"messages": []map[string]any{
					{"role": "user", "content": "older user input"},
					{"role": "assistant", "content": "older output"},
					{"role": "user", "content": textBlocks},
				},
			},
		},
		{
			name:     "chat text blocks",
			protocol: ContentModerationProtocolOpenAIChat,
			body: map[string]any{"messages": []map[string]any{
				{"role": "system", "content": "built-in system policy"},
				{"role": "developer", "content": "built-in developer policy"},
				{"role": "user", "content": textBlocks},
			}},
		},
		{
			name:     "responses text blocks",
			protocol: ContentModerationProtocolOpenAIResponses,
			body: map[string]any{
				"instructions": "built-in system policy",
				"input":        []map[string]any{{"role": "user", "content": textBlocks}},
			},
		},
		{
			name:     "websocket text blocks",
			protocol: ContentModerationProtocolOpenAIResponses,
			body: map[string]any{
				"type": "response.create",
				"response": map[string]any{
					"instructions": "built-in system policy",
					"input":        []map[string]any{{"role": "user", "content": textBlocks}},
				},
			},
		},
		{
			name:     "gemini text parts",
			protocol: ContentModerationProtocolGemini,
			body: map[string]any{
				"systemInstruction": map[string]any{"parts": []map[string]string{{"text": "built-in system policy"}}},
				"contents": []map[string]any{{"role": "user", "parts": []map[string]string{
					{"text": instructions}, {"text": environment}, {"text": summary}, {"text": prompt},
				}}},
			},
		},
		{
			name:     "mixed reminder and input",
			protocol: ContentModerationProtocolAnthropicMessages,
			body: map[string]any{"messages": []map[string]any{{
				"role": "user",
				"content": "<system-reminder>injected prefix</system-reminder>\n\n" + prompt +
					"\n\n<system-reminder>injected suffix</system-reminder>",
			}}},
		},
		{
			name:     "tagged input with reminders",
			protocol: ContentModerationProtocolOpenAIChat,
			body: map[string]any{"messages": []map[string]any{{
				"role": "user",
				"content": "<system-reminder>injected prefix</system-reminder>\n<user_query>" +
					prompt + "</user_query>\n" + environment,
			}}},
		},
		{
			name:     "user query example inside built-in reminder",
			protocol: ContentModerationProtocolOpenAIChat,
			body: map[string]any{"messages": []map[string]any{{
				"role":    "user",
				"content": "<system-reminder>Example: <user_query>built-in example</user_query></system-reminder>\n\n" + prompt,
			}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.body)
			require.NoError(t, err)
			original := string(body)
			captured := svc.CaptureUsageRequestBody(context.Background(), tt.protocol, body, "application/json")
			require.NotNil(t, captured)
			require.Equal(t, prompt, *captured)
			require.Equal(t, original, string(body))
		})
	}
}

func TestCaptureUsageRequestBodyExcludesCompactionSummaries(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled: "true",
	}}
	svc := runtimeCacheTestService(repo, time.Hour)
	const summary = usageCompactionSummaryPreambleFixture + "\n\n## Active Task\nAutomatically generated context."
	const prompt = "  Continue with my change.\n\nKeep the formatting.  "
	const quoted = "Explain this automatic summary:\n\n" + summary
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{"summary only", summary, ""},
		{"leading whitespace", "\n\t  " + summary, ""},
		{"summary containing user query example", summary + "\n<user_query>historical input</user_query>", ""},
		{"summary after client wrapper", "<environment_context>workspace</environment_context>\n\n" + summary, ""},
		{"summary after history", []map[string]any{{"role": "user", "content": "older input"}, {"role": "user", "content": summary}}, ""},
		{"current input after summary", []map[string]any{{"role": "user", "content": summary}, {"role": "user", "content": prompt}}, prompt},
		{"mixed flat content blocks", []map[string]any{{"type": "input_text", "text": summary}, {"type": "input_text", "text": prompt}}, prompt},
		{"quoted summary in a question", quoted, quoted},
		{"ordinary request about summaries", "Another language model started to solve this problem. Please review its summary.", "Another language model started to solve this problem. Please review its summary."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(map[string]any{"input": tt.input})
			require.NoError(t, err)
			original := string(body)
			captured := svc.CaptureUsageRequestBody(context.Background(), ContentModerationProtocolOpenAIResponses, body, "application/json")
			if tt.want == "" {
				require.Nil(t, captured)
			} else {
				require.NotNil(t, captured)
				require.Equal(t, tt.want, *captured)
			}
			require.Equal(t, original, string(body))
		})
	}
}

func TestCaptureUsageRequestBodyNeverFallsBackToRawPayload(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled:      "true",
		SettingKeyContentModerationConfig: runtimeCacheTestConfig(t),
	}}
	svc := runtimeCacheTestService(repo, time.Hour)
	tests := []struct {
		name     string
		protocol string
		body     string
	}{
		{"unknown protocol", "unknown", `{"prompt":"unclassified content","system":"built-in policy"}`},
		{"invalid json", ContentModerationProtocolOpenAIChat, `{"messages":`},
		{"non json", ContentModerationProtocolOpenAIChat, "unclassified content"},
		{"system only", ContentModerationProtocolOpenAIChat, `{"messages":[{"role":"system","content":"built-in policy"}]}`},
		{"developer only", ContentModerationProtocolOpenAIResponses, `{"input":[{"role":"developer","content":"built-in policy"}]}`},
		{"reminder only", ContentModerationProtocolAnthropicMessages, `{"messages":[{"role":"user","content":"<system-reminder>built-in policy</system-reminder>"}]}`},
		{"environment only", ContentModerationProtocolOpenAIResponses, `{"input":"<environment_context><cwd>/workspace</cwd></environment_context>"}`},
		{"unfinished reminder", ContentModerationProtocolOpenAIResponses, `{"input":"<system-reminder>built-in policy"}`},
		{"empty user query", ContentModerationProtocolOpenAIResponses, `{"input":"built-in policy\n<user_query>  </user_query>"}`},
		{"websocket control event", ContentModerationProtocolOpenAIResponses, `{"type":"response.cancel","input":"control metadata"}`},
		{"anthropic tool result", ContentModerationProtocolAnthropicMessages, `{"messages":[{"role":"user","content":"old input"},{"role":"user","content":[{"type":"tool_result","content":"tool output"}]}]}`},
		{"responses tool result without user input", ContentModerationProtocolOpenAIResponses, `{"input":[{"type":"function_call_output","output":"tool output"}]}`},
		{"gemini tool result", ContentModerationProtocolGemini, `{"contents":[{"role":"user","parts":[{"text":"old input"}]},{"role":"user","parts":[{"functionResponse":{"response":{"text":"tool output"}}}]}]}`},
		{"image only", ContentModerationProtocolOpenAIChat, `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,aGVsbG8="}}]}]}`},
		{"large built-in payload", ContentModerationProtocolOpenAIChat, `{"messages":[{"role":"system","content":"` + strings.Repeat("a", AuditRequestBodyCaptureLimit) + `"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Nil(t, svc.CaptureUsageRequestBody(context.Background(), tt.protocol, []byte(tt.body), "application/json"))
		})
	}
}

func TestCaptureUsageRequestBodySkipsKnownProtocolWithoutCurrentUserText(t *testing.T) {
	repo := &contentModerationRuntimeSettingRepo{values: map[string]string{
		SettingKeyRiskControlEnabled:      "true",
		SettingKeyContentModerationConfig: runtimeCacheTestConfig(t),
	}}
	svc := runtimeCacheTestService(repo, time.Hour)
	body := []byte(`{"messages":[{"role":"assistant","content":"tool call"},{"role":"tool","content":"tool output"}]}`)

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
