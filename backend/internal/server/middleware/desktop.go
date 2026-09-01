package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/desktopresponse"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const desktopAuthorizedMemberContextKey = "desktop_authorized_member"

func StrictBodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

func DesktopAdminBodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet {
			c.Next()
			return
		}
		limit := int64(16 * 1024)
		if strings.HasSuffix(c.Request.URL.Path, "/model-configuration") {
			limit = 64 * 1024
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}

func DesktopUpdateArtifactBodyLimit(cfg *config.Config) gin.HandlerFunc {
	maxFileBytes := int64(200 << 20)
	if cfg != nil && cfg.DesktopUpdateStorage.MaxUploadBytes > 0 {
		maxFileBytes = cfg.DesktopUpdateStorage.MaxUploadBytes
	}
	return StrictBodyLimit(maxFileBytes + (1 << 20))
}

func IsBodyTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}

func DesktopAuth(desktop *service.DesktopService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c.GetHeader("Authorization"))
		authorized, _, err := desktop.Authorize(c.Request.Context(), token)
		if err != nil {
			desktopresponse.Error(c, err)
			c.Abort()
			return
		}
		c.Set(desktopAuthorizedMemberContextKey, authorized)
		c.Next()
	}
}

func GetDesktopAuthorizedMember(c *gin.Context) (*service.DesktopAuthorizedMember, bool) {
	value, exists := c.Get(desktopAuthorizedMemberContextKey)
	member, ok := value.(*service.DesktopAuthorizedMember)
	return member, exists && ok
}

func bearerToken(header string) string {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}
