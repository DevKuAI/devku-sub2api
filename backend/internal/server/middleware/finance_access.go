package middleware

import (
	"net/http"
	"strings"
)

// Match registered routes so encoded paths and query parameters cannot widen access.
func isAccountingPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/admin/accounting/")
}

// Finance users only need their current identity and session revocation outside accounting.
func financeSessionAllowed(method, path string) bool {
	return (method == http.MethodGet && path == "/api/v1/auth/me") ||
		(method == http.MethodPost && path == "/api/v1/auth/revoke-all-sessions")
}
