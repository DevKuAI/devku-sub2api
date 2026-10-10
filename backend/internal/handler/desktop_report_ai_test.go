package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDesktopReportResponseParsing(t *testing.T) {
	text, err := desktopReportResponseText([]byte(`{"status":"completed","output":[{"type":"reasoning","content":[{"type":"output_text","text":"private reasoning"}]},{"type":"message","content":[{"type":"output_text","text":"日报"}]}]}`))
	require.NoError(t, err)
	require.Equal(t, "日报", text)
	text, err = desktopReportResponseText([]byte(`{"choices":[{"message":{"content":"日报"},"finish_reason":"stop"}]}`))
	require.NoError(t, err)
	require.Equal(t, "日报", text)
	for _, body := range []string{`{"choices":[{"message":{"content":"truncated"},"finish_reason":"length"}]}`, `{"status":"incomplete","output_text":"partial"}`, `{"status":"completed","output":[]}`, `invalid`} {
		_, err = desktopReportResponseText([]byte(body))
		require.Error(t, err)
	}
}
