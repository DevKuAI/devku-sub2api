package service

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
	"golang.org/x/net/html"
)

type usageRequestBodyContextKey struct{}

type usageRequestBodyHolder struct {
	mu   sync.RWMutex
	body *string
}

// CaptureUsageRequestBody stores the full, redacted user input as plain text
// while risk control is enabled. The gateway already bounds the incoming request size;
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
	snapshot := redactAuditString(content, 0)
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
		var last gjson.Result
		input.ForEach(func(_, item gjson.Result) bool {
			typ := item.Get("type").String()
			role := item.Get("role").String()
			if role == "" && typ == "compaction_trigger" {
				return true
			}
			last = item
			flat = flat && role == "" && (typ == "input_text" || typ == "input_image" || typ == "input_file")
			return true
		})
		if !flat {
			input = last
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
		if text := value.String(); strings.TrimSpace(text) != "" && !isUsageCompactionSummary(stripUsageClientPromptBlocks(text)) {
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
	// Only recognized client context is removed; ordinary user prose is
	// not classified by keywords, and its whitespace is kept when no block is removed.
	original := content
	content = stripUsageClientPromptBlocks(content)
	if isUsageCompactionSummary(content) {
		return ""
	}
	if query, ok := unwrapUsageUserQuery(content); ok {
		return query
	}
	if content != original {
		content = strings.TrimSpace(content)
	}
	return content
}

// Compaction handoffs can arrive as user messages. Match the complete client
// preamble so ordinary requests about summaries are retained.
func isUsageCompactionSummary(content string) bool {
	const preamble = "Another language model started to solve this problem and produced a summary of its thinking process. " +
		"You also have access to the state of the tools that were used by that language model. " +
		"Use this to build on the work that has already been done and avoid duplicating work. " +
		"Here is the summary produced by the other language model, use the information in this summary to assist with your own analysis:"
	return strings.HasPrefix(strings.TrimSpace(content), preamble)
}

func stripUsageClientPromptBlocks(content string) string {
	wrappers := [...]struct{ tag, heading string }{
		{"system-reminder", ""},
		{"environment_context", ""},
		{"environment_details", ""},
		{"user_instructions", ""},
		{"in-app-browser-context", ""},
		{"INSTRUCTIONS", "# AGENTS.md instructions"},
	}
	for _, wrapper := range wrappers {
		content = stripUsagePromptBlock(content, wrapper.tag, wrapper.heading)
	}
	return content
}

func stripUsagePromptBlock(content, tag, heading string) string {
	opening := "<" + tag
	if !strings.Contains(content, opening) {
		return content
	}
	codeRanges := usagePromptCodeRanges(content)
	var out strings.Builder
	cursor, search := 0, 0
	for search < len(content) {
		start := strings.Index(content[search:], opening)
		if start < 0 {
			break
		}
		start += search
		search = start + len(opening)
		if usagePromptCodeRangeAt(codeRanges, start) != nil {
			continue
		}
		lineStart := start
		for lineStart > 0 && (content[lineStart-1] == ' ' || content[lineStart-1] == '\t' || content[lineStart-1] == '\r') {
			lineStart--
		}
		if lineStart > 0 && content[lineStart-1] != '\n' {
			continue
		}
		blockStart := start
		if heading != "" {
			prefix := strings.TrimRight(content[:lineStart], " \t\r\n")
			headingStart := strings.LastIndexByte(prefix, '\n') + 1
			if prefix[headingStart:] != heading || usagePromptCodeRangeAt(codeRanges, headingStart) != nil {
				continue
			}
			blockStart = headingStart
		}
		end := usagePromptBlockEnd(content, start, tag, codeRanges)
		if end < 0 {
			continue
		}
		_, _ = out.WriteString(content[cursor:blockStart])
		cursor = end
		search = cursor
	}
	if cursor == 0 {
		return content
	}
	_, _ = out.WriteString(content[cursor:])
	return out.String()
}

func usagePromptBlockEnd(content string, start int, tag string, codeRanges []usagePromptRange) int {
	tokenizer := html.NewTokenizer(strings.NewReader(content[start:]))
	tokenType := tokenizer.Next()
	if tokenType != html.StartTagToken && tokenType != html.SelfClosingTagToken {
		return -1
	}
	name, _ := tokenizer.TagName()
	if !strings.EqualFold(string(name), tag) {
		return -1
	}
	offset := start + len(tokenizer.Raw())
	if tokenType == html.SelfClosingTagToken {
		return offset
	}
	for depth := 1; depth > 0; {
		tokenType = tokenizer.Next()
		tokenStart := offset
		offset += len(tokenizer.Raw())
		if tokenType == html.ErrorToken {
			// An unclosed client wrapper must not expose its remaining context.
			return len(content)
		}
		if tokenType != html.StartTagToken && tokenType != html.EndTagToken {
			continue
		}
		// Indented tags still delimit a recognized wrapper; fenced examples do not.
		if codeRange := usagePromptCodeRangeAt(codeRanges, tokenStart); codeRange != nil && codeRange.fenced {
			continue
		}
		name, _ = tokenizer.TagName()
		if strings.EqualFold(string(name), tag) {
			if tokenType == html.StartTagToken {
				depth++
			} else {
				depth--
			}
		}
	}
	return offset
}

type usagePromptRange struct {
	start, end int
	fenced     bool
}

// Keep fenced and indented code as literal user text, including unfinished fences.
func usagePromptCodeRanges(content string) []usagePromptRange {
	var ranges []usagePromptRange
	var fence byte
	fenceLength, fenceStart := 0, 0
	for start := 0; start < len(content); {
		end := len(content)
		if newline := strings.IndexByte(content[start:], '\n'); newline >= 0 {
			end = start + newline + 1
		}
		line := strings.TrimRight(content[start:end], "\r\n")
		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)
		marker, length := byte(0), 0
		if indent <= 3 && len(trimmed) > 0 && (trimmed[0] == '`' || trimmed[0] == '~') {
			marker = trimmed[0]
			for length < len(trimmed) && trimmed[length] == marker {
				length++
			}
		}
		switch {
		case fence != 0:
			if marker == fence && length >= fenceLength && strings.TrimSpace(trimmed[length:]) == "" {
				ranges = append(ranges, usagePromptRange{start: fenceStart, end: end, fenced: true})
				fence = 0
			}
		case length >= 3 && (marker == '~' || !strings.Contains(trimmed[length:], "`")):
			fence, fenceLength, fenceStart = marker, length, start
		case indent >= 4 || strings.HasPrefix(trimmed, "\t"):
			ranges = append(ranges, usagePromptRange{start: start, end: end})
		}
		start = end
	}
	if fence != 0 {
		ranges = append(ranges, usagePromptRange{start: fenceStart, end: len(content), fenced: true})
	}
	return ranges
}

func usagePromptCodeRangeAt(ranges []usagePromptRange, position int) *usagePromptRange {
	index := sort.Search(len(ranges), func(i int) bool { return ranges[i].end > position })
	if index < len(ranges) && ranges[index].start <= position {
		return &ranges[index]
	}
	return nil
}

func lastUsagePromptMarker(content, marker string, end int, codeRanges []usagePromptRange) int {
	for end > 0 {
		index := strings.LastIndex(content[:end], marker)
		if index < 0 || usagePromptCodeRangeAt(codeRanges, index) == nil {
			return index
		}
		end = index
	}
	return -1
}

func unwrapUsageUserQuery(content string) (string, bool) {
	const openingTag = "<user_query>"
	const closingTag = "</user_query>"

	if !strings.Contains(content, closingTag) {
		return "", false
	}
	codeRanges := usagePromptCodeRanges(content)
	closingIndex := lastUsagePromptMarker(content, closingTag, len(content), codeRanges)
	if closingIndex < 0 {
		return "", false
	}
	openingIndex := lastUsagePromptMarker(content, openingTag, closingIndex, codeRanges)
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
