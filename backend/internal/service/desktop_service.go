package service

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

type DesktopService struct {
	repo    DesktopRepository
	usage   DesktopUsageRepository
	refresh DesktopRefreshStore
	limiter DesktopLoginLimiter
	tokens  *DesktopTokenManager
	apiKeys *APIKeyService
	config  config.DesktopConfig
	now     func() time.Time
}

func NewDesktopService(
	repo DesktopRepository,
	usage DesktopUsageRepository,
	refresh DesktopRefreshStore,
	limiter DesktopLoginLimiter,
	tokens *DesktopTokenManager,
	apiKeys *APIKeyService,
	cfg *config.Config,
) *DesktopService {
	return &DesktopService{
		repo: repo, usage: usage, refresh: refresh, limiter: limiter,
		tokens: tokens, apiKeys: apiKeys, config: cfg.Desktop, now: time.Now,
	}
}

func (s *DesktopService) CreateOrganization(ctx context.Context, input DesktopCreateOrganizationInput) (*DesktopOrganization, error) {
	code, err := NormalizeDesktopOrganizationCode(input.Code)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 200 || input.GatewayUserID <= 0 || input.GroupID <= 0 {
		return nil, ErrDesktopValidation
	}
	publicID, err := GenerateDesktopPublicID("org")
	if err != nil {
		return nil, err
	}
	input.PublicID, input.Code, input.Name = publicID, code, name
	return s.repo.CreateOrganization(ctx, input)
}

func (s *DesktopService) ListOrganizations(ctx context.Context, params pagination.PaginationParams, filters DesktopOrganizationListFilters) ([]DesktopOrganization, *pagination.PaginationResult, error) {
	filters.Search = strings.TrimSpace(filters.Search)
	if err := validateDesktopStatusFilter(filters.Status); err != nil {
		return nil, nil, err
	}
	return s.repo.ListOrganizations(ctx, params, filters)
}

func (s *DesktopService) GetOrganization(ctx context.Context, publicID string) (*DesktopOrganization, error) {
	return s.repo.GetOrganization(ctx, publicID)
}

func (s *DesktopService) UpdateOrganization(ctx context.Context, publicID string, input DesktopUpdateOrganizationInput) (*DesktopOrganization, error) {
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" || len([]rune(name)) > 200 {
			return nil, ErrDesktopValidation.WithMetadata(map[string]string{"field": "name"})
		}
		input.Name = &name
	}
	if input.Status != nil {
		if err := validateDesktopStatus(*input.Status); err != nil {
			return nil, err
		}
	}
	updated, keys, err := s.repo.UpdateOrganization(ctx, publicID, input)
	if err != nil {
		return nil, err
	}
	s.invalidateAPIKeys(ctx, keys)
	return updated, nil
}

func (s *DesktopService) UpdateTargetConfig(ctx context.Context, publicID string, target *DesktopTargetConfig) (*DesktopOrganization, error) {
	if err := target.Validate(); err != nil {
		return nil, err
	}
	return s.repo.UpdateTargetConfig(ctx, publicID, target)
}

func (s *DesktopService) CreateMember(ctx context.Context, organizationPublicID, name, phone string) (*DesktopMember, error) {
	organization, err := s.repo.GetOrganization(ctx, organizationPublicID)
	if err != nil {
		return nil, err
	}
	if organization.Status != DesktopStatusActive {
		return nil, ErrDesktopOrganizationDisabled
	}
	member, err := s.buildDesktopMember(organizationPublicID, name, phone)
	if err != nil {
		return nil, err
	}
	key, err := s.newMemberAPIKey(organization, member.PublicID)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateMember(ctx, organizationPublicID, DesktopCreateMemberInput{Member: member, APIKey: key})
}

func (s *DesktopService) ListMembers(ctx context.Context, organizationPublicID string, params pagination.PaginationParams, filters DesktopMemberListFilters) ([]DesktopMember, *pagination.PaginationResult, error) {
	filters.Search = strings.TrimSpace(filters.Search)
	if err := validateDesktopStatusFilter(filters.Status); err != nil {
		return nil, nil, err
	}
	if filters.Search != "" {
		if normalized, err := NormalizeDesktopPhone(filters.Search); err == nil {
			filters.Phone = normalized
			filters.Search = ""
		}
	}
	return s.repo.ListMembers(ctx, organizationPublicID, params, filters)
}

func (s *DesktopService) ListMembersWithUsage(ctx context.Context, organizationPublicID string, params pagination.PaginationParams, filters DesktopMemberListFilters) ([]DesktopMember, *pagination.PaginationResult, error) {
	members, result, err := s.ListMembers(ctx, organizationPublicID, params, filters)
	if err != nil || len(members) == 0 {
		return members, result, err
	}

	now := s.now()
	memberIDs := make([]int64, len(members))
	for i := range members {
		memberIDs[i] = members[i].ID
	}
	usage, err := s.usage.GetDesktopMembersUsage(
		ctx,
		memberIDs,
		timezone.StartOfDay(now).UTC(),
		now.AddDate(0, 0, -30).UTC(),
		now.UTC(),
	)
	if err != nil {
		return nil, nil, ErrDesktopUsageUnavailable.WithCause(err)
	}
	for i := range members {
		members[i].Usage = usage[members[i].ID]
		if members[i].Usage == nil {
			members[i].Usage = &DesktopMemberUsage{}
		}
	}
	return members, result, nil
}

func (s *DesktopService) UpdateMember(ctx context.Context, organizationPublicID, memberPublicID string, input DesktopUpdateMemberInput, phone *string) (*DesktopMember, error) {
	if input.Name != nil {
		name, err := NormalizeDesktopName(*input.Name)
		if err != nil {
			return nil, err
		}
		input.Name, input.NameNormalized = &name, &name
	}
	if input.Status != nil {
		if err := validateDesktopStatus(*input.Status); err != nil {
			return nil, err
		}
	}
	if phone != nil {
		e164, err := NormalizeDesktopPhone(*phone)
		if err != nil {
			return nil, err
		}
		input.Phone = &e164
	}
	updated, keys, err := s.repo.UpdateMember(ctx, organizationPublicID, memberPublicID, input)
	if err != nil {
		return nil, err
	}
	s.invalidateAPIKeys(ctx, keys)
	if input.Name != nil || phone != nil || input.Status != nil {
		_ = s.refresh.RevokeMember(ctx, memberPublicID)
	}
	return updated, nil
}

func (s *DesktopService) DeleteMember(ctx context.Context, organizationPublicID, memberPublicID string) error {
	keys, err := s.repo.DeleteMember(ctx, organizationPublicID, memberPublicID)
	if err != nil {
		return err
	}
	s.invalidateAPIKeys(ctx, keys)
	_ = s.refresh.RevokeMember(ctx, memberPublicID)
	return nil
}

func (s *DesktopService) RotateMemberAPIKey(ctx context.Context, organizationPublicID, memberPublicID string) (*DesktopMember, error) {
	organization, err := s.repo.GetOrganization(ctx, organizationPublicID)
	if err != nil {
		return nil, err
	}
	key, err := s.newMemberAPIKey(organization, memberPublicID)
	if err != nil {
		return nil, err
	}
	member, oldKeys, err := s.repo.RotateMemberAPIKey(ctx, organizationPublicID, memberPublicID, key)
	if err != nil {
		return nil, err
	}
	s.invalidateAPIKeys(ctx, oldKeys)
	return member, nil
}

func (s *DesktopService) ListAvailableGatewayUsers(ctx context.Context, params pagination.PaginationParams, search string) ([]User, *pagination.PaginationResult, error) {
	return s.repo.ListAvailableGatewayUsers(ctx, params, strings.TrimSpace(search))
}

func (s *DesktopService) EnsureGatewayUserCanBeDeleted(ctx context.Context, userID int64) error {
	hasDependency, err := s.repo.HasOrganizationForUser(ctx, userID)
	if err != nil {
		return err
	}
	if hasDependency {
		return ErrDesktopDependency
	}
	return nil
}

func (s *DesktopService) EnsureGroupCanBeDeleted(ctx context.Context, groupID int64) error {
	hasDependency, err := s.repo.HasOrganizationForGroup(ctx, groupID)
	if err != nil {
		return err
	}
	if hasDependency {
		return ErrDesktopDependency
	}
	return nil
}

func (s *DesktopService) EnsureGatewayUserGroupAccess(ctx context.Context, userID int64, restrictPublicGroups bool, allowedGroupIDs []int64) error {
	reader, ok := s.repo.(interface {
		DesktopOrganizationGroupForUser(context.Context, int64) (int64, bool, bool, error)
	})
	if !ok {
		return s.EnsureGatewayUserCanBeDeleted(ctx, userID)
	}
	groupID, isExclusive, assigned, err := reader.DesktopOrganizationGroupForUser(ctx, userID)
	if err != nil || !assigned {
		return err
	}
	user := User{AllowedGroups: allowedGroupIDs, RestrictPublicGroups: restrictPublicGroups}
	if user.CanBindGroup(groupID, isExclusive) {
		return nil
	}
	return ErrDesktopDependency
}

type DesktopOrganizationLookupResult struct {
	PublicID string `json:"public_id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
}

func (s *DesktopService) LookupOrganization(ctx context.Context, code, ip, installationID string) (*DesktopOrganizationLookupResult, error) {
	retry, err := s.limiter.AllowLookup(ctx, ip, installationID)
	if err != nil {
		return nil, err
	}
	if retry > 0 {
		return nil, rateLimitedError(retry)
	}
	normalized, err := NormalizeDesktopOrganizationCode(code)
	if err != nil {
		return nil, ErrDesktopOrganizationNotFound
	}
	organization, err := s.repo.FindActiveOrganizationByCode(ctx, normalized)
	if err != nil {
		return nil, ErrDesktopOrganizationNotFound
	}
	return &DesktopOrganizationLookupResult{PublicID: organization.PublicID, Code: organization.Code, Name: organization.Name}, nil
}

type DesktopTokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

func (s *DesktopService) Login(ctx context.Context, organizationCode, name, phone, ip, installationID string) (*DesktopTokenPair, error) {
	code := strings.ToLower(strings.TrimSpace(organizationCode))
	e164, phoneErr := NormalizeDesktopPhone(phone)
	phoneHashInput := strings.TrimSpace(phone)
	if phoneErr == nil {
		phoneHashInput = e164
	}
	phoneHash := HashDesktopOpaqueToken("desktop-phone:v1:" + phoneHashInput)
	retry, err := s.limiter.AllowLogin(ctx, ip, installationID, code, phoneHash)
	if err != nil {
		return nil, err
	}
	if retry > 0 {
		return nil, rateLimitedError(retry)
	}
	if phoneErr != nil {
		return nil, s.loginFailure(ctx, code, phoneHash)
	}
	organization, err := s.repo.FindActiveOrganizationByCode(ctx, code)
	if err != nil {
		return nil, s.loginFailure(ctx, code, phoneHash)
	}
	member, err := s.repo.FindMemberByPhone(ctx, organization.ID, e164)
	if err != nil {
		return nil, s.loginFailure(ctx, code, phoneHash)
	}
	normalizedName, err := NormalizeDesktopName(name)
	if err != nil || subtle.ConstantTimeCompare([]byte(normalizedName), []byte(member.NameNormalized)) != 1 || member.Status != DesktopStatusActive {
		return nil, s.loginFailure(ctx, code, phoneHash)
	}
	authorized, err := s.repo.GetAuthorizedMember(ctx, member.PublicID)
	if err != nil || !desktopAuthorizationCurrent(authorized) {
		return nil, s.loginFailure(ctx, code, phoneHash)
	}
	if err := s.limiter.ClearLoginFailures(ctx, code, phoneHash); err != nil {
		return nil, err
	}
	return s.issueNewSession(ctx, authorized)
}

func (s *DesktopService) Refresh(ctx context.Context, refreshToken string) (*DesktopTokenPair, error) {
	newRefresh, err := GenerateDesktopRefreshToken()
	if err != nil {
		return nil, err
	}
	result, session, err := s.refresh.Rotate(ctx, HashDesktopOpaqueToken(refreshToken), HashDesktopOpaqueToken(newRefresh), s.now())
	if err != nil {
		return nil, err
	}
	if result != DesktopRefreshRotated || session == nil {
		return nil, ErrDesktopRefreshInvalid
	}
	authorized, err := s.repo.GetAuthorizedMember(ctx, session.MemberPublicID)
	if err != nil || !desktopAuthorizationCurrent(authorized) {
		_ = s.refresh.RevokeFamily(ctx, session.FamilyID)
		return nil, ErrDesktopMembershipRevoked
	}
	access, err := s.issueAccessToken(authorized, session.FamilyID)
	if err != nil {
		_ = s.refresh.RevokeFamily(ctx, session.FamilyID)
		return nil, err
	}
	return &DesktopTokenPair{AccessToken: access, RefreshToken: newRefresh, TokenType: "Bearer", ExpiresIn: s.config.AccessTokenTTLMinutes * 60}, nil
}

func (s *DesktopService) Logout(ctx context.Context, accessToken string) error {
	claims, err := s.tokens.Parse(accessToken)
	if err != nil {
		return err
	}
	return s.refresh.RevokeFamily(ctx, claims.SessionID)
}

func (s *DesktopService) Authorize(ctx context.Context, accessToken string) (*DesktopAuthorizedMember, *DesktopAccessClaims, error) {
	claims, err := s.tokens.Parse(accessToken)
	if err != nil {
		return nil, nil, err
	}
	authorized, err := s.repo.GetAuthorizedMember(ctx, claims.Subject)
	if err != nil || !desktopAuthorizationCurrent(authorized) || authorized.Organization.PublicID != claims.OrganizationID ||
		authorized.Member.AuthVersion != claims.MemberVersion || authorized.Organization.AuthVersion != claims.OrganizationVersion {
		return nil, nil, ErrDesktopMembershipRevoked
	}
	return authorized, claims, nil
}

type DesktopMe struct {
	MemberPublicID       string `json:"public_id"`
	Name                 string `json:"name"`
	Phone                string `json:"phone"`
	OrganizationPublicID string `json:"organization_id"`
	OrganizationCode     string `json:"organization_code"`
	OrganizationName     string `json:"organization_name"`
}

func (s *DesktopService) Me(authorized *DesktopAuthorizedMember) DesktopMe {
	return DesktopMe{
		MemberPublicID: authorized.Member.PublicID, Name: authorized.Member.Name, Phone: authorized.Member.Phone,
		OrganizationPublicID: authorized.Organization.PublicID, OrganizationCode: authorized.Organization.Code, OrganizationName: authorized.Organization.Name,
	}
}

type DesktopModelConfiguration struct {
	ConfigurationVersion string                   `json:"configuration_version"`
	BaseURL              string                   `json:"base_url"`
	ModelToken           string                   `json:"model_token"`
	Targets              map[string]DesktopTarget `json:"targets"`
}

func (s *DesktopService) ModelConfiguration(authorized *DesktopAuthorizedMember, requestedTargets []string) (*DesktopModelConfiguration, string, error) {
	member, organization := authorized.Member, authorized.Organization
	if member.CurrentAPIKeyID == nil || member.CurrentAPIKeyDeleted || member.CurrentAPIKeyStatus != StatusAPIKeyActive || member.CurrentAPIKey == "" || organization.TargetConfig == nil {
		return nil, "", ErrDesktopConfigurationMissing
	}
	selected := selectDesktopTargets(organization.TargetConfig, requestedTargets)
	if len(selected) == 0 {
		return nil, "", ErrDesktopConfigurationMissing
	}
	canonical, err := organization.TargetConfig.CanonicalJSON()
	if err != nil {
		return nil, "", ErrDesktopConfigurationMissing.WithCause(err)
	}
	version := desktopConfigurationVersion(canonical, s.config.PublicGatewayBaseURL, *member.CurrentAPIKeyID, member.CurrentAPIKeyStatus)
	return &DesktopModelConfiguration{
		ConfigurationVersion: version, BaseURL: s.config.PublicGatewayBaseURL,
		ModelToken: member.CurrentAPIKey, Targets: selected,
	}, version, nil
}

type DesktopUsagePeriod struct {
	Used      int64  `json:"used"`
	Limit     *int64 `json:"limit"`
	Remaining *int64 `json:"remaining"`
}

type DesktopUsageSummary struct {
	Timezone string             `json:"timezone"`
	Today    DesktopUsagePeriod `json:"today"`
	Month    DesktopUsagePeriod `json:"month"`
}

func (s *DesktopService) UsageSummary(ctx context.Context, authorized *DesktopAuthorizedMember, timezoneName string) (*DesktopUsageSummary, error) {
	location, err := time.LoadLocation(timezoneName)
	if err != nil || timezoneName == "Local" {
		return nil, ErrDesktopValidation.WithMetadata(map[string]string{"field": "timezone"})
	}
	keyIDs, err := s.repo.ListMemberAPIKeyIDs(ctx, authorized.Member.ID)
	if err != nil {
		return nil, ErrDesktopUsageUnavailable.WithCause(err)
	}
	now := s.now().In(location)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
	today, err := s.usage.GetAPIKeysStatsAggregated(ctx, keyIDs, todayStart.UTC(), todayStart.AddDate(0, 0, 1).UTC())
	if err != nil {
		return nil, ErrDesktopUsageUnavailable.WithCause(err)
	}
	month, err := s.usage.GetAPIKeysStatsAggregated(ctx, keyIDs, monthStart.UTC(), monthStart.AddDate(0, 1, 0).UTC())
	if err != nil {
		return nil, ErrDesktopUsageUnavailable.WithCause(err)
	}
	return &DesktopUsageSummary{
		Timezone: timezoneName,
		Today:    DesktopUsagePeriod{Used: today.TotalTokens},
		Month:    DesktopUsagePeriod{Used: month.TotalTokens},
	}, nil
}

func (s *DesktopService) buildDesktopMember(organizationPublicID, name, phone string) (*DesktopMember, error) {
	normalizedName, err := NormalizeDesktopName(name)
	if err != nil {
		return nil, err
	}
	e164, err := NormalizeDesktopPhone(phone)
	if err != nil {
		return nil, err
	}
	publicID, err := GenerateDesktopPublicID("mem")
	if err != nil {
		return nil, err
	}
	return &DesktopMember{
		PublicID: publicID, Name: normalizedName, NameNormalized: normalizedName,
		Phone: e164, Status: DesktopStatusActive,
	}, nil
}

func (s *DesktopService) newMemberAPIKey(organization *DesktopOrganization, memberPublicID string) (*APIKey, error) {
	key, err := s.apiKeys.GenerateKey()
	if err != nil {
		return nil, err
	}
	groupID := organization.GroupID
	return &APIKey{
		UserID: organization.GatewayUserID, Key: key, Name: "desktop:" + organization.PublicID + ":" + memberPublicID,
		GroupID: &groupID, Status: StatusAPIKeyActive, IPWhitelist: []string{}, IPBlacklist: []string{},
	}, nil
}

func (s *DesktopService) issueNewSession(ctx context.Context, authorized *DesktopAuthorizedMember) (*DesktopTokenPair, error) {
	familyID, err := GenerateDesktopPublicID("session")
	if err != nil {
		return nil, err
	}
	refreshToken, err := GenerateDesktopRefreshToken()
	if err != nil {
		return nil, err
	}
	accessToken, err := s.issueAccessToken(authorized, familyID)
	if err != nil {
		return nil, err
	}
	session := DesktopRefreshSession{
		FamilyID: familyID, MemberPublicID: authorized.Member.PublicID,
		AbsoluteExpiresAt: s.now().AddDate(0, 0, s.config.RefreshFamilyTTLDays),
	}
	if err := s.refresh.Create(ctx, HashDesktopOpaqueToken(refreshToken), session); err != nil {
		return nil, err
	}
	return &DesktopTokenPair{AccessToken: accessToken, RefreshToken: refreshToken, TokenType: "Bearer", ExpiresIn: s.config.AccessTokenTTLMinutes * 60}, nil
}

func (s *DesktopService) issueAccessToken(authorized *DesktopAuthorizedMember, familyID string) (string, error) {
	return s.tokens.Issue(
		&DesktopMemberOrganizationClaims{PublicID: authorized.Member.PublicID, AuthVersion: authorized.Member.AuthVersion},
		&DesktopMemberOrganizationClaims{PublicID: authorized.Organization.PublicID, AuthVersion: authorized.Organization.AuthVersion},
		familyID, s.now(),
	)
}

func (s *DesktopService) loginFailure(ctx context.Context, code, phoneHash string) error {
	retry, err := s.limiter.RecordLoginFailure(ctx, code, phoneHash)
	if err != nil {
		return err
	}
	if retry > 0 {
		return rateLimitedError(retry)
	}
	return ErrDesktopMemberMismatch
}

func (s *DesktopService) invalidateAPIKeys(ctx context.Context, keys []string) {
	for _, key := range keys {
		if key != "" {
			s.apiKeys.InvalidateAuthCacheByKey(ctx, key)
		}
	}
}

func desktopAuthorizationCurrent(authorized *DesktopAuthorizedMember) bool {
	return authorized != nil && authorized.Member != nil && authorized.Organization != nil && authorized.GatewayUser != nil &&
		authorized.Member.Status == DesktopStatusActive && authorized.Organization.Status == DesktopStatusActive && authorized.GatewayUser.Status == domain.StatusActive
}

func validateDesktopStatus(status string) error {
	if status != DesktopStatusActive && status != DesktopStatusDisabled {
		return ErrDesktopValidation.WithMetadata(map[string]string{"field": "status"})
	}
	return nil
}

func validateDesktopStatusFilter(status string) error {
	if status == "" {
		return nil
	}
	return validateDesktopStatus(status)
}

func rateLimitedError(retry time.Duration) error {
	seconds := int(retry.Round(time.Second).Seconds())
	if seconds < 1 {
		seconds = 1
	}
	return ErrDesktopRateLimited.WithMetadata(map[string]string{"retry_after": strconv.Itoa(seconds)})
}

func selectDesktopTargets(targetConfig *DesktopTargetConfig, requested []string) map[string]DesktopTarget {
	wanted := make(map[string]bool, len(requested))
	for _, name := range requested {
		wanted[strings.TrimSpace(name)] = true
	}
	include := func(name string) bool { return len(wanted) == 0 || wanted[name] }
	selected := make(map[string]DesktopTarget, 2)
	if target := targetConfig.Targets.ChatGPTCodex; target != nil && target.Enabled && include("chatgpt_codex") {
		selected["chatgpt_codex"] = *target
	}
	if target := targetConfig.Targets.Workbuddy; target != nil && target.Enabled && include("workbuddy") {
		selected["workbuddy"] = *target
	}
	return selected
}

func desktopConfigurationVersion(canonical []byte, baseURL string, keyID int64, keyStatus string) string {
	values := [][]byte{canonical, []byte(baseURL), []byte(strconv.FormatInt(keyID, 10)), []byte(keyStatus)}
	hash := sha256.New()
	var length [8]byte
	for _, value := range values {
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write(value)
	}
	return "cfg_" + hex.EncodeToString(hash.Sum(nil))[:20]
}
