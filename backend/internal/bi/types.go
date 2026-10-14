// Package bi exposes independent mini-program identities and scoped Desktop reads.
package bi

import "time"

const (
	MobileAudience = "bi-miniprogram"
	WebAudience    = "sub2api-web"
	TokenIssuer    = "devku-sub2api"
	AccessTTL      = 15 * time.Minute
	RefreshTTL     = 12 * time.Hour
	InvitationTTL  = 24 * time.Hour
)

var Capabilities = []string{"analytics:read", "members:read", "conversations:read", "reports:read", "reports:copy"}

type User struct {
	ID                   string `json:"id"`
	DisplayName          string `json:"display_name"`
	Status               string `json:"status"`
	AuthorizationVersion int64  `json:"authorization_version"`
}
type Principal struct{ UserID, SessionID string }
type Session struct {
	Result           string `json:"result"`
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	User             User   `json:"user"`
}
type Access struct {
	UserID                 string   `json:"bi_user_id"`
	OrganizationAlias      string   `json:"organization_alias"`
	DisplayName            string   `json:"display_name"`
	OrganizationID         string   `json:"organization_id"`
	Name                   string   `json:"name"`
	Scope                  string   `json:"scope"`
	Capabilities           []string `json:"capabilities"`
	Status                 string   `json:"status"`
	Revision               int64    `json:"revision"`
	InternalOrganizationID int64    `json:"-"`
}
type Invitation struct {
	ID               string     `json:"id"`
	OrganizationID   string     `json:"organization_id"`
	OrganizationName string     `json:"organization_name"`
	Capabilities     []string   `json:"capabilities"`
	RecipientName    string     `json:"recipient_name"`
	Note             string     `json:"note"`
	Status           string     `json:"status"`
	ExpiresAt        time.Time  `json:"expires_at"`
	CreatedAt        time.Time  `json:"created_at"`
	ClaimedBy        *string    `json:"claimed_by"`
	ClaimedAt        *time.Time `json:"claimed_at"`
}
type IssuedInvitation struct {
	Invitation
	Token string `json:"token"`
	Path  string `json:"path"`
}
type Page[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
	HasMore  bool  `json:"has_more"`
}

func pageResult[T any](items []T, page, size int, total int64) Page[T] {
	if items == nil {
		items = []T{}
	}
	return Page[T]{Items: items, Page: page, PageSize: size, Total: total, HasMore: int64(page*size) < total}
}
