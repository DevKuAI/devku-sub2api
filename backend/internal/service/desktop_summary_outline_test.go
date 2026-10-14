package service

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDesktopSummaryOutlineValidatesFrozenSourcesAndSeparatesSuggestions(t *testing.T) {
	content := "## 事实进展\n完成联调 [日总结:2026-10-12]\n## 建议\n建议补验收 [日总结:2026-10-13]\n未经核实 [日总结:2026-09-01]"
	outline := BuildDesktopSummaryOutline(content, []string{"2026-10-12", "2026-10-13"})
	require.Len(t, outline.Sections, 2)
	require.Equal(t, "progress", outline.Sections[0].Kind)
	require.Equal(t, "suggestions", outline.Sections[1].Kind)
	require.Equal(t, []string{"2026-10-12", "2026-10-13"}, outline.ValidatedDates)
	require.Equal(t, []string{"2026-09-01"}, outline.UnverifiedDates)
	require.Equal(t, []string{"2026-10-13"}, outline.Sections[1].SourceDates)
	require.Equal(t, 5, outline.Sections[1].EndLine)
	require.Len(t, BuildDesktopSummaryOutline("历史纯文本", nil).Sections, 1)
}
