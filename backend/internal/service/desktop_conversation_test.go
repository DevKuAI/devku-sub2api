package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func validDesktopConversationInput() *DesktopConversationInput {
	return &DesktopConversationInput{
		SchemaVersion: 2, RecordID: uuid.NewString(), Client: "workbuddy", InstallationID: uuid.NewString(),
		OrganizationID: "org_one", MemberID: "mem_one", SessionID: "source_one", StartedAt: time.Now().UTC(), StoppedAt: time.Now().UTC(),
		Prompts:  []DesktopTextSegment{{Text: "first"}, {Text: "second", Truncated: true}},
		Response: &DesktopTextSegment{Text: "answer"}, CaptureStatus: "captured",
	}
}

func TestDesktopConversationStrictContract(t *testing.T) {
	input := validDesktopConversationInput()
	raw, err := json.Marshal(input)
	require.NoError(t, err)
	decoded, err := DecodeDesktopConversation(raw)
	require.NoError(t, err)
	require.Equal(t, input.Prompts, decoded.Prompts)
	tests := map[string]func(map[string]any){
		"unknown":           func(m map[string]any) { m["hiddenReasoning"] = "private" },
		"case alias":        func(m map[string]any) { m["RecordId"] = m["recordId"]; delete(m, "recordId") },
		"missing response":  func(m map[string]any) { delete(m, "response") },
		"null schema":       func(m map[string]any) { m["schemaVersion"] = nil },
		"wrong status":      func(m map[string]any) { m["response"] = nil },
		"missing truncated": func(m map[string]any) { m["prompts"] = []any{map[string]any{"text": ""}} },
		"null truncated":    func(m map[string]any) { m["prompts"] = []any{map[string]any{"text": "", "truncated": nil}} },
		"unknown segment": func(m map[string]any) {
			m["response"] = map[string]any{"text": "", "truncated": false, "tool": "private"}
		},
		"empty prompts":    func(m map[string]any) { m["prompts"] = []any{} },
		"too many prompts": func(m map[string]any) { m["prompts"] = make([]DesktopTextSegment, 65) },
		"non UTC":          func(m map[string]any) { m["startedAt"] = "2026-09-18T00:00:00+00:00" },
		"comma fractional": func(m map[string]any) { m["startedAt"] = "2026-09-18T00:00:00,01Z" },
		"reversed times":   func(m map[string]any) { m["startedAt"] = "2099-01-01T00:00:00Z" },
		"bad client":       func(m map[string]any) { m["client"] = "other" },
		"uppercase uuid":   func(m map[string]any) { m["recordId"] = strings.ToUpper(input.RecordID) },
		"not uuid v4":      func(m map[string]any) { m["recordId"] = "00000000-0000-0000-0000-000000000000" },
		"NUL text":         func(m map[string]any) { m["response"] = DesktopTextSegment{Text: "a\x00b"} },
		"NUL cwd":          func(m map[string]any) { m["cwd"] = "a\x00b" },
		"overlong cwd":     func(m map[string]any) { m["cwd"] = strings.Repeat("文", 4097) },
		"byte limit": func(m map[string]any) {
			m["response"] = DesktopTextSegment{Text: strings.Repeat("文", DesktopConversationMaxTextBytes/3+1)}
		},
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			var value map[string]any
			require.NoError(t, json.Unmarshal(raw, &value))
			change(value)
			data, err := json.Marshal(value)
			require.NoError(t, err)
			_, err = DecodeDesktopConversation(data)
			require.ErrorIs(t, err, ErrDesktopValidation)
		})
	}
	for _, invalid := range [][]byte{append(append([]byte{}, raw...), []byte("{}")...), bytesReplace(raw, []byte(`"first"`), []byte{'"', 0xff, '"'}), bytesReplace(raw, []byte(`"first"`), []byte(`"\ud800"`)), bytesReplace(raw, []byte(`"schemaVersion":2`), []byte(`"schemaVersion":2,"schemaVersion":2`))} {
		_, err := DecodeDesktopConversation(invalid)
		require.ErrorIs(t, err, ErrDesktopValidation)
	}
	input.CaptureStatus, input.Response = "response_missing", nil
	input.Prompts = []DesktopTextSegment{{Text: ""}}
	input.StartedAt = time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	raw, err = json.Marshal(input)
	require.NoError(t, err)
	_, err = DecodeDesktopConversation(raw)
	require.NoError(t, err)
}

func bytesReplace(raw, old, next []byte) []byte {
	return []byte(strings.Replace(string(raw), string(old), string(next), 1))
}

type desktopSessionStub struct {
	session *DesktopSession
	touches int
	deleted bool
	err     error
}

func (s *desktopSessionStub) Create(_ context.Context, _ string, value *DesktopSession) error {
	s.session = value
	return s.err
}
func (s *desktopSessionStub) Get(context.Context, string) (*DesktopSession, error) {
	return s.session, s.err
}
func (s *desktopSessionStub) Touch(context.Context, string) error {
	if s.err == nil {
		s.touches++
	}
	return s.err
}
func (s *desktopSessionStub) Delete(context.Context, string) error { s.deleted = true; return s.err }

type desktopConversationLimiterStub struct {
	delay time.Duration
	calls int
}

func (s *desktopConversationLimiterStub) Allow(context.Context, string) (time.Duration, error) {
	s.calls++
	return s.delay, nil
}

type desktopConversationRepositoryStub struct {
	DesktopConversationRepository
	writes         int
	err            error
	organizationID int64
}

func (s *desktopConversationRepositoryStub) Create(_ context.Context, _, _ int64, input *DesktopConversationInput) (*DesktopConversationReceipt, error) {
	s.writes++
	return &DesktopConversationReceipt{RecordID: input.RecordID, ReceivedAt: time.Now()}, s.err
}
func (s *desktopConversationRepositoryStub) List(_ context.Context, organizationID int64, _ pagination.PaginationParams, _ DesktopConversationFilters) ([]DesktopConversationMetadata, *pagination.PaginationResult, error) {
	s.organizationID = organizationID
	return []DesktopConversationMetadata{}, &pagination.PaginationResult{}, nil
}

func TestDesktopConversationActivityOrdering(t *testing.T) {
	for _, scenario := range []string{"ok", "identity", "invalid", "limited", "expired", "storage", "duplicate", "v1", "disabled"} {
		t.Run(scenario, func(t *testing.T) {
			input := validDesktopConversationInput()
			sessions := &desktopSessionStub{}
			limiter := &desktopConversationLimiterStub{}
			repo := &desktopConversationRepositoryStub{}
			svc := &DesktopService{sessions: sessions, conversationLimiter: limiter, conversations: repo}
			auth := &DesktopAuthorization{Session: &DesktopSession{InstallationID: input.InstallationID}, Member: &DesktopAuthorizedMember{Member: &DesktopMember{ID: 1, PublicID: input.MemberID}, Organization: &DesktopOrganization{ID: 2, PublicID: input.OrganizationID, ConversationReportingEnabled: true}}}
			switch scenario {
			case "identity":
				input.MemberID = "mem_other"
			case "invalid":
				input.Prompts = nil
			case "limited":
				limiter.delay = time.Minute
			case "expired":
				sessions.err = ErrDesktopUnauthenticated
			case "storage":
				repo.err = ErrDesktopConversationStorage
			case "duplicate":
				repo.err = ErrDesktopConversationExists
			case "v1":
				auth.Session = nil
			case "disabled":
				auth.Member.Organization.ConversationReportingEnabled = false
			}
			_, err := svc.CreateConversation(context.Background(), auth, input)
			if scenario == "ok" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			if scenario == "ok" || scenario == "storage" || scenario == "duplicate" {
				require.Equal(t, 1, sessions.touches)
				require.Equal(t, 1, repo.writes)
			} else {
				require.Zero(t, sessions.touches)
				require.Zero(t, repo.writes)
			}
		})
	}
}

func TestDesktopV2RechecksIdentityAndInstallation(t *testing.T) {
	installation := uuid.NewString()
	sessions := &desktopSessionStub{session: &DesktopSession{MemberPublicID: "mem_one", OrganizationPublicID: "org_one", InstallationID: installation, MemberVersion: 1, OrganizationVersion: 1}}
	member := &DesktopAuthorizedMember{Member: &DesktopMember{PublicID: "mem_one", Status: "active", AuthVersion: 1}, Organization: &DesktopOrganization{PublicID: "org_one", Status: "active", AuthVersion: 1}, GatewayUser: &User{Status: "active"}}
	svc := &DesktopService{sessions: sessions, repo: &desktopRepositoryStub{authorized: member}}
	token := "dks_" + base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	auth, err := svc.AuthorizeRequest(context.Background(), token, installation)
	require.NoError(t, err)
	require.Zero(t, sessions.touches)
	require.Equal(t, HashDesktopOpaqueToken(token), auth.TokenHash)
	_, err = svc.AuthorizeRequest(context.Background(), token, uuid.NewString())
	require.ErrorIs(t, err, ErrDesktopInstallationMismatch)
	for _, target := range []*string{&member.Member.Status, &member.Organization.Status, &member.GatewayUser.Status} {
		*target = "disabled"
		_, err = svc.AuthorizeRequest(context.Background(), token, installation)
		require.ErrorIs(t, err, ErrDesktopMembershipRevoked)
		*target = "active"
	}
	member.Member.AuthVersion++
	_, err = svc.AuthorizeRequest(context.Background(), token, installation)
	require.ErrorIs(t, err, ErrDesktopMembershipRevoked)
	require.NoError(t, svc.LogoutAuthorized(context.Background(), auth, token))
	require.True(t, sessions.deleted)
	require.Zero(t, sessions.touches)
}

func TestDesktopManagedConversationScopeComesFromUser(t *testing.T) {
	repo := &desktopConversationRepositoryStub{}
	identityRepo := &desktopRepositoryStub{organization: &DesktopOrganization{ID: 7, PublicID: "org_owned", ConversationReportingEnabled: true}}
	svc := &DesktopService{repo: identityRepo, conversations: repo}
	_, _, err := svc.ListConversations(context.Background(), "org_other", 42, pagination.DefaultPagination(), DesktopConversationFilters{})
	require.NoError(t, err)
	require.Equal(t, []int64{42}, identityRepo.scopedUserIDs)
	require.EqualValues(t, 7, repo.organizationID)
}

func TestDesktopConversationReportingRequiresOptInBeforeActivity(t *testing.T) {
	input := validDesktopConversationInput()
	sessions := &desktopSessionStub{}
	limiter := &desktopConversationLimiterStub{}
	records := &desktopConversationRepositoryStub{}
	svc := &DesktopService{sessions: sessions, conversationLimiter: limiter, conversations: records}
	auth := &DesktopAuthorization{Session: &DesktopSession{InstallationID: input.InstallationID}, Member: &DesktopAuthorizedMember{Member: &DesktopMember{ID: 1, PublicID: input.MemberID}, Organization: &DesktopOrganization{ID: 2, PublicID: input.OrganizationID}}}
	_, err := svc.CreateConversation(context.Background(), auth, input)
	require.ErrorIs(t, err, ErrDesktopConversationReportingDisabled)
	require.Zero(t, limiter.calls)
	require.Zero(t, sessions.touches)
	require.Zero(t, records.writes)
}

func (s *desktopConversationRepositoryStub) Get(_ context.Context, organizationID int64, recordID string) (*DesktopConversationDetail, error) {
	s.organizationID = organizationID
	return &DesktopConversationDetail{DesktopConversationMetadata: DesktopConversationMetadata{RecordID: recordID}}, nil
}

func TestDesktopConversationReportingControlsManagedHistoryOnly(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, managerID := range []int64{0, 42} {
			records := &desktopConversationRepositoryStub{}
			identity := &desktopRepositoryStub{organization: &DesktopOrganization{ID: 7, PublicID: "org_one", ConversationReportingEnabled: enabled}}
			svc := &DesktopService{repo: identity, conversations: records}
			_, _, listErr := svc.ListConversations(context.Background(), "org_one", managerID, pagination.DefaultPagination(), DesktopConversationFilters{})
			_, getErr := svc.GetConversation(context.Background(), "org_one", managerID, uuid.NewString())
			if managerID > 0 && !enabled {
				require.ErrorIs(t, listErr, ErrDesktopConversationReportingDisabled)
				require.ErrorIs(t, getErr, ErrDesktopConversationReportingDisabled)
				require.Zero(t, records.organizationID)
			} else {
				require.NoError(t, listErr)
				require.NoError(t, getErr)
				require.EqualValues(t, 7, records.organizationID)
			}
		}
	}
}
