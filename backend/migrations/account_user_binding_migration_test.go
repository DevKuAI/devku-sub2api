package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountUserBindingMigrationAllowsOneUserToOwnMultipleAccounts(t *testing.T) {
	content, err := FS.ReadFile("235_account_user_binding.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS bound_user_id BIGINT")
	require.Contains(t, sql, "FOREIGN KEY (bound_user_id) REFERENCES users(id) ON DELETE SET NULL")
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS idx_accounts_bound_user_id")
	require.NotContains(t, sql, "UNIQUE")
}
