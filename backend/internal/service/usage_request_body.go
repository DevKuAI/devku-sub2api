package service

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
)

type usageRequestBodyContextKey struct{}

type usageRequestBodyHolder struct {
	mu   sync.RWMutex
	body *string
}

// CaptureUsageRequestBody stores only the latest user text for known AI
// protocols while risk control is enabled. Runtime-setting failures fail closed.
func (s *ContentModerationService) CaptureUsageRequestBody(ctx context.Context, protocol string, body []byte, contentType string) *string {
	if s == nil || s.settingRepo == nil || len(body) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	enabled, err := s.settingRepo.GetValue(ctx, SettingKeyRiskControlEnabled)
	if err != nil || enabled != "true" {
		return nil
	}
	snapshotBody := body
	if len(body) <= AuditRequestBodyCaptureLimit {
		var captured bool
		snapshotBody, captured = compactUsageRequestBody(protocol, body)
		if !captured {
			return nil
		}
	}
	redacted := RedactAuditBody(snapshotBody, contentType)
	if redacted == "" {
		return nil
	}
	return &redacted
}

func compactUsageRequestBody(protocol string, body []byte) ([]byte, bool) {
	content := extractLatestUsageRequestContent(protocol, body)
	if content == "" {
		if isKnownUsagePromptProtocol(protocol) && gjson.ValidBytes(body) {
			return nil, false
		}
		return body, true
	}
	compact, err := json.Marshal(map[string]string{"prompt": content})
	if err != nil {
		return body, true
	}
	return compact, true
}

func isKnownUsagePromptProtocol(protocol string) bool {
	switch protocol {
	case ContentModerationProtocolAnthropicMessages,
		ContentModerationProtocolOpenAIChat,
		ContentModerationProtocolOpenAIResponses,
		ContentModerationProtocolGemini,
		ContentModerationProtocolOpenAIImages:
		return true
	default:
		return false
	}
}

func extractLatestUsageRequestContent(protocol string, body []byte) string {
	if !gjson.ValidBytes(body) {
		return ""
	}
	parts := make([]string, 0, 2)
	images := make([]string, 0)
	switch protocol {
	case ContentModerationProtocolAnthropicMessages:
		collectLastAnthropicUserMessage(gjson.GetBytes(body, "messages"), &parts, &images)
	case ContentModerationProtocolOpenAIChat:
		collectLastRoleMessage(gjson.GetBytes(body, "messages"), "user", &parts, &images)
	case ContentModerationProtocolOpenAIResponses:
		input := gjson.GetBytes(body, "input")
		if !input.Exists() {
			input = gjson.GetBytes(body, "response.input")
		}
		collectLastResponsesInput(input, &parts, &images)
	case ContentModerationProtocolGemini:
		collectLastGeminiContent(gjson.GetBytes(body, "contents"), &parts, &images)
	case ContentModerationProtocolOpenAIImages:
		addModerationText(&parts, gjson.GetBytes(body, "prompt").String())
	default:
		collectLastResponsesInput(gjson.GetBytes(body, "input"), &parts, &images)
		if len(parts) == 0 {
			collectLastRoleMessage(gjson.GetBytes(body, "messages"), "user", &parts, &images)
		}
		if len(parts) == 0 {
			collectLastGeminiContent(gjson.GetBytes(body, "contents"), &parts, &images)
		}
		if len(parts) == 0 {
			addModerationText(&parts, gjson.GetBytes(body, "prompt").String())
		}
	}
	return unwrapUsageUserQuery(strings.Join(parts, "\n\n"))
}

func unwrapUsageUserQuery(content string) string {
	const openingTag = "<user_query>"
	const closingTag = "</user_query>"

	closingIndex := strings.LastIndex(content, closingTag)
	if closingIndex < 0 {
		return strings.TrimSpace(content)
	}
	openingIndex := strings.LastIndex(content[:closingIndex], openingTag)
	if openingIndex < 0 {
		return strings.TrimSpace(content)
	}
	return strings.TrimSpace(content[openingIndex+len(openingTag) : closingIndex])
}

// WithUsageRequestBodySnapshot stores a mutable holder in ctx. Reusing the
// holder lets WebSocket turns replace their snapshot without retaining a chain
// of prior request bodies.
func WithUsageRequestBodySnapshot(ctx context.Context, body *string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if holder, ok := ctx.Value(usageRequestBodyContextKey{}).(*usageRequestBodyHolder); ok && holder != nil {
		holder.set(body)
		return ctx
	}
	return context.WithValue(ctx, usageRequestBodyContextKey{}, &usageRequestBodyHolder{body: cloneStringPtr(body)})
}

func UsageRequestBodyFromContext(ctx context.Context) *string {
	if ctx == nil {
		return nil
	}
	holder, _ := ctx.Value(usageRequestBodyContextKey{}).(*usageRequestBodyHolder)
	if holder == nil {
		return nil
	}
	return holder.get()
}

func ResolveUsageRequestBody(ctx context.Context, explicit *string) *string {
	if explicit != nil {
		return cloneStringPtr(explicit)
	}
	return UsageRequestBodyFromContext(ctx)
}

func (h *usageRequestBodyHolder) set(body *string) {
	h.mu.Lock()
	h.body = cloneStringPtr(body)
	h.mu.Unlock()
}

func (h *usageRequestBodyHolder) get() *string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return cloneStringPtr(h.body)
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
