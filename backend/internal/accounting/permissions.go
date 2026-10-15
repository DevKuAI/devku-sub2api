package accounting

import (
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"strings"
)

const (
	PermissionRead             = "accounting:read"
	PermissionWrite            = "accounting:write"
	PermissionConfirm          = "accounting:confirm"
	PermissionReconcile        = "accounting:reconcile"
	PermissionManage           = "accounting:manage"
	PermissionOrganizationRead = "organization:accounting:read"
)

func permissionFor(method, path string) string {
	if method == "GET" {
		return PermissionRead
	}
	if strings.Contains(path, "reconciliation") {
		return PermissionReconcile
	}
	if strings.HasSuffix(path, "/confirm") || strings.HasSuffix(path, "/verify") {
		return PermissionConfirm
	}
	if strings.HasSuffix(path, "/void") || strings.HasSuffix(path, "/close") || strings.HasSuffix(path, "/sharing") || method == "DELETE" {
		return PermissionManage
	}
	return PermissionWrite
}
func requireAccountingPermission(c *gin.Context) {
	role, ok := middleware.GetUserRoleFromContext(c)
	subject, authenticated := middleware.GetAuthSubjectFromContext(c)
	if !ok || (role != domain.RoleAdmin && role != domain.RoleFinance) || !authenticated || subject.UserID <= 0 {
		response.Forbidden(c, "Accounting permission required")
		c.Abort()
		return
	}
	c.Set("accounting_permission", permissionFor(c.Request.Method, c.FullPath()))
	c.Next()
}
