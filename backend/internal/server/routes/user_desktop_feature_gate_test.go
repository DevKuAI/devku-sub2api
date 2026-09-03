package routes

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDesktopUserRoutesFollowFeatureFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, test := range []struct {
		name    string
		enabled bool
		want    int
	}{
		{name: "disabled"},
		{name: "enabled", enabled: true, want: 8},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			handlers := &handler.Handlers{Desktop: handler.NewDesktopHandler(nil)}
			cfg := &config.Config{Desktop: config.DesktopConfig{Enabled: test.enabled}}

			registerDesktopUserRoutesIfEnabled(router.Group("/api/v1"), handlers, cfg)

			registered := 0
			for _, route := range router.Routes() {
				if strings.HasPrefix(route.Path, "/api/v1/desktop/organization") {
					registered++
				}
			}
			require.Equal(t, test.want, registered)
		})
	}
}
