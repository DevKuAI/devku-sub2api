package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDesktopUpdateUploadTokenFromEnvironment(t *testing.T) {
	for _, token := range []string{"", strings.Repeat("test", 16)} {
		t.Run("length="+strconv.Itoa(len(token)), func(t *testing.T) {
			resetViperWithJWTSecret(t)
			path := filepath.Join(t.TempDir(), "config.yaml")
			require.NoError(t, os.WriteFile(path, []byte("desktop_update_upload_token: ''\n"), 0600))
			t.Setenv("CONFIG_FILE", path)
			t.Setenv("DESKTOP_UPDATE_UPLOAD_TOKEN", token)
			cfg, err := Load()
			require.NoError(t, err)
			require.Equal(t, token, cfg.DesktopUpdateUploadToken)
		})
	}
}
