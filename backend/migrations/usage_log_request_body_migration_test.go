package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageLogRequestBodyMigration(t *testing.T) {
	content, err := FS.ReadFile("234_add_usage_log_request_body.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS request_body TEXT")
	require.NotContains(t, sql, "NOT NULL")
	require.NotContains(t, sql, "DEFAULT")
}
