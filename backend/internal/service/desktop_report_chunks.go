package service

import (
	"encoding/json"
	"strings"

	"github.com/tiktoken-go/tokenizer"
)

const desktopReportInputTokens = 6000

// Complete records share a batch; oversized segments repeat their source metadata.
func reportSourceInputs(sources []DesktopReportSource, model string) []string {
	codec, _ := openAIInputTokensCodecForModel(model)
	parts := []string{}
	for _, source := range sources {
		raw, _ := json.Marshal(source)
		if reportInputFits("["+string(raw)+"]", codec) {
			parts = append(parts, string(raw))
			continue
		}
		segments := []struct {
			kind    string
			segment DesktopTextSegment
		}{}
		for _, prompt := range source.Prompts {
			segments = append(segments, struct {
				kind    string
				segment DesktopTextSegment
			}{"prompt", prompt})
		}
		if source.Response != nil {
			segments = append(segments, struct {
				kind    string
				segment DesktopTextSegment
			}{"response", *source.Response})
		}
		for index, segment := range segments {
			text := []rune(segment.segment.Text)
			for part := 1; len(text) > 0; part++ {
				encode := func(n int) string {
					raw, _ := json.Marshal(map[string]any{"record_id": source.RecordID, "client": source.Client, "session_id": source.SessionID, "stopped_at": source.StoppedAt, "capture_status": source.CaptureStatus, "kind": segment.kind, "segment": index + 1, "part": part, "truncated": segment.segment.Truncated, "text": string(text[:n])})
					return string(raw)
				}
				low, high := 1, min(len(text), desktopReportChunkRunes-1024)
				for low < high {
					mid := (low + high + 1) / 2
					if reportInputFits("["+encode(mid)+"]", codec) {
						low = mid
					} else {
						high = mid - 1
					}
				}
				n := low
				if n < len(text) {
					if split := strings.LastIndex(string(text[:n]), "\n"); split > 0 {
						prefix := []rune(string(text[:n])[:split+1])
						if len(prefix) > n/2 {
							n = len(prefix)
						}
					}
				}
				parts = append(parts, encode(n))
				text = text[n:]
			}
		}
	}
	result := []string{}
	batch := ""
	for _, part := range parts {
		next := part
		if batch != "" {
			next = batch + "," + part
		}
		if batch != "" && !reportInputFits("["+next+"]", codec) {
			result = append(result, "["+batch+"]")
			batch = part
		} else {
			batch = next
		}
	}
	if batch != "" {
		result = append(result, "["+batch+"]")
	}
	return result
}

func reportInputFits(input string, codec tokenizer.Codec) bool {
	if len([]rune(input)) > desktopReportChunkRunes {
		return false
	}
	if codec == nil {
		return len([]rune(input)) <= desktopReportInputTokens
	}
	count, err := codec.Count(input)
	return err == nil && count <= desktopReportInputTokens
}

// Paragraph boundaries and member labels are retained across reduction batches.
func reportTextInputs(input, model string) []string {
	codec, _ := openAIInputTokensCodecForModel(model)
	result := []string{}
	batch := ""
	label := ""
	for _, paragraph := range strings.SplitAfter(input, "\n\n") {
		trimmed := strings.TrimSpace(paragraph)
		if strings.HasPrefix(trimmed, "[") {
			if end := strings.Index(trimmed, "]"); end > 0 && end < 100 {
				label = trimmed[:end+1]
			}
		}
		if reportInputFits(batch+paragraph, codec) {
			batch += paragraph
			continue
		}
		if batch != "" {
			result = append(result, batch)
			batch = ""
		}
		if reportInputFits(paragraph, codec) {
			batch = paragraph
			continue
		}
		text := []rune(paragraph)
		for len(text) > 0 {
			prefix := ""
			if label != "" {
				prefix = label + " (continued)\n"
			}
			low, high := 1, min(len(text), desktopReportChunkRunes-len([]rune(prefix)))
			for low < high {
				mid := (low + high + 1) / 2
				if reportInputFits(prefix+string(text[:mid]), codec) {
					low = mid
				} else {
					high = mid - 1
				}
			}
			n := low
			if n < len(text) {
				if split := strings.LastIndex(string(text[:n]), "\n"); split > 0 {
					head := []rune(string(text[:n])[:split+1])
					if len(head) > n/2 {
						n = len(head)
					}
				}
			}
			result = append(result, prefix+string(text[:n]))
			text = text[n:]
		}
	}
	if batch != "" {
		result = append(result, batch)
	}
	return result
}
