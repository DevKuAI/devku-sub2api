package service

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/google/uuid"
)

const DesktopConversationMaxBodyBytes int64 = 32 << 20
const DesktopConversationMaxTextBytes = 2 << 20

var desktopUTCTimestamp = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z$`)

var (
	ErrDesktopConversationReportingDisabled = infraerrors.Forbidden("CONVERSATION_REPORTING_DISABLED", "conversation reporting is disabled for this organization")
	ErrDesktopConversationIdentity          = infraerrors.Forbidden("CONVERSATION_IDENTITY_MISMATCH", "conversation identity does not match the session")
	ErrDesktopConversationExists            = infraerrors.Conflict("CONVERSATION_RECORD_EXISTS", "conversation record already exists")
	ErrDesktopConversationNotFound          = infraerrors.NotFound("CONVERSATION_RECORD_NOT_FOUND", "conversation record not found")
	ErrDesktopConversationStorage           = infraerrors.New(http.StatusServiceUnavailable, "CONVERSATION_STORAGE_UNAVAILABLE", "conversation storage is unavailable")
	ErrDesktopMediaType                     = infraerrors.New(http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "uncompressed application/json is required")
)

type DesktopTextSegment struct {
	Text      string `json:"text"`
	Truncated bool   `json:"truncated"`
}

type DesktopConversationInput struct {
	SchemaVersion  int                  `json:"schemaVersion"`
	RecordID       string               `json:"recordId"`
	Client         string               `json:"client"`
	InstallationID string               `json:"installationId"`
	OrganizationID string               `json:"organizationId"`
	MemberID       string               `json:"memberId"`
	SessionID      string               `json:"sessionId"`
	SourceTurnID   *string              `json:"sourceTurnId"`
	StartedAt      time.Time            `json:"startedAt"`
	StoppedAt      time.Time            `json:"stoppedAt"`
	CWD            *string              `json:"cwd"`
	Prompts        []DesktopTextSegment `json:"prompts"`
	Response       *DesktopTextSegment  `json:"response"`
	CaptureStatus  string               `json:"captureStatus"`
}

// Metadata is deliberately separate from the full body so lists cannot expose it.
type DesktopConversationMetadata struct {
	RecordID        string    `json:"record_id"`
	OrganizationID  string    `json:"organization_id"`
	MemberID        string    `json:"member_id"`
	MemberName      string    `json:"member_name"`
	MemberDeleted   bool      `json:"member_deleted"`
	Client          string    `json:"client"`
	InstallationID  string    `json:"installation_id"`
	SourceSessionID string    `json:"source_session_id"`
	SourceTurnID    *string   `json:"source_turn_id"`
	StartedAt       time.Time `json:"started_at"`
	StoppedAt       time.Time `json:"stopped_at"`
	ReceivedAt      time.Time `json:"received_at"`
	CWD             *string   `json:"cwd"`
	CaptureStatus   string    `json:"capture_status"`
	PromptCount     int       `json:"prompt_count"`
}

type DesktopConversationDetail struct {
	DesktopConversationMetadata
	SchemaVersion int                  `json:"schema_version"`
	Prompts       []DesktopTextSegment `json:"prompts"`
	Response      *DesktopTextSegment  `json:"response"`
}

type DesktopConversationReceipt struct {
	RecordID   string    `json:"record_id"`
	ReceivedAt time.Time `json:"received_at"`
}

type DesktopConversationFilters struct {
	MemberID        string
	MemberSearch    string
	Client          string
	CaptureStatus   string
	RecordID        string
	SourceSessionID string
	InstallationID  string
	ReceivedFrom    *time.Time
	ReceivedTo      *time.Time
}

type DesktopConversationRepository interface {
	Create(context.Context, int64, int64, *DesktopConversationInput) (*DesktopConversationReceipt, error)
	List(context.Context, int64, pagination.PaginationParams, DesktopConversationFilters) ([]DesktopConversationMetadata, *pagination.PaginationResult, error)
	Get(context.Context, int64, string) (*DesktopConversationDetail, error)
	Statistics(context.Context, int64, DesktopConversationFilters, DesktopConversationPeriods) (*DesktopConversationStatistics, error)
}

type DesktopConversationLimiter interface {
	Allow(context.Context, string) (time.Duration, error)
}

func DecodeDesktopConversation(raw []byte) (*DesktopConversationInput, error) {
	fields, err := DecodeDesktopJSONObject(raw,
		[]string{"schemaVersion", "recordId", "client", "installationId", "organizationId", "memberId", "sessionId", "startedAt", "stoppedAt", "prompts", "response", "captureStatus"},
		[]string{"sourceTurnId", "cwd"}, []string{"response", "sourceTurnId", "cwd"})
	if err != nil {
		return nil, err
	}
	var input DesktopConversationInput
	if json.Unmarshal(raw, &input) != nil {
		return nil, ErrDesktopValidation
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}
	for _, name := range []string{"startedAt", "stoppedAt"} {
		var value string
		if json.Unmarshal(fields[name], &value) != nil || !desktopUTCTimestamp.MatchString(value) {
			return nil, ErrDesktopValidation
		}
	}
	var prompts []json.RawMessage
	if json.Unmarshal(fields["prompts"], &prompts) != nil {
		return nil, ErrDesktopValidation
	}
	for _, prompt := range prompts {
		if _, err := DecodeDesktopJSONObject(prompt, []string{"text", "truncated"}, nil, nil); err != nil {
			return nil, err
		}
	}
	if input.Response != nil {
		if _, err := DecodeDesktopJSONObject(fields["response"], []string{"text", "truncated"}, nil, nil); err != nil {
			return nil, err
		}
	}
	return &input, nil
}

// DecodeDesktopJSONObject rejects case aliases, duplicate keys and missing values.
// Go's normal struct decoder alone accepts those inputs and substitutes invalid UTF-8.
func DecodeDesktopJSONObject(raw []byte, required, optional, nullable []string) (map[string]json.RawMessage, error) {
	if !jsontext.Value(raw).IsValid() {
		return nil, ErrDesktopValidation
	}
	allowed, nulls := map[string]bool{}, map[string]bool{}
	for _, key := range append(append([]string{}, required...), optional...) {
		allowed[key] = true
	}
	for _, key := range nullable {
		nulls[key] = true
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, ErrDesktopValidation
	}
	fields := make(map[string]json.RawMessage, len(allowed))
	for decoder.More() {
		token, err := decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || !allowed[key] || fields[key] != nil {
			return nil, ErrDesktopValidation
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil || (!nulls[key] && bytes.Equal(bytes.TrimSpace(value), []byte("null"))) {
			return nil, ErrDesktopValidation
		}
		fields[key] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, ErrDesktopValidation
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, ErrDesktopValidation
	}
	for _, key := range required {
		if fields[key] == nil {
			return nil, ErrDesktopValidation
		}
	}
	return fields, nil
}

func (in *DesktopConversationInput) Validate() error {
	id, err := uuid.Parse(in.RecordID)
	if err != nil || id.String() != in.RecordID || id.Version() != 4 || id.Variant() != uuid.RFC4122 || in.SchemaVersion != 2 {
		return ErrDesktopValidation
	}
	if ValidateDesktopInstallationID(in.InstallationID) != nil || !desktopConversationClient(in.Client) {
		return ErrDesktopValidation
	}
	if !desktopConversationString(in.OrganizationID, 1, 64) || !desktopConversationString(in.MemberID, 1, 64) || !desktopConversationString(in.SessionID, 1, 512) {
		return ErrDesktopValidation
	}
	if in.SourceTurnID != nil && !desktopConversationString(*in.SourceTurnID, 1, 512) {
		return ErrDesktopValidation
	}
	if in.CWD != nil && !desktopConversationString(*in.CWD, 0, 4096) {
		return ErrDesktopValidation
	}
	if in.StartedAt.After(in.StoppedAt) || len(in.Prompts) < 1 || len(in.Prompts) > 64 {
		return ErrDesktopValidation
	}
	if (in.CaptureStatus != "captured" || in.Response == nil) && (in.CaptureStatus != "response_missing" || in.Response != nil) {
		return ErrDesktopValidation
	}
	for _, text := range in.Prompts {
		if !validDesktopSegment(text) {
			return ErrDesktopValidation
		}
	}
	if in.Response != nil && !validDesktopSegment(*in.Response) {
		return ErrDesktopValidation
	}
	return nil
}

func desktopConversationClient(value string) bool {
	return value == "workbuddy" || value == "chatgpt_codex"
}
func desktopConversationString(value string, min, max int) bool {
	length := utf8.RuneCountInString(value)
	return utf8.ValidString(value) && !strings.ContainsRune(value, 0) && length >= min && length <= max
}
func validDesktopSegment(value DesktopTextSegment) bool {
	return len(value.Text) <= DesktopConversationMaxTextBytes && utf8.ValidString(value.Text) && !strings.ContainsRune(value.Text, 0)
}

func (s *DesktopService) CreateConversation(ctx context.Context, auth *DesktopAuthorization, input *DesktopConversationInput) (*DesktopConversationReceipt, error) {
	if auth == nil || auth.Session == nil {
		return nil, ErrDesktopSessionRequired
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if input.OrganizationID != auth.Member.Organization.PublicID || input.MemberID != auth.Member.Member.PublicID || input.InstallationID != auth.Session.InstallationID {
		return nil, ErrDesktopConversationIdentity
	}
	if !auth.Member.Organization.ConversationReportingEnabled {
		return nil, ErrDesktopConversationReportingDisabled
	}
	if s.conversationLimiter == nil || s.conversations == nil {
		return nil, ErrDesktopConversationStorage
	}
	retry, err := s.conversationLimiter.Allow(ctx, auth.Member.Member.PublicID)
	if err != nil {
		return nil, err
	}
	if retry > 0 {
		return nil, rateLimitedError(retry)
	}
	if err := s.TouchSession(ctx, auth); err != nil {
		return nil, err
	}
	return s.conversations.Create(ctx, auth.Member.Organization.ID, auth.Member.Member.ID, input)
}

func (f DesktopConversationFilters) Validate() error {
	if f.Client != "" && !desktopConversationClient(f.Client) {
		return ErrDesktopValidation
	}
	if f.CaptureStatus != "" && f.CaptureStatus != "captured" && f.CaptureStatus != "response_missing" {
		return ErrDesktopValidation
	}
	if !desktopConversationString(f.MemberID, 0, 64) || !desktopConversationString(f.MemberSearch, 0, 100) || !desktopConversationString(f.SourceSessionID, 0, 512) {
		return ErrDesktopValidation
	}
	if f.InstallationID != "" && ValidateDesktopInstallationID(f.InstallationID) != nil {
		return ErrDesktopValidation
	}
	if f.RecordID != "" {
		if id, err := uuid.Parse(f.RecordID); err != nil || id.String() != f.RecordID {
			return ErrDesktopValidation
		}
	}
	if f.ReceivedFrom != nil && f.ReceivedTo != nil && !f.ReceivedFrom.Before(*f.ReceivedTo) {
		return ErrDesktopValidation
	}
	return nil
}

// A positive manager ID always resolves the organization from the web identity.
func (s *DesktopService) conversationOrganization(ctx context.Context, organizationID string, managerID int64) (*DesktopOrganization, error) {
	if managerID > 0 {
		organization, err := s.GetManagedOrganization(ctx, managerID)
		if err != nil {
			return nil, err
		}
		if !organization.ConversationReportingEnabled {
			return nil, ErrDesktopConversationReportingDisabled
		}
		return organization, nil
	}
	return s.GetOrganization(ctx, organizationID)
}

func (s *DesktopService) ListConversations(ctx context.Context, organizationID string, managerID int64, params pagination.PaginationParams, filters DesktopConversationFilters) ([]DesktopConversationMetadata, *pagination.PaginationResult, error) {
	if err := filters.Validate(); err != nil {
		return nil, nil, err
	}
	if params.Page < 1 || params.PageSize < 1 || params.PageSize > 100 || (params.SortOrder != "asc" && params.SortOrder != "desc") {
		return nil, nil, ErrDesktopValidation
	}
	organization, err := s.conversationOrganization(ctx, organizationID, managerID)
	if err != nil {
		return nil, nil, err
	}
	items, page, err := s.conversations.List(ctx, organization.ID, params, filters)
	if err != nil {
		return nil, nil, err
	}
	for i := range items {
		items[i].OrganizationID = organization.PublicID
	}
	return items, page, nil
}

func (s *DesktopService) GetConversation(ctx context.Context, organizationID string, managerID int64, recordID string) (*DesktopConversationDetail, error) {
	id, err := uuid.Parse(recordID)
	if err != nil || id.String() != recordID {
		return nil, ErrDesktopConversationNotFound
	}
	organization, err := s.conversationOrganization(ctx, organizationID, managerID)
	if err != nil {
		return nil, err
	}
	result, err := s.conversations.Get(ctx, organization.ID, recordID)
	if err != nil {
		return nil, err
	}
	result.OrganizationID = organization.PublicID
	return result, nil
}
