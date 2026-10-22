package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/tiktoken-go/tokenizer"
)

const desktopReportInputTokens = 6000

const desktopReportSourceBatchSize = 4

// Keep decoded source bodies bounded while preparing a member's durable inputs.
func (s *DesktopReportService) memberInputs(ctx context.Context, org int64, snapshot *DesktopReportSnapshot, model string) ([]string, error) {
	reader, ok := s.repo.(DesktopReportSnapshotReader)
	if !ok || len(snapshot.Sources) > 0 {
		return reportSourceInputs(snapshot.Sources, model), nil
	}
	ids := snapshot.Member.SourceIDs
	if len(ids) != snapshot.Member.RecordCount {
		return nil, NewDesktopReportUnavailable("sources_unavailable")
	}
	builder := newReportSourceInputBuilder(model)
	for offset := 0; offset < len(ids); offset += desktopReportSourceBatchSize {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		batch := ids[offset:min(offset+desktopReportSourceBatchSize, len(ids))]
		sources, err := reader.SnapshotSources(ctx, org, batch)
		if err != nil {
			return nil, err
		}
		if len(sources) != len(batch) {
			return nil, NewDesktopReportUnavailable("sources_unavailable")
		}
		builder.addSources(sources)
	}
	return builder.finish(), nil
}

// Complete records share a batch; oversized segments repeat their source metadata.
func reportSourceInputs(sources []DesktopReportSource, model string) []string {
	builder := newReportSourceInputBuilder(model)
	builder.addSources(sources)
	return builder.finish()
}

type reportSourceInputBuilder struct {
	codec  tokenizer.Codec
	inputs []string
	batch  string
}

func newReportSourceInputBuilder(model string) *reportSourceInputBuilder {
	codec, _ := openAIInputTokensCodecForModel(model)
	return &reportSourceInputBuilder{codec: codec, inputs: []string{}}
}

func (b *reportSourceInputBuilder) addSources(sources []DesktopReportSource) {
	codec := b.codec
	for _, source := range sources {
		raw, _ := json.Marshal(source)
		if reportInputFits("["+string(raw)+"]", codec) {
			b.append(string(raw))
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
				b.append(encode(n))
				text = text[n:]
			}
		}
	}
}

func (b *reportSourceInputBuilder) append(part string) {
	next := part
	if b.batch != "" {
		next = b.batch + "," + part
	}
	if b.batch != "" && !reportInputFits("["+next+"]", b.codec) {
		b.inputs = append(b.inputs, "["+b.batch+"]")
		b.batch = part
	} else {
		b.batch = next
	}
}

func (b *reportSourceInputBuilder) finish() []string {
	if b.batch != "" {
		b.inputs = append(b.inputs, "["+b.batch+"]")
		b.batch = ""
	}
	return b.inputs
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
