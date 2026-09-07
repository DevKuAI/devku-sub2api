package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageRequestBodyResponsesControlAndAttachments(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"user before trigger", `{"input":[{"type":"message","role":"user","content":"hello"},{"type":"compaction_trigger"}]}`, "hello"},
		{"multiple triggers and history", `{"input":[{"type":"compaction_trigger"},{"role":"user","content":"old"},{"role":"assistant","content":"answer"},{"role":"user","content":"latest"},{"type":"compaction_trigger"},{"type":"compaction_trigger"}]}`, "latest"},
		{"websocket trigger", `{"type":"response.create","response":{"input":[{"role":"user","content":"hello"},{"type":"compaction_trigger"}]}}`, "hello"},
		{"flat text before file", `{"input":[{"type":"input_text","text":"hello"},{"type":"input_file","file_id":"file-example"}]}`, "hello"},
		{"flat text around attachments", `{"input":[{"type":"input_file","file_data":"private file data"},{"type":"input_text","text":"first"},{"type":"input_image","image_url":"private image data"},{"type":"input_file","file_url":"https://example.test/private"},{"type":"input_text","text":"second"},{"type":"compaction_trigger"}]}`, "first\n\nsecond"},
		{"message containing file", `{"input":[{"role":"user","content":[{"type":"input_text","text":"hello"},{"type":"input_file","file_data":"private file data"}]},{"type":"compaction_trigger"}]}`, "hello"},
		{"trigger only", `{"input":[{"type":"compaction_trigger"}]}`, ""},
		{"file and trigger only", `{"input":[{"type":"input_file","file_data":"private file data"},{"type":"compaction_trigger"}]}`, ""},
		{"tool output before trigger", `{"input":[{"role":"user","content":"old"},{"type":"function_call_output","output":"tool output"},{"type":"compaction_trigger"}]}`, ""},
		{"assistant before trigger", `{"input":[{"role":"user","content":"old"},{"role":"assistant","content":"answer"},{"type":"compaction_trigger"}]}`, ""},
		{"file after historical message", `{"input":[{"role":"user","content":"old"},{"type":"input_file","file_id":"file-example"},{"type":"compaction_trigger"}]}`, ""},
		{"unknown last item", `{"input":[{"role":"user","content":"old"},{"type":"unknown_event","text":"internal event"},{"type":"compaction_trigger"}]}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(tt.body)
			require.Equal(t, tt.want, extractLatestUsageRequestContent(ContentModerationProtocolOpenAIResponses, body))
			require.Equal(t, tt.body, string(body))
		})
	}
}

func TestUsageRequestBodyClientWrapperBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"attributes", "<user_instructions priority=\"0\">built-in context</user_instructions>\n\nhello", "hello"},
		{"quoted delimiter", "<system-reminder note=\"a > b\">built-in context</system-reminder>\nhello", "hello"},
		{"multiline attributes", "<environment_context\n source='client'>built-in context</environment_context>\nhello", "hello"},
		{"browser context", "<in-app-browser-context source=\"ambient-ui-state\">Current URL: https://example.test/</in-app-browser-context>\n\nhello", "hello"},
		{"browser context only", "<in-app-browser-context source=\"ambient-ui-state\">internal context</in-app-browser-context>", ""},
		{"unclosed browser context", "<in-app-browser-context source=\"ambient-ui-state\">internal context", ""},
		{"unclosed attributed wrapper", "hello\n<user_instructions priority=\"0\">internal context", "hello"},
		{"nested wrapper", "<system-reminder>outer\n<system-reminder>inner</system-reminder>\nouter tail</system-reminder>\nhello", "hello"},
		{"indented closing tag", "<system-reminder>\ninternal context\n    </system-reminder>\nhello", "hello"},
		{"tab-indented closing tag", "<system-reminder>\ninternal context\n\t</system-reminder>\nhello", "hello"},
		{"indented nested wrapper", "<system-reminder>outer\n    <system-reminder>inner\n</system-reminder>\nouter tail</system-reminder>\nhello", "hello"},
		{"fenced closing tag inside wrapper", "<system-reminder>internal context\n```xml\n    </system-reminder>\n```\ninternal tail\n</system-reminder>\nhello", "hello"},
		{"agents header with blank line", "# AGENTS.md instructions\n\n<INSTRUCTIONS priority=\"0\">internal context</INSTRUCTIONS>\nhello", "hello"},
		{"agents header CRLF", "# AGENTS.md instructions\r\n<INSTRUCTIONS>internal context</INSTRUCTIONS>\r\nhello", "hello"},
		{"wrapper after code fence", "```text\nexample\n```\n<system-reminder>internal context</system-reminder>\nhello", "```text\nexample\n```\n\nhello"},
		{"code before actual user query", "```xml\n<user_query>example</user_query>\n```\n<user_query>hello</user_query>", "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(map[string]string{"input": tt.input})
			require.NoError(t, err)
			require.Equal(t, tt.want, extractLatestUsageRequestContent(ContentModerationProtocolOpenAIResponses, body))
		})
	}
}

func TestUsageRequestBodyPreservesUserCodeExamples(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"backtick fence", "Please explain this code:\n```xml\n<system-reminder>user example</system-reminder>\n```"},
		{"tilde fence", "~~~xml\n<user_instructions priority=\"0\">user example</user_instructions>\n~~~"},
		{"long fence with shorter inner fence", "````text\n```xml\n<system-reminder>user example</system-reminder>\n```\n````"},
		{"unclosed fence", "```xml\n<system-reminder>user example</system-reminder>"},
		{"CRLF fence", "  ```xml\r\n<environment_context>user example</environment_context>\r\n  ```"},
		{"indented code", "Example:\n\n    <system-reminder>user example</system-reminder>\n\thello"},
		{"agents example", "```text\n# AGENTS.md instructions\n<INSTRUCTIONS>user example</INSTRUCTIONS>\n```"},
		{"user query example", "Explain:\n```xml\n<user_query>user example</user_query>\n```\nKeep the whole question."},
		{"browser context example", "```xml\n<in-app-browser-context source=\"ambient-ui-state\">user example</in-app-browser-context>\n```"},
		{"inline tag mention", "Explain <user_instructions priority=\"0\"> and its closing tag </user_instructions>."},
		{"different tag name", "<user_instructions_example>user example</user_instructions_example>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(map[string]string{"input": tt.input})
			require.NoError(t, err)
			require.Equal(t, tt.input, extractLatestUsageRequestContent(ContentModerationProtocolOpenAIResponses, body))
		})
	}
}
