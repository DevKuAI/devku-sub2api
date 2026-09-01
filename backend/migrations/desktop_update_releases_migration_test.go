package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDesktopUpdateReleasesMigrationConstrainsLifecycleAndArtifacts(t *testing.T) {
	content, err := FS.ReadFile("232_desktop_update_releases.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "public_id VARCHAR(40) NOT NULL UNIQUE")
	require.Contains(t, sql, "version VARCHAR(64) NOT NULL UNIQUE")
	require.Contains(t, sql, "artifacts JSONB NOT NULL")
	require.Contains(t, sql, "CHECK (status IN ('draft', 'published', 'withdrawn'))")
	require.Contains(t, sql, "jsonb_object_length(artifacts) = 3")
	require.Contains(t, sql, "artifacts ?& ARRAY['darwin-aarch64', 'darwin-x86_64', 'windows-x86_64']")
	require.Contains(t, sql, "jsonb_typeof(artifacts #> '{darwin-aarch64,url}') = 'string'")
	require.Contains(t, sql, "jsonb_typeof(artifacts #> '{darwin-aarch64,object_key}') = 'string'")
	require.Contains(t, sql, "jsonb_typeof(artifacts #> '{darwin-aarch64,size}') = 'number'")
	require.Contains(t, sql, "jsonb_typeof(artifacts #> '{windows-x86_64,sha256}') = 'string'")
	require.Contains(t, sql, "status = 'withdrawn' AND published_at IS NOT NULL AND withdrawn_at IS NOT NULL AND withdrawal_reason IS NOT NULL")
	require.Contains(t, sql, "idx_desktop_update_releases_status_published")
	require.Contains(t, sql, "idx_desktop_update_releases_created_at")
}
