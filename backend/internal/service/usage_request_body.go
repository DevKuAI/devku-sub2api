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

// CaptureUsageRequestBody stores the full, redacted current user text while risk
// control is enabled. The gateway already bounds the incoming request size;
// management audit-body limits must not discard or truncate user messages.
func (s *ContentModerationService) CaptureUsageRequestBody(ctx context.Context, protocol string, body []byte, _ string) *string {
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
	content := extractLatestUsageRequestContent(protocol, body)
	if content == "" {
		return nil
	}
	encoded, err := json.Marshal(map[string]string{"prompt": redactAuditString(content, 0)})
	if err != nil {
		return nil
	}
	snapshot := string(encoded)
	return &snapshot
}

func extractLatestUsageRequestContent(protocol string, body []byte) string {
	if !gjson.ValidBytes(body) {
		return ""
	}
	parts := make([]string, 0, 2)
	switch protocol {
	case ContentModerationProtocolAnthropicMessages, ContentModerationProtocolOpenAIChat:
		last := lastUsageRequestItem(gjson.GetBytes(body, "messages"))
		if last.Get("role").String() == "user" {
			collectUsageRequestText(last.Get("content"), &parts, 0)
		}
	case ContentModerationProtocolOpenAIResponses:
		if eventType := gjson.GetBytes(body, "type").String(); eventType != "" && eventType != "response.create" {
			return ""
		}
		input := gjson.GetBytes(body, "input")
		if !input.Exists() {
			input = gjson.GetBytes(body, "response.input")
		}
		collectUsageResponsesText(input, &parts)
	case ContentModerationProtocolGemini:
		last := lastUsageRequestItem(gjson.GetBytes(body, "contents"))
		if role := last.Get("role").String(); role == "" || role == "user" {
			if content := last.Get("parts"); content.IsArray() {
				content.ForEach(func(_, part gjson.Result) bool {
					if !part.Get("thought").Bool() {
						collectUsageRequestText(part.Get("text"), &parts, 0)
					}
					return true
				})
			}
		}
	case ContentModerationProtocolOpenAIImages:
		collectUsageRequestText(gjson.GetBytes(body, "prompt"), &parts, 0)
	default:
		return ""
	}
	return cleanUsageUserText(strings.Join(parts, "\n\n"))
}

func lastUsageRequestItem(value gjson.Result) gjson.Result {
	var last gjson.Result
	if value.IsArray() {
		value.ForEach(func(_, item gjson.Result) bool {
			last = item
			return true
		})
	}
	return last
}

func collectUsageResponsesText(input gjson.Result, parts *[]string) {
	if input.IsArray() {
		// A flat content array is one input, while a message array may contain history.
		flat := true
		input.ForEach(func(_, item gjson.Result) bool {
			typ := item.Get("type").String()
			flat = item.Get("role").String() == "" && (typ == "input_text" || typ == "input_image")
			return flat
		})
		if !flat {
			input = lastUsageRequestItem(input)
		}
	}
	collectUsageRequestText(input, parts, 0)
}

// Read only text fields. In particular, tool_result.content and base64 images
// must never become user messages or be copied into the retention snapshot.
func collectUsageRequestText(value gjson.Result, parts *[]string, depth int) {
	if depth > 16 {
		return
	}
	switch {
	case value.Type == gjson.String:
		if text := value.String(); strings.TrimSpace(text) != "" {
			*parts = append(*parts, text)
		}
	case value.IsArray():
		value.ForEach(func(_, item gjson.Result) bool {
			collectUsageRequestText(item, parts, depth+1)
			return true
		})
	case value.IsObject():
		if role := value.Get("role").String(); role != "" && role != "user" {
			return
		}
		switch value.Get("type").String() {
		case "", "text", "input_text", "message":
			if text := value.Get("text"); text.Type == gjson.String {
				collectUsageRequestText(text, parts, depth+1)
			}
			collectUsageRequestText(value.Get("content"), parts, depth+1)
		}
	}
}

func cleanUsageUserText(content string) string {
	// Only explicit client wrapper blocks are removed; ordinary user prose is
	// not classified by keywords, and its whitespace is kept when no block is removed.
	wrappers := [...]struct{ opening, closing string }{
		{"<system-reminder>", "</system-reminder>"},
		{"<environment_context>", "</environment_context>"},
		{"<environment_details>", "</environment_details>"},
		{"<user_instructions>", "</user_instructions>"},
		{"# AGENTS.md instructions\n<INSTRUCTIONS>", "</INSTRUCTIONS>"},
		{"# AGENTS.md instructions\r\n<INSTRUCTIONS>", "</INSTRUCTIONS>"},
	}
	original := content
	for _, wrapper := range wrappers {
		content = stripUsagePromptBlock(content, wrapper.opening, wrapper.closing)
	}
	if query, ok := unwrapUsageUserQuery(content); ok {
		return query
	}
	if content != original {
		content = strings.TrimSpace(content)
	}
	return content
}

func stripUsagePromptBlock(content, opening, closing string) string {
	var out strings.Builder
	cursor, search := 0, 0
	for search < len(content) {
		start := strings.Index(content[search:], opening)
		if start < 0 {
			break
		}
		start += search
		search = start + len(opening)
		lineStart := start
		for lineStart > 0 && (content[lineStart-1] == ' ' || content[lineStart-1] == '\t' || content[lineStart-1] == '\r') {
			lineStart--
		}
		if lineStart > 0 && content[lineStart-1] != '\n' {
			continue
		}
		_, _ = out.WriteString(content[cursor:start])
		end := strings.Index(content[search:], closing)
		if end < 0 {
			// An unfinished wrapper is not a trustworthy source of user text.
			return out.String()
		}
		cursor = search + end + len(closing)
		search = cursor
	}
	if cursor == 0 {
		return content
	}
	_, _ = out.WriteString(content[cursor:])
	return out.String()
}

func unwrapUsageUserQuery(content string) (string, bool) {
	const openingTag = "<user_query>"
	const closingTag = "</user_query>"

	closingIndex := strings.LastIndex(content, closingTag)
	if closingIndex < 0 {
		return "", false
	}
	openingIndex := strings.LastIndex(content[:closingIndex], openingTag)
	if openingIndex < 0 {
		return "", false
	}
	query := content[openingIndex+len(openingTag) : closingIndex]
	if strings.TrimSpace(query) == "" {
		query = ""
	}
	return query, true
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
