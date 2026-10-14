package service

import (
	"regexp"
	"slices"
	"strings"
)

type DesktopSummarySection struct {
	Kind        string   `json:"kind"`
	Title       string   `json:"title"`
	StartLine   int      `json:"start_line"`
	EndLine     int      `json:"end_line"`
	SourceDates []string `json:"source_dates"`
}
type DesktopSummaryOutline struct {
	Sections        []DesktopSummarySection `json:"sections"`
	ValidatedDates  []string                `json:"validated_source_dates"`
	UnverifiedDates []string                `json:"unverified_source_dates"`
}

var summaryDateReference = regexp.MustCompile(`\[日总结:(\d{4}-\d{2}-\d{2})\]`)

// Annotate stored text without rewriting history or trusting model-produced references.
func BuildDesktopSummaryOutline(content string, allowedDates []string) DesktopSummaryOutline {
	result := DesktopSummaryOutline{Sections: []DesktopSummarySection{}, ValidatedDates: []string{}, UnverifiedDates: []string{}}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	section := DesktopSummarySection{Kind: "summary", Title: "总结", SourceDates: []string{}}
	appendSection := func(end int) {
		section.EndLine = end
		if section.StartLine < end {
			result.Sections = append(result.Sections, section)
		}
	}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			title := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			if title != "" {
				appendSection(i)
				section = DesktopSummarySection{Kind: summarySectionKind(title), Title: title, StartLine: i, SourceDates: []string{}}
			}
		}
		for _, match := range summaryDateReference.FindAllStringSubmatch(line, -1) {
			date := match[1]
			if slices.Contains(allowedDates, date) {
				if !slices.Contains(section.SourceDates, date) {
					section.SourceDates = append(section.SourceDates, date)
				}
				if !slices.Contains(result.ValidatedDates, date) {
					result.ValidatedDates = append(result.ValidatedDates, date)
				}
			} else if !slices.Contains(result.UnverifiedDates, date) {
				result.UnverifiedDates = append(result.UnverifiedDates, date)
			}
		}
	}
	appendSection(len(lines))
	return result
}
func summarySectionKind(title string) string {
	for _, entry := range []struct{ word, kind string }{{"建议", "suggestions"}, {"协调", "coordination"}, {"问题", "issues"}, {"产出", "outcomes"}, {"成果", "outcomes"}, {"进展", "progress"}} {
		if strings.Contains(title, entry.word) {
			return entry.kind
		}
	}
	return "summary"
}
