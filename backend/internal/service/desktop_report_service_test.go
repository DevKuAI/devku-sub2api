package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type reportTestIdentity struct {
	DesktopRepository
	organization *DesktopOrganization
}

func (r *reportTestIdentity) GetOrganization(context.Context, string) (*DesktopOrganization, error) {
	return r.organization, nil
}
func (r *reportTestIdentity) GetOrganizationForGatewayUser(_ context.Context, user int64) (*DesktopOrganization, error) {
	if user != r.organization.GatewayUserID {
		return nil, ErrDesktopOrganizationNotFound
	}
	return r.organization, nil
}
func (r *reportTestIdentity) ScopedToGatewayUser(int64) DesktopRepository { return r }

type reportTestRepo struct {
	executions []DesktopReportExecution
	beginError error
	DesktopReportRepository
	members     []DesktopReportMember
	snapshots   []DesktopReportSnapshot
	task        *DesktopReportTask
	enqueued    int
	saved       int
	scannedDate string
	ids         []string
}

func (r *reportTestRepo) Day(_ context.Context, _ int64, date string) ([]DesktopReportMember, error) {
	r.scannedDate = date
	return append([]DesktopReportMember{}, r.members...), nil
}
func (r *reportTestRepo) Task(context.Context, int64, string) (*DesktopReportTask, error) {
	return r.task, nil
}
func (r *reportTestRepo) Enqueue(_ context.Context, org int64, date, mode string) (*DesktopReportTask, error) {
	r.enqueued++
	r.task = &DesktopReportTask{ID: 1, OrganizationID: org, Date: date, Status: "pending", Revision: 1}
	return r.task, nil
}
func (r *reportTestRepo) DueOrganizations(_ context.Context, date string) ([]string, error) {
	r.scannedDate = date
	return r.ids, nil
}
func (r *reportTestRepo) Claim(context.Context) (*DesktopReportTask, error) { return nil, nil }
func (r *reportTestRepo) Snapshot(context.Context, int64, string) ([]DesktopReportSnapshot, error) {
	return r.snapshots, nil
}
func (r *reportTestRepo) Save(_ context.Context, t *DesktopReportTask, _ time.Time, _ bool) error {
	r.saved++
	r.task = t
	return nil
}
func (r *reportTestRepo) OrganizationPublicID(context.Context, int64) (string, error) {
	return "org_one", nil
}
func (r *reportTestRepo) AnalysisKey(_ context.Context, o *DesktopOrganization) (*APIKey, error) {
	return &APIKey{ID: 3, UserID: o.GatewayUserID, GroupID: &o.GroupID}, nil
}

func (r *reportTestRepo) BeginExecution(_ context.Context, _ *DesktopReportTask, execution *DesktopReportExecution) error {
	if r.beginError != nil {
		return r.beginError
	}
	execution.ID = int64(len(r.executions) + 1)
	r.executions = append(r.executions, *execution)
	return nil
}
func (r *reportTestRepo) FinishExecution(_ context.Context, execution *DesktopReportExecution) error {
	for i := range r.executions {
		if r.executions[i].ID == execution.ID {
			r.executions[i] = *execution
			return nil
		}
	}
	return ErrDesktopReportLeaseLost
}
func (r *reportTestRepo) Executions(context.Context, int64, string) ([]DesktopReportExecution, error) {
	return append([]DesktopReportExecution{}, r.executions...), nil
}

type reportTestAI struct {
	blocked    error
	calls      []string
	failMember string
	beforeCall func(int)
}

func (a *reportTestAI) Check(context.Context, *DesktopOrganization, *APIKey, string) error {
	return a.blocked
}
func (a *reportTestAI) Generate(_ context.Context, _ *DesktopOrganization, _ *APIKey, model, prompt, input, id string) (string, error) {
	a.calls = append(a.calls, model+" "+id)
	if a.beforeCall != nil {
		a.beforeCall(len(a.calls))
	}
	if a.failMember != "" && strings.Contains(id, "member:"+a.failMember) {
		return "", errors.New("upstream failure")
	}
	return "有证据的工作摘要", nil
}
func reportFixture() (*DesktopReportService, *reportTestRepo, *reportTestAI, *DesktopOrganization) {
	o := &DesktopOrganization{ID: 1, PublicID: "org_one", Status: "active", GatewayUserID: 2, GroupID: 7, ConversationSummaryEnabled: true, ConversationReportingEnabled: true, AnalysisModel: "test-model"}
	repo := &reportTestRepo{}
	ai := &reportTestAI{}
	desktop := &DesktopService{repo: &reportTestIdentity{organization: o}}
	s := NewDesktopReportService(desktop, repo)
	desktop.reports = s
	s.ai = ai
	s.now = func() time.Time { return time.Date(2026, 10, 10, 2, 10, 0, 0, s.loc) }
	return s, repo, ai, o
}
func reportSnapshot(id string) DesktopReportSnapshot {
	return DesktopReportSnapshot{Member: DesktopReportMember{MemberID: id, Name: id, RecordCount: 1, Status: "pending", SourceIDs: []string{"record-" + id}}, Sources: []DesktopReportSource{{RecordID: "record-" + id, Prompts: []DesktopTextSegment{{Text: "完成测试"}}, CaptureStatus: "response_missing"}}}
}

func TestDesktopReportsEligibilityAndNoRecords(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*DesktopOrganization, *reportTestAI)
	}{
		{"disabled", func(o *DesktopOrganization, _ *reportTestAI) { o.ConversationSummaryEnabled = false }},
		{"no model", func(o *DesktopOrganization, _ *reportTestAI) { o.AnalysisModel = "" }},
		{"reporting disabled", func(o *DesktopOrganization, _ *reportTestAI) { o.ConversationReportingEnabled = false }},
		{"organization disabled", func(o *DesktopOrganization, _ *reportTestAI) { o.Status = "disabled" }},
		{"model unsupported", func(_ *DesktopOrganization, a *reportTestAI) {
			a.blocked = NewDesktopReportUnavailable("model_unsupported")
		}},
		{"no accounts", func(_ *DesktopOrganization, a *reportTestAI) {
			a.blocked = NewDesktopReportUnavailable("model_unavailable")
		}},
		{"no balance", func(_ *DesktopOrganization, a *reportTestAI) {
			a.blocked = NewDesktopReportUnavailable("billing_unavailable")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, r, a, o := reportFixture()
			r.members = []DesktopReportMember{{RecordCount: 1}}
			test.change(o, a)
			_, err := s.Enqueue(context.Background(), o, "2026-10-09", "generate")
			require.Error(t, err)
			require.Zero(t, r.enqueued)
			require.Empty(t, a.calls)
		})
	}
	s, r, a, o := reportFixture()
	r.members = []DesktopReportMember{{MemberID: "empty", Status: "no_records"}}
	task, err := s.Enqueue(context.Background(), o, "2026-10-09", "generate")
	require.NoError(t, err)
	require.Nil(t, task)
	require.Zero(t, r.enqueued)
	day, err := s.Day(context.Background(), o, "2026-10-09")
	require.NoError(t, err)
	require.Equal(t, "no_records", day.Status)
	require.Len(t, day.Members, 1)
	require.Empty(t, a.calls)
}
func TestDesktopReportsPreviousDateAndSchedule(t *testing.T) {
	s, r, a, _ := reportFixture()
	date, err := s.date("")
	require.NoError(t, err)
	require.Equal(t, "2026-10-09", date)
	_, err = s.date("2026-10-10")
	require.Error(t, err)
	_, err = s.date("2026-02-30")
	require.Error(t, err)
	s.now = func() time.Time { return time.Date(2026, 10, 9, 17, 59, 0, 0, time.UTC) }
	s.tick(context.Background())
	require.Empty(t, r.scannedDate)
	s.now = func() time.Time { return time.Date(2026, 10, 9, 18, 0, 0, 0, time.UTC) }
	s.tick(context.Background())
	require.Equal(t, "2026-10-09", r.scannedDate)
	require.Empty(t, a.calls)
}
func TestDesktopReportsPartialRetryAndModelRecovery(t *testing.T) {
	s, r, a, _ := reportFixture()
	r.snapshots = []DesktopReportSnapshot{reportSnapshot("one"), reportSnapshot("two")}
	a.failMember = "two"
	task := &DesktopReportTask{ID: 1, OrganizationID: 1, Date: "2026-10-09", Revision: 1}
	next, err := s.process(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, s.now().Add(time.Minute), next)
	require.Equal(t, "pending", task.Status)
	require.Equal(t, "completed", task.Payload.Summary.Status)
	require.Len(t, a.calls, 3)
	require.Equal(t, "test-model", task.Payload.Snapshots[0].Member.Model)
	require.Equal(t, "test-model", task.Payload.Summary.Model)
	a.blocked = NewDesktopReportUnavailable("model_unavailable")
	_, err = s.process(context.Background(), task)
	require.Error(t, err)
	require.Len(t, a.calls, 3)
	a.blocked = nil
	a.failMember = ""
	now := s.now().Add(time.Minute)
	s.now = func() time.Time { return now }
	_, err = s.process(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, "completed", task.Status)
	require.Len(t, a.calls, 5, "successful member is reused; missing member and summary run")
}
func TestDesktopReportsRetryBudgetAndAllFailed(t *testing.T) {
	s, r, a, _ := reportFixture()
	r.snapshots = []DesktopReportSnapshot{reportSnapshot("one")}
	a.failMember = "one"
	task := &DesktopReportTask{ID: 1, OrganizationID: 1, Date: "2026-10-09", Revision: 1}
	for attempt := 1; attempt <= 3; attempt++ {
		next, err := s.process(context.Background(), task)
		require.NoError(t, err)
		if attempt < 3 {
			now := next
			s.now = func() time.Time { return now }
		}
	}
	require.Equal(t, "failed", task.Status)
	require.Len(t, a.calls, 3)
	require.Empty(t, task.Payload.Summary.Content)
	_, err := s.process(context.Background(), task)
	require.NoError(t, err)
	require.Len(t, a.calls, 3)
}
func TestDesktopReportsStageRechecksAndChunkResume(t *testing.T) {
	s, r, a, o := reportFixture()
	snapshot := reportSnapshot("one")
	snapshot.Sources[0].Prompts[0].Text = strings.Repeat("字", desktopReportChunkRunes+100)
	r.snapshots = []DesktopReportSnapshot{snapshot}
	a.beforeCall = func(n int) {
		if n == 1 {
			o.ConversationSummaryEnabled = false
		}
	}
	task := &DesktopReportTask{ID: 1, OrganizationID: 1, Date: "2026-10-09", Revision: 1}
	_, err := s.process(context.Background(), task)
	require.Error(t, err)
	require.Len(t, a.calls, 1)
	require.Len(t, task.Payload.Snapshots[0].Work.Outputs, 1)
	o.ConversationSummaryEnabled = true
	a.beforeCall = nil
	_, err = s.process(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, "completed", task.Status)
	require.Len(t, a.calls, 4, "resume remaining chunk, merge, then summarize")
	require.Len(t, r.executions, 4)
	require.Equal(t, 1, r.executions[2].Round)
	for _, execution := range r.executions {
		require.Equal(t, "test-model", execution.Model)
	}
}
func TestDesktopReportsConfigAndIdentity(t *testing.T) {
	require.Error(t, validateDesktopSummaryConfig(true, true, ""))
	require.Error(t, validateDesktopSummaryConfig(true, false, "model"))
	require.NoError(t, validateDesktopSummaryConfig(false, false, ""))
	s, _, _, o := reportFixture()
	s.desktop.now = s.now
	o.ConversationSummaryEnabled = false
	_, err := s.desktop.ReportDay(context.Background(), "forged", o.GatewayUserID, "2026-10-09")
	require.ErrorIs(t, err, ErrDesktopOrgReadOnly)
	_, err = s.desktop.ReportDay(context.Background(), "org_one", 99, "2026-10-09")
	require.ErrorIs(t, err, ErrDesktopOrganizationNotFound)
	require.False(t, IsDesktopReportRequest(context.Background()))
	require.True(t, IsDesktopReportRequest(WithDesktopReportRequest(context.Background())))
}

func TestDesktopReportInternalKeySurvivesAuthCache(t *testing.T) {
	svc := &APIKeyService{}
	key := &APIKey{ID: 2, UserID: 3, Key: "system-key", ManagedBy: "desktop_analysis", User: &User{ID: 3, Status: StatusActive}}
	snapshot := svc.snapshotFromAPIKey(context.Background(), key)
	require.Equal(t, "desktop_analysis", snapshot.ManagedBy)
	restored := svc.snapshotToAPIKey(key.Key, snapshot)
	require.Equal(t, "desktop_analysis", restored.ManagedBy)
}

func TestDesktopReportsLateMemberDoesNotChangeSnapshotCoverage(t *testing.T) {
	s, r, _, o := reportFixture()
	r.members = []DesktopReportMember{{MemberID: "one", Name: "One", RecordCount: 2}, {MemberID: "late", Name: "Late", RecordCount: 1}, {MemberID: "empty", Name: "Empty", Status: "no_records"}}
	snapshot := reportSnapshot("one")
	snapshot.Member.Status = "completed"
	snapshot.Member.Content = "fixed report"
	r.task = &DesktopReportTask{Status: "completed", Payload: DesktopReportPayload{Model: o.AnalysisModel, Snapshots: []DesktopReportSnapshot{snapshot}, Summary: DesktopGeneratedReport{Status: "completed", Content: "fixed summary"}}}
	day, err := s.Day(context.Background(), o, "2026-10-09")
	require.NoError(t, err)
	require.Equal(t, 1, day.ExpectedMembers)
	require.Equal(t, 1, day.CompletedMembers)
	require.Empty(t, day.MissingMembers)
	require.Equal(t, "completed", day.Summary.Status)
	require.Equal(t, "not_in_snapshot", day.Members[1].Status)
	require.Equal(t, 1, day.Members[0].RecordCount)
}

func TestDesktopReportsRecoveredEligibilityAndConfigurationChanges(t *testing.T) {
	s, r, a, o := reportFixture()
	r.members = []DesktopReportMember{{MemberID: "one", RecordCount: 1, Status: "pending"}}
	r.task = &DesktopReportTask{Status: "waiting", Reason: "model_unavailable"}
	day, err := s.Day(context.Background(), o, "2026-10-09")
	require.NoError(t, err)
	require.Empty(t, day.Reason)
	require.Equal(t, "pending", day.Status)
	r.snapshots = []DesktopReportSnapshot{reportSnapshot("one")}
	task := &DesktopReportTask{ID: 1, OrganizationID: 1, Date: "2026-10-09", Revision: 1}
	a.beforeCall = func(int) { o.AnalysisModel = "new-model" }
	_, err = s.process(context.Background(), task)
	var unavailable *DesktopReportUnavailable
	require.ErrorAs(t, err, &unavailable)
	require.Equal(t, "configuration_changed", unavailable.Reason)
	require.Len(t, a.calls, 1)
	day, err = s.Day(context.Background(), o, "2026-10-09")
	require.NoError(t, err)
	require.Equal(t, "configuration_changed", day.Reason)
}

func TestDesktopReportsRecordEveryInvocationModelAndKeepHistory(t *testing.T) {
	s, r, a, o := reportFixture()
	r.snapshots = []DesktopReportSnapshot{reportSnapshot("one")}
	a.failMember = "one"
	task := &DesktopReportTask{ID: 1, OrganizationID: 1, Date: "2026-10-09", Revision: 1}
	next, err := s.process(context.Background(), task)
	require.NoError(t, err)
	require.Len(t, r.executions, 1)
	require.Equal(t, "failed", r.executions[0].Status)
	require.Equal(t, "test-model", r.executions[0].Model)
	a.failMember = ""
	now := next
	s.now = func() time.Time { return now }
	_, err = s.process(context.Background(), task)
	require.NoError(t, err)
	require.Len(t, r.executions, 3)
	require.Equal(t, "member", r.executions[1].Kind)
	require.Equal(t, 2, r.executions[1].Attempt)
	require.Equal(t, "summary", r.executions[2].Kind)
	o.AnalysisModel = "new-model"
	r.snapshots = []DesktopReportSnapshot{reportSnapshot("one")}
	task = &DesktopReportTask{ID: 1, OrganizationID: 1, Date: "2026-10-09", Revision: 2}
	_, err = s.process(context.Background(), task)
	require.NoError(t, err)
	require.Len(t, r.executions, 5)
	require.Equal(t, "test-model", r.executions[0].Model)
	require.Equal(t, "new-model", r.executions[3].Model)
	require.Equal(t, 2, r.executions[3].Revision)
	r.members = []DesktopReportMember{{MemberID: "one", RecordCount: 1}}
	day, err := s.Day(context.Background(), o, "2026-10-09")
	require.NoError(t, err)
	require.Len(t, day.Members[0].Executions, 3)
	require.Len(t, day.Summary.Executions, 2)
	ids := map[string]bool{}
	for _, execution := range r.executions {
		require.NotEmpty(t, execution.RequestID)
		require.False(t, ids[execution.RequestID])
		ids[execution.RequestID] = true
		require.NotNil(t, execution.FinishedAt)
	}
}
func TestDesktopReportsHistoryRequiredBeforeCallingAI(t *testing.T) {
	s, r, a, _ := reportFixture()
	r.snapshots = []DesktopReportSnapshot{reportSnapshot("one")}
	r.beginError = ErrDesktopReportLeaseLost
	_, err := s.process(context.Background(), &DesktopReportTask{ID: 1, OrganizationID: 1, Date: "2026-10-09", Revision: 1})
	require.ErrorIs(t, err, ErrDesktopReportLeaseLost)
	require.Empty(t, a.calls)
	require.Empty(t, r.executions)
}
