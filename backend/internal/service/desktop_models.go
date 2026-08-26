package service

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

const (
	DesktopStatusActive           = "active"
	DesktopStatusDisabled         = "disabled"
	DesktopWireAPIResponses       = "responses"
	DesktopWireAPIChatCompletions = "chat_completions"
)

var (
	ErrDesktopOrganizationNotFound = infraerrors.NotFound("ORGANIZATION_NOT_FOUND", "organization not found")
	ErrDesktopMemberNotFound       = infraerrors.NotFound("MEMBER_NOT_FOUND", "member not found")
	ErrDesktopConfigurationMissing = infraerrors.NotFound("MODEL_CONFIGURATION_NOT_ASSIGNED", "model configuration is not assigned")
	ErrDesktopValidation           = infraerrors.New(http.StatusUnprocessableEntity, "VALIDATION_FAILED", "validation failed")
	ErrDesktopUnauthenticated      = infraerrors.Unauthorized("UNAUTHENTICATED", "authentication required")
	ErrDesktopRefreshInvalid       = infraerrors.Unauthorized("REFRESH_TOKEN_INVALID", "refresh token is invalid")
	ErrDesktopMemberMismatch       = infraerrors.Unauthorized("MEMBER_INFORMATION_MISMATCH", "member information does not match")
	ErrDesktopMembershipRevoked    = infraerrors.Forbidden("MEMBERSHIP_REVOKED", "membership has been revoked")
	ErrDesktopGatewayUserAssigned  = infraerrors.Conflict("GATEWAY_USER_ALREADY_ASSIGNED", "gateway user is already assigned")
	ErrDesktopProvisioningLocked   = infraerrors.Conflict("ORGANIZATION_PROVISIONING_LOCKED", "organization provisioning fields are locked")
	ErrDesktopOrganizationDisabled = infraerrors.Conflict("ORGANIZATION_DISABLED", "organization is disabled")
	ErrDesktopMemberDisabled       = infraerrors.Conflict("MEMBER_DISABLED", "member is disabled")
	ErrDesktopManagedAPIKey        = infraerrors.Conflict("DESKTOP_MANAGED_API_KEY", "desktop managed API key cannot be changed here")
	ErrDesktopRotationConflict     = infraerrors.Conflict("MODEL_TOKEN_ROTATION_CONFLICT", "model token rotation conflict")
	ErrDesktopDependency           = infraerrors.Conflict("DESKTOP_ORGANIZATION_DEPENDENCY", "desktop organization dependency exists")
	ErrDesktopAuthStoreUnavailable = infraerrors.New(http.StatusServiceUnavailable, "DESKTOP_AUTH_STORE_UNAVAILABLE", "desktop authentication store is unavailable")
	ErrDesktopUsageUnavailable     = infraerrors.New(http.StatusServiceUnavailable, "USAGE_SOURCE_UNAVAILABLE", "usage source is unavailable")
	ErrDesktopRateLimited          = infraerrors.TooManyRequests("RATE_LIMITED", "too many requests")
)

type DesktopOrganization struct {
	ID                   int64
	PublicID             string
	Code                 string
	Name                 string
	Status               string
	AuthVersion          int64
	GatewayUserID        int64
	GatewayUserEmail     string
	GatewayUserName      string
	GroupID              int64
	GroupName            string
	TargetConfig         *DesktopTargetConfig
	MemberCount          int
	TargetConfigAssigned bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type DesktopMember struct {
	ID                            int64
	PublicID                      string
	OrganizationID                int64
	Name                          string
	NameNormalized                string
	Phone                         string
	Status                        string
	AuthVersion                   int64
	APIKeySuspendedByOrganization bool
	CurrentAPIKeyID               *int64
	CurrentAPIKey                 string
	CurrentAPIKeyStatus           string
	CurrentAPIKeyDeleted          bool
	CreatedAt                     time.Time
	UpdatedAt                     time.Time
}

func (m *DesktopMember) ModelTokenStatus() string {
	if m == nil || m.CurrentAPIKeyID == nil || m.CurrentAPIKeyDeleted {
		return "missing"
	}
	if m.CurrentAPIKeyStatus == StatusAPIKeyActive {
		return "active"
	}
	return "disabled"
}

type DesktopTargetConfig struct {
	SchemaVersion int            `json:"schema_version"`
	Targets       DesktopTargets `json:"targets"`
}

type DesktopTargets struct {
	ChatGPTCodex *DesktopTarget `json:"chatgpt_codex,omitempty"`
	Workbuddy    *DesktopTarget `json:"workbuddy,omitempty"`
}

type DesktopTarget struct {
	Enabled           bool    `json:"enabled"`
	ProviderID        string  `json:"provider_id"`
	DisplayName       string  `json:"display_name"`
	RequestedModel    string  `json:"requested_model"`
	WireAPI           string  `json:"wire_api"`
	MinimumAppVersion *string `json:"minimum_app_version"`
	RestartRequired   bool    `json:"restart_required"`
}

func (c *DesktopTargetConfig) Validate() error {
	if c == nil || c.SchemaVersion != 1 {
		return ErrDesktopValidation.WithMetadata(map[string]string{"field": "target_config.schema_version"})
	}
	if c.Targets.ChatGPTCodex == nil && c.Targets.Workbuddy == nil {
		return ErrDesktopValidation.WithMetadata(map[string]string{"field": "target_config.targets"})
	}
	if target := c.Targets.ChatGPTCodex; target != nil {
		if err := validateDesktopTarget("chatgpt_codex", target, DesktopWireAPIResponses); err != nil {
			return err
		}
	}
	if target := c.Targets.Workbuddy; target != nil {
		if target.Enabled {
			return ErrDesktopValidation.WithMetadata(map[string]string{"field": "target_config.targets.workbuddy.enabled"})
		}
		if err := validateDesktopTarget("workbuddy", target, DesktopWireAPIChatCompletions); err != nil {
			return err
		}
	}
	return nil
}

func validateDesktopTarget(name string, target *DesktopTarget, expectedWireAPI string) error {
	if target.WireAPI != expectedWireAPI {
		return ErrDesktopValidation.WithMetadata(map[string]string{"field": "target_config.targets." + name + ".wire_api"})
	}
	if strings.TrimSpace(target.ProviderID) == "" || strings.TrimSpace(target.DisplayName) == "" || strings.TrimSpace(target.RequestedModel) == "" {
		return ErrDesktopValidation.WithMetadata(map[string]string{"field": "target_config.targets." + name})
	}
	return nil
}

func (c *DesktopTargetConfig) CanonicalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(c)
}

func DecodeDesktopTargetConfig(raw []byte) (*DesktopTargetConfig, error) {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "{}" {
		return nil, nil
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var config DesktopTargetConfig
	if err := decoder.Decode(&config); err != nil {
		return nil, ErrDesktopValidation.WithCause(err)
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &config, nil
}

type DesktopOrganizationListFilters struct {
	Search string
	Status string
}

type DesktopMemberListFilters struct {
	Search string
	Status string
	Phone  string
}

type DesktopCreateOrganizationInput struct {
	PublicID      string
	Code          string
	Name          string
	GatewayUserID int64
	GroupID       int64
}

type DesktopUpdateOrganizationInput struct {
	Name          *string
	Status        *string
	GatewayUserID *int64
	GroupID       *int64
}

type DesktopCreateMemberInput struct {
	Member *DesktopMember
	APIKey *APIKey
}

type DesktopUpdateMemberInput struct {
	Name             *string
	NameNormalized   *string
	Phone            *string
	Status           *string
	RevokeCredential bool
}

type DesktopAuthorizedMember struct {
	Member       *DesktopMember
	Organization *DesktopOrganization
	GatewayUser  *User
}

type DesktopUsageRepository interface {
	GetAPIKeysStatsAggregated(ctx context.Context, apiKeyIDs []int64, startTime, endTime time.Time) (*usagestats.UsageStats, error)
}

type DesktopRepository interface {
	CreateOrganization(ctx context.Context, input DesktopCreateOrganizationInput) (*DesktopOrganization, error)
	ListOrganizations(ctx context.Context, params pagination.PaginationParams, filters DesktopOrganizationListFilters) ([]DesktopOrganization, *pagination.PaginationResult, error)
	GetOrganization(ctx context.Context, publicID string) (*DesktopOrganization, error)
	UpdateOrganization(ctx context.Context, publicID string, input DesktopUpdateOrganizationInput) (*DesktopOrganization, []string, error)
	UpdateTargetConfig(ctx context.Context, publicID string, config *DesktopTargetConfig) (*DesktopOrganization, error)
	CreateMember(ctx context.Context, organizationPublicID string, input DesktopCreateMemberInput) (*DesktopMember, error)
	ListMembers(ctx context.Context, organizationPublicID string, params pagination.PaginationParams, filters DesktopMemberListFilters) ([]DesktopMember, *pagination.PaginationResult, error)
	GetMember(ctx context.Context, organizationPublicID, memberPublicID string) (*DesktopMember, error)
	UpdateMember(ctx context.Context, organizationPublicID, memberPublicID string, input DesktopUpdateMemberInput) (*DesktopMember, []string, error)
	DeleteMember(ctx context.Context, organizationPublicID, memberPublicID string) ([]string, error)
	RotateMemberAPIKey(ctx context.Context, organizationPublicID, memberPublicID string, key *APIKey) (*DesktopMember, []string, error)
	FindActiveOrganizationByCode(ctx context.Context, code string) (*DesktopOrganization, error)
	FindMemberByPhone(ctx context.Context, organizationID int64, phone string) (*DesktopMember, error)
	GetAuthorizedMember(ctx context.Context, memberPublicID string) (*DesktopAuthorizedMember, error)
	ListMemberAPIKeyIDs(ctx context.Context, memberID int64) ([]int64, error)
	IsManagedAPIKey(ctx context.Context, apiKeyID int64) (bool, error)
	HasOrganizationForUser(ctx context.Context, userID int64) (bool, error)
	HasOrganizationForGroup(ctx context.Context, groupID int64) (bool, error)
	ListAvailableGatewayUsers(ctx context.Context, params pagination.PaginationParams, search string) ([]User, *pagination.PaginationResult, error)
}

type DesktopRefreshSession struct {
	FamilyID          string
	MemberPublicID    string
	AbsoluteExpiresAt time.Time
}

type DesktopRefreshRotateResult int

const (
	DesktopRefreshUnknown DesktopRefreshRotateResult = iota
	DesktopRefreshRotated
	DesktopRefreshReplayed
)

type DesktopRefreshStore interface {
	Create(ctx context.Context, tokenHash string, session DesktopRefreshSession) error
	Rotate(ctx context.Context, oldTokenHash, newTokenHash string, now time.Time) (DesktopRefreshRotateResult, *DesktopRefreshSession, error)
	RevokeFamily(ctx context.Context, familyID string) error
	RevokeMember(ctx context.Context, memberPublicID string) error
}

type DesktopLoginLimiter interface {
	AllowLookup(ctx context.Context, ip, installationID string) (time.Duration, error)
	AllowLogin(ctx context.Context, ip, installationID, organizationCode, phoneHash string) (time.Duration, error)
	RecordLoginFailure(ctx context.Context, organizationCode, phoneHash string) (time.Duration, error)
	ClearLoginFailures(ctx context.Context, organizationCode, phoneHash string) error
}

var desktopCodePattern = regexp.MustCompile(`^[a-z0-9]{2,16}$`)

func NormalizeDesktopOrganizationCode(code string) (string, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	if !desktopCodePattern.MatchString(code) {
		return "", ErrDesktopValidation.WithMetadata(map[string]string{"field": "code"})
	}
	return code, nil
}
