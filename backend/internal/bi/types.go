// Package bi implements the isolated enterprise analytics API.
package bi

import "time"

const (
	MobileAudience = "bi-miniprogram"
	WebAudience    = "sub2api-web"
	TokenIssuer    = "devku-sub2api"
	AccessTTL      = 15 * time.Minute
	RefreshTTL     = 12 * time.Hour
	ChallengeTTL   = 10 * time.Minute
)

type User struct {
	ID                   string   `json:"id"`
	DisplayName          string   `json:"display_name"`
	Capabilities         []string `json:"capabilities"`
	AuthorizationVersion string   `json:"authorization_version"`
}

type Organization struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Status       string   `json:"status"`
	Role         string   `json:"role"`
	AllTeams     bool     `json:"all_teams"`
	TeamIDs      []string `json:"team_ids"`
	Capabilities []string `json:"capabilities"`
}

type Session struct {
	Result           string `json:"result"`
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	User             User   `json:"user"`
}

type BindingChallenge struct {
	Result           string    `json:"result"`
	BindingTicket    string    `json:"binding_ticket"`
	UserCode         string    `json:"user_code"`
	ExpiresAt        time.Time `json:"expires_at"`
	VerificationPath string    `json:"verification_path"`
}

type BindingSummary struct {
	ID          string     `json:"id"`
	DisplayName string     `json:"display_name"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at"`
}

type Page[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
	SnapshotID string  `json:"snapshot_id"`
}

// Principal holds only server-side identity; it must never be serialized.
type Principal struct {
	ManagerID string
	UserID    int64
	BindingID string
	SessionID string
}
