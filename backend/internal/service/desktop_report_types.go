package service

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrDesktopReportLeaseLost = errors.New("desktop report lease lost")

func validateDesktopSummaryConfig(enabled, reporting bool, model string) error {
	if len([]rune(model)) > 200 || (enabled && (!reporting || strings.TrimSpace(model) == "")) {
		return ErrDesktopValidation.WithMetadata(map[string]string{"field": "analysis_model"})
	}
	return nil
}

// The marker cannot be supplied by an external HTTP request.
type desktopReportRequestKey struct{}

func WithDesktopReportRequest(ctx context.Context) context.Context {
	return context.WithValue(ctx, desktopReportRequestKey{}, true)
}
func IsDesktopReportRequest(ctx context.Context) bool {
	value, _ := ctx.Value(desktopReportRequestKey{}).(bool)
	return value
}

type DesktopReportSource struct {
	RecordID      string               `json:"record_id"`
	Client        string               `json:"client"`
	SessionID     string               `json:"session_id"`
	StoppedAt     time.Time            `json:"stopped_at"`
	Prompts       []DesktopTextSegment `json:"prompts"`
	Response      *DesktopTextSegment  `json:"response"`
	CaptureStatus string               `json:"capture_status"`
}

// Model is the immutable Model ID supplied to this invocation, not current configuration.
type DesktopReportExecution struct {
	ID         int64      `json:"id"`
	Revision   int        `json:"revision"`
	Kind       string     `json:"kind"`
	MemberID   string     `json:"member_id,omitempty"`
	Model      string     `json:"model"`
	Round      int        `json:"round"`
	Chunk      int        `json:"chunk"`
	Attempt    int        `json:"attempt"`
	RequestID  string     `json:"request_id"`
	Status     string     `json:"status"`
	Error      string     `json:"error"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

type DesktopReportMember struct {
	Executions  []DesktopReportExecution `json:"executions"`
	MemberID    string                   `json:"member_id"`
	Name        string                   `json:"name"`
	Deleted     bool                     `json:"deleted"`
	RecordCount int                      `json:"record_count"`
	Status      string                   `json:"status"`
	Content     string                   `json:"content"`
	Model       string                   `json:"model"`
	GeneratedAt *time.Time               `json:"generated_at"`
	Error       string                   `json:"error"`
	SourceIDs   []string                 `json:"source_ids"`
	Sources     []DesktopReportSource    `json:"-"`
}

type DesktopGeneratedReport struct {
	Executions  []DesktopReportExecution `json:"executions"`
	Status      string                   `json:"status"`
	Content     string                   `json:"content"`
	Model       string                   `json:"model"`
	GeneratedAt *time.Time               `json:"generated_at"`
	Error       string                   `json:"error"`
}

// Work is persisted after every successful chunk so a resumed task reuses it.
type DesktopReportWork struct {
	Inputs   []string  `json:"inputs,omitempty"`
	Outputs  []string  `json:"outputs,omitempty"`
	Attempts int       `json:"attempts"`
	Round    int       `json:"round"`
	RetryAt  time.Time `json:"retry_at,omitempty"`
}

type DesktopReportSnapshot struct {
	Member  DesktopReportMember   `json:"member"`
	Sources []DesktopReportSource `json:"sources"`
	Work    DesktopReportWork     `json:"work"`
}

type DesktopReportPayload struct {
	Model              string                  `json:"model"`
	GroupID            int64                   `json:"group_id"`
	UserID             int64                   `json:"user_id"`
	Snapshots          []DesktopReportSnapshot `json:"snapshots"`
	Summary            DesktopGeneratedReport  `json:"summary"`
	SummaryWork        DesktopReportWork       `json:"summary_work"`
	SummaryFingerprint string                  `json:"summary_fingerprint"`
}

type DesktopReportTask struct {
	ID             int64                `json:"id"`
	OrganizationID int64                `json:"-"`
	Date           string               `json:"date"`
	Status         string               `json:"status"`
	Reason         string               `json:"reason"`
	Revision       int                  `json:"revision"`
	LeaseToken     string               `json:"-"`
	Payload        DesktopReportPayload `json:"-"`
}

type DesktopReportDay struct {
	Date             string                 `json:"date"`
	Timezone         string                 `json:"timezone"`
	AnalysisModel    string                 `json:"analysis_model"`
	Status           string                 `json:"status"`
	Reason           string                 `json:"reason"`
	Task             *DesktopReportTask     `json:"task,omitempty"`
	Members          []DesktopReportMember  `json:"members"`
	Summary          DesktopGeneratedReport `json:"summary"`
	ExpectedMembers  int                    `json:"expected_members"`
	CompletedMembers int                    `json:"completed_members"`
	MissingMembers   []string               `json:"missing_members"`
}

type DesktopReportRepository interface {
	Day(context.Context, int64, string) ([]DesktopReportMember, error)
	Task(context.Context, int64, string) (*DesktopReportTask, error)
	Enqueue(context.Context, int64, string, string) (*DesktopReportTask, error)
	DueOrganizations(context.Context, string) ([]string, error)
	Claim(context.Context) (*DesktopReportTask, error)
	Snapshot(context.Context, int64, string) ([]DesktopReportSnapshot, error)
	Save(context.Context, *DesktopReportTask, time.Time, bool) error
	Renew(context.Context, *DesktopReportTask) error
	OrganizationPublicID(context.Context, int64) (string, error)
	AnalysisKey(context.Context, *DesktopOrganization) (*APIKey, error)
	BeginExecution(context.Context, *DesktopReportTask, *DesktopReportExecution) error
	FinishExecution(context.Context, *DesktopReportExecution) error
	Executions(context.Context, int64, string) ([]DesktopReportExecution, error)
}

type DesktopReportAI interface {
	Check(context.Context, *DesktopOrganization, *APIKey, string) error
	Generate(context.Context, *DesktopOrganization, *APIKey, string, string, string, string) (string, error)
}
