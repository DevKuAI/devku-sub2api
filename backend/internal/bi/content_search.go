package bi

import (
	"errors"
	"sort"
	"strings"
	"time"
)

func snippet(value string) string {
	runes := []rune(value)
	if len(runes) > 240 {
		return string(runes[:240]) + "…"
	}
	return value
}

func (f *analysisFrame) searchContent(query, kind string) ([]SearchHit, error) {
	result := []SearchHit{}
	query = strings.ToLower(query)
	for _, entityKind := range []string{"knowledge", "case", "application"} {
		if kind != "all" && kind != entityKind {
			continue
		}
		for id, r := range f.records[entityKind] {
			title, summary, updated := r.str("title"), r.str("summary"), r.str("updated_at")
			var status *string
			switch entityKind {
			case "knowledge":
				if _, err := f.readableKnowledge(id); errors.Is(err, ErrNotFound) {
					continue
				} else if err != nil {
					return nil, err
				}
				status = ptr(r.str("status"))
			case "case":
				if _, err := f.readableCase(id); errors.Is(err, ErrNotFound) {
					continue
				} else if err != nil {
					return nil, err
				}
				summary = r.str("_snippet")
				updated = r.str("occurred_at")
			case "application":
				allowed, err := f.visible(entityKind, id)
				if err != nil {
					return nil, err
				}
				if !allowed || !f.relevantOption(entityKind, id) {
					continue
				}
				title = r.str("name")
				updated = f.state.Context.ContentAsOf.Format(time.RFC3339Nano)
			}
			if query != "" && !strings.Contains(strings.ToLower(title+" "+summary), query) {
				continue
			}
			result = append(result, SearchHit{Target: Target{Kind: entityKind, ID: id}, Title: title, Snippet: snippet(summary), UpdatedAt: updated, KnowledgeStatus: status})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Target.Kind != result[j].Target.Kind {
			return result[i].Target.Kind < result[j].Target.Kind
		}
		return result[i].Target.ID < result[j].Target.ID
	})
	return result, nil
}
